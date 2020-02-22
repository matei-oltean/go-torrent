package fileutils

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"time"
)

// the actions for an udp transfer
const (
	aConnect uint32 = iota
	aAnnounce
	aScrape
	aError
)

// udpConn is a simple struct with a connection and its scheme
type udpConn struct {
	Conn   *net.UDPConn
	Scheme string
}

// SubFile represents a subfile in the case of multi file torrents
type SubFile struct {
	CumStart int    // start of the file
	Length   int    // length of the file
	Path     string // path to the file
}

// TorrentFile represents a flattened torrent file
type TorrentFile struct {
	Announce []*url.URL
	Info     *Info
}

// parseAnnounceList parses and flattens the announce list
// it should be a list of lists of urls (as strings)
func parseAnnounceList(l []bencode) []*url.URL {
	q := []*url.URL{}
	for _, subL := range l {
		if subL.List == nil || len(subL.List) == 0 {
			continue
		}
		for _, u := range subL.List {
			if u.Str == "" {
				continue
			}
			parsedU, err := url.Parse(u.Str)
			if err != nil {
				continue
			}
			q = append(q, parsedU)
		}
	}
	return q
}

// splitPieces splits the concatenated hashes of the files into a list of hashes
func splitPieces(pieces string) ([][20]byte, error) {
	buff := []byte(pieces)
	if len(buff)%20 != 0 {
		return nil, fmt.Errorf("Pieces has a length not divisible by 20: %d", len(buff))
	}
	hashes := make([][20]byte, len(buff)/20)
	for i := range hashes {
		copy(hashes[i][:], buff[i*20:(i+1)*20])
	}
	return hashes, nil
}

// parseFiles parses the files into a slice of subFile
// also returns the total file length
func parseFiles(files []bencode) ([]SubFile, int, error) {
	res := make([]SubFile, len(files))
	totalLen := 0
	for i, file := range files {
		dic := file.Dict
		if dic == nil {
			return nil, 0, fmt.Errorf("file %d has no dictionary", i)
		}
		length, ok := dic["length"]
		if !ok {
			return nil, 0, fmt.Errorf("file %d missing key length", i)
		}
		if length.Int <= 0 {
			return nil, 0, fmt.Errorf("file %d has a negative value for length: %d", i, length.Int)
		}

		path, ok := dic["path"]
		if !ok || path.List == nil || len(path.List) == 0 {
			return nil, 0, fmt.Errorf("file %d missing key path", i)
		}
		paths := ""
		for _, p := range path.List {
			paths = filepath.Join(paths, p.Str)
		}
		res[i] = SubFile{
			CumStart: totalLen,
			Length:   length.Int,
			Path:     paths,
		}
		totalLen += length.Int
	}
	return res, totalLen, nil
}

func prettyTorrentBencode(ben *bencode) (*TorrentFile, error) {
	dic := ben.Dict
	if dic == nil {
		return nil, errors.New("torrent file has no dictionary")
	}
	announce, ok := dic["announce"]
	if !ok || announce.Str == "" {
		return nil, errors.New("torrent file missing announce key")
	}
	u, err := url.Parse(announce.Str)
	if err != nil {
		return nil, fmt.Errorf("could not parse announce: %s", err.Error())
	}
	ann := []*url.URL{u}
	annList, ok := dic["announce-list"]
	if ok && annList.List != nil {
		urls := parseAnnounceList(annList.List)
		if len(urls) > 0 {
			ann = urls
		}
	}

	info, ok := dic["info"]
	if !ok || info.Dict == nil {
		return nil, errors.New("torrent file missing info key")
	}
	dict := info.Dict
	piece, ok := dict["pieces"]
	if !ok || piece.Str == "" {
		return nil, errors.New("info dictionary missing key pieces")
	}

	name, ok := dict["name"]
	if !ok || name.Str == "" {
		return nil, errors.New("info dictionary missing key name")
	}

	pieceLen, ok := dict["piece length"]
	if !ok {
		return nil, errors.New("info dictionary missing key piece length")
	}
	if pieceLen.Int <= 0 {
		return nil, fmt.Errorf("negative value for piece length: %d", pieceLen.Int)
	}

	finalLen := 0
	var subFiles []SubFile
	// in case of single file, there is a length key
	length, ok := dict["length"]
	if ok {
		if length.Int < 0 {
			return nil, fmt.Errorf("negative value for length: %d", length.Int)
		}
		finalLen = length.Int
		file := SubFile{
			Length: finalLen,
			Path:   name.Str,
		}
		subFiles = append(subFiles, file)
	} else {
		files, ok := dict["files"]
		if !ok || files.List == nil || len(files.List) == 0 {
			return nil, errors.New("info dictionary missing keys length and files")
		}
		subFiles, finalLen, err = parseFiles(files.List)
		if err != nil {
			return nil, err
		}
	}

	pieces, err := splitPieces(piece.Str)
	if err != nil {
		return nil, err
	}
	return &TorrentFile{
		Announce: ann,
		Info: &Info{
			Hash:        ben.Hash,
			Length:      finalLen,
			Files:       subFiles,
			Name:        name.Str,
			PieceLength: pieceLen.Int,
			Pieces:      pieces},
	}, nil
}

// OpenTorrent returns a TorrentFile by reading a file at a certain path
func OpenTorrent(path string) (*TorrentFile, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	bencode, err := decode(bufio.NewReader(file), new(bytes.Buffer), false)
	if err != nil {
		return nil, err
	}
	return prettyTorrentBencode(bencode)
}

// GetPeers returns the list of peers from a torrent file and client ID
func (t *TorrentFile) GetPeers(clientID [20]byte) (*TrackerResponse, error) {
	for _, u := range t.Announce {
		switch u.Scheme {
		case "http", "https":
			return t.Info.getPeersHTTPS(clientID, u)
		case "udp", "udp4", "udp6":
			return t.getPeersUDP(clientID)
		default:
			continue
		}
	}
	return nil, errors.New("none of the trackers urls could be parsed")
}

// connectToUDP tries to connect to a UDP tracker
// returns a connection ID if successful
func connectToUDP(conn *net.UDPConn) (uint64, error) {
	var protocolID uint64 = 0x41727101980
	transactionID := rand.Uint32()
	req := make([]byte, 16)

	binary.BigEndian.PutUint64(req, protocolID)
	binary.BigEndian.PutUint32(req[8:], aConnect)
	binary.BigEndian.PutUint32(req[12:], transactionID)

	_, err := conn.Write(req)
	if err != nil {
		return 0, err
	}
	// response format is:
	// uint32 action
	// uint32 transaction_id
	// uint64 connection_id
	res := make([]byte, 16)
	resLen, err := conn.Read(res)
	if err != nil {
		return 0, err
	}
	if resLen != 16 {
		return 0, fmt.Errorf("expected response size 16 got %d instead", resLen)
	}
	resAction := binary.BigEndian.Uint32(res[:4])
	if resAction != uint32(0) {
		return 0, fmt.Errorf("expected action of 0 got %d instead", resAction)
	}
	resTransactionID := binary.BigEndian.Uint32(res[4:8])
	if resTransactionID != transactionID {
		return 0, errors.New("received a different transaction_id")
	}
	return binary.BigEndian.Uint64(res[8:]), nil
}

// getPeersUDP returns the list of peers using udp from a torrent file and client ID
// see http://www.bittorrent.org/beps/bep_0015.html for more detail
func (t *TorrentFile) getPeersUDP(clientID [20]byte) (*TrackerResponse, error) {
	i := 0
	conns := make([]udpConn, len(t.Announce))
	for _, u := range t.Announce {
		if u.Scheme != "udp" && u.Scheme != "udp4" && u.Scheme != "udp6" {
			continue
		}
		addr, err := net.ResolveUDPAddr(u.Scheme, u.Host)
		if err != nil {
			continue
		}
		conn, err := net.DialUDP(u.Scheme, nil, addr)
		if err != nil {
			continue
		}
		defer conn.Close()
		conns[i] = udpConn{conn, u.Scheme}
		i++
	}
	conns = conns[:i]

	// shuffle the list of peers
	for j := range conns {
		k := rand.Intn(j + 1)
		conns[j], conns[k] = conns[k], conns[j]
	}
	// since we are using udp, retry 8 times with an increasing deadline
	for try := 0; try < 8; try++ {
		l := len(conns)
		for k := 0; k < l; k++ {
			uConn := conns[0]
			conn := uConn.Conn
			conns = conns[1:]
			conn.SetDeadline(time.Now().Add(15 * (1 << try) * time.Second))
			connID, err := connectToUDP(conn)
			if err != nil {
				// continue on a timeout
				if err, ok := err.(net.Error); ok && err.Timeout() {
					conns = append(conns, uConn)
				}
				continue
			}
			return t.Info.getPeerFromConnectionID(clientID, conn, connID, uConn.Scheme == "udp6")
		}
	}
	return nil, errors.New("timed out after 8 retries")
}
