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
	"os"
	"time"
)

// the actions for an udp transfer
const (
	aConnect uint32 = iota
	aAnnounce
	aScrape
	aError
)

// UDP tracker constants
const (
	udpMaxRetries  = 8                // max retry attempts for UDP tracker
	udpBaseTimeout = 15 * time.Second // base timeout, doubles each retry
)

// udpConn is a simple struct with a connection and its scheme
type udpConn struct {
	Conn   *net.UDPConn
	Scheme string
}

// TorrentFile represents a flattened torrent file
type TorrentFile struct {
	Announce []*url.URL
	Info     *TorrentInfo
}

// parseAnnounceList parses and flattens the announce list
// it should be a list of lists of urls (as strings)
func parseAnnounceList(l []bencode) []*url.URL {
	q := []*url.URL{}
	for _, subL := range l {
		if len(subL.List) == 0 {
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

// prettyTorrentBencode parses the bencode as a TorrentFile
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

	bInfo, ok := dic["info"]
	if !ok || bInfo.Dict == nil {
		return nil, errors.New("torrent file missing info key")
	}

	info, err := prettyBencodeInfo(&bInfo, ben.Hash)
	if err != nil {
		return nil, err
	}

	return &TorrentFile{
		Announce: ann,
		Info:     info,
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
	// since we are using udp, retry with an increasing deadline
	for try := range udpMaxRetries {
		l := len(conns)
		for range l {
			uConn := conns[0]
			conn := uConn.Conn
			conns = conns[1:]
			conn.SetDeadline(time.Now().Add(udpBaseTimeout * (1 << try)))
			connID, err := connectToUDP(conn)
			if err != nil {
				// continue on a timeout
				var netErr net.Error
				if errors.As(err, &netErr) && netErr.Timeout() {
					conns = append(conns, uConn)
				}
				continue
			}
			ipv6 := uConn.Scheme == "udp6"
			return announceUDP(conn, connID, t.Info.Hash, clientID, int64(t.Info.Length), ipv6)
		}
	}
	return nil, fmt.Errorf("timed out after %d retries", udpMaxRetries)
}
