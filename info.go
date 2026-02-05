package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"net/url"
	"path/filepath"
	"strconv"
)

// BitTorrent client port range (BEP 3 recommends 6881-6889)
const (
	portRangeStart = 6881
	portRangeEnd   = 6889
)

// SubFile represents a subfile in the case of multi file torrents
type SubFile struct {
	CumStart int    // start of the file
	Length   int    // length of the file
	Path     string // path to the file
}

// TorrentInfo represents the info dictionary for a torrent
type TorrentInfo struct {
	Hash        [20]byte
	Length      int
	Files       []SubFile
	Name        string
	PieceLength int
	Pieces      [][20]byte
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
		pathParts := make([]string, len(path.List))
		for j, p := range path.List {
			pathParts[j] = p.Str
		}
		paths := filepath.Join(pathParts...)
		res[i] = SubFile{
			CumStart: totalLen,
			Length:   length.Int,
			Path:     paths,
		}
		totalLen += length.Int
	}
	return res, totalLen, nil
}

// prettyBencodeInfo parses the bencode as an TorrentInfo
func prettyBencodeInfo(info *bencode, hash [20]byte) (*TorrentInfo, error) {
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
	var err error
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
	return &TorrentInfo{
		Hash:        hash,
		Length:      finalLen,
		Files:       subFiles,
		Name:        name.Str,
		PieceLength: pieceLen.Int,
		Pieces:      pieces}, nil
}

// Multi returns true if there are multiple files
func (inf *TorrentInfo) Multi() bool {
	return len(inf.Files) > 1
}

// getPeersHTTPS returns the list of peers using https from an info dictionary and client ID
func (inf *TorrentInfo) getPeersHTTPS(clientID [20]byte, url *url.URL) (*TrackerResponse, error) {
	var res *TrackerResponse
	var err error
	// Try ports in the standard BitTorrent range
	for port := portRangeStart; port <= portRangeEnd && res == nil; port++ {
		trackerURL := inf.announceURL(clientID, url, port)
		res, err = getTrackerResponse(trackerURL)
		if err == nil {
			return res, nil
		}
	}
	return nil, err
}

// announceURL builds the url to call the announcer from a peer id and a port number
func (inf *TorrentInfo) announceURL(id [20]byte, u *url.URL, port int) string {
	param := url.Values{
		"info_hash":  []string{string(inf.Hash[:])},
		"peer_id":    []string{string(string(id[:]))},
		"port":       []string{strconv.Itoa(port)},
		"uploaded":   []string{"0"},
		"downloaded": []string{"0"},
		"left":       []string{strconv.Itoa(inf.Length)},
		"compact":    []string{"1"},
	}
	u.RawQuery = param.Encode()
	return u.String()
}

// getPeerFromConnectionID gets the list of UDP peers from a connection id
// ipv6 is true for an ipv6 connection
func (inf *TorrentInfo) getPeerFromConnectionID(clientID [20]byte, conn *net.UDPConn, connID uint64, ipv6 bool) (*TrackerResponse, error) {
	transactionID := rand.Uint32() // random id

	buf := new(bytes.Buffer)
	binary.Write(buf, binary.BigEndian, int32(-1)) // -1 means all peers
	expectedPeers := buf.Bytes()

	req := make([]byte, 98)

	binary.BigEndian.PutUint64(req, connID)
	binary.BigEndian.PutUint32(req[8:], aAnnounce)
	binary.BigEndian.PutUint32(req[12:], transactionID)
	copy(req[16:], inf.Hash[:])
	copy(req[36:], clientID[:])
	binary.BigEndian.PutUint64(req[56:], uint64(0))          // downloaded; downloaded bytes for this session
	binary.BigEndian.PutUint64(req[64:], uint64(inf.Length)) // left; bytes left to download
	binary.BigEndian.PutUint64(req[72:], uint64(0))          // uploaded; uploaded bytes for this session
	binary.BigEndian.PutUint32(req[80:], uint32(0))          // event; 0: none, 1: completed, 2: started, 3: stopped
	binary.BigEndian.PutUint32(req[84:], uint32(0))          // IP address; not usable for IPv6: should stay 0
	binary.BigEndian.PutUint32(req[88:], rand.Uint32())      // key; unique random
	copy(req[92:], expectedPeers)                            // num_want; number of expected peers; -1 means all
	binary.BigEndian.PutUint16(req[96:], uint16(6881))       // port; should be between 6881 and 6889
	_, err := conn.Write(req)
	if err != nil {
		return nil, err
	}

	// response format is:
	// uint32 action
	// uint32 transaction ID
	// uint32 interval
	// uint32 leechers
	// uint32 seeders
	// list of addresses: uint32 IPv4/16 byte IPv6 address + 16 bit port
	res := make([]byte, 508)
	n, err := conn.Read(res)
	if err != nil {
		return nil, err
	}
	if n < 20 {
		return nil, fmt.Errorf("expected a response of length >= 20 got %d instead", n)
	}
	res = res[:n]
	resAction := binary.BigEndian.Uint32(res)
	if resAction != aAnnounce {
		return nil, fmt.Errorf("expected action 1 got %d instead", resAction)
	}
	resTransactionID := binary.BigEndian.Uint32(res[4:])
	if resTransactionID != transactionID {
		return nil, errors.New("received a different transaction_id")
	}

	interval := int(binary.BigEndian.Uint32(res[8:]))

	peers := res[20:]
	ipSize := net.IPv4len
	if ipv6 {
		ipSize = net.IPv6len
	}
	peerSize := 2 + ipSize
	peerList := make([]string, len(peers)/peerSize)
	i := 0
	for ; i < len(peers); i += peerSize {
		// if the port is null, we have reached the end of the peers
		port := int(binary.BigEndian.Uint16(peers[i+ipSize:]))
		if port == 0 {
			break
		}
		ip := net.IP(peers[i : i+ipSize])
		peerList[i/peerSize] = net.JoinHostPort(ip.String(), strconv.Itoa(port))
	}
	peerList = peerList[:i/peerSize]
	return &TrackerResponse{interval, peerList}, nil
}

// ParseInfo parses a bencoded dictionary as an TorrentInfo struct
func ParseInfo(info []byte, hash [20]byte) (*TorrentInfo, error) {
	ben, err := decode(bufio.NewReader(bytes.NewReader(info)), new(bytes.Buffer), false)
	if err != nil {
		return nil, err
	}
	return prettyBencodeInfo(ben, hash)
}
