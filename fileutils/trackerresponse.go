package fileutils

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"net/http"
)

// Peer represents a peer: its IP and port
type Peer struct {
	IP   net.IP
	Port uint16
}

// TrackerResponse represents the tracker response to a get message
// Peers use IPv4
// Peers6 use IPv6
type TrackerResponse struct {
	Interval uint64
	Peers    []Peer
	Peers6   []Peer
}

func parsePeerList(peers string, ipv6 bool) ([]Peer, error) {
	peerBytes := []byte(peers)
	ipSize := net.IPv4len
	if ipv6 {
		ipSize = net.IPv6len
	}
	peerSize := 2 + ipSize
	if len(peerBytes)%peerSize != 0 {
		return nil, fmt.Errorf("Peers has a length not divisible by %d: %d", peerSize, len(peerBytes))
	}
	peerList := make([]Peer, len(peerBytes)/peerSize)
	for i := 0; i < len(peerBytes); i += peerSize {
		ip := net.IP(peerBytes[i : i+ipSize])
		port := binary.BigEndian.Uint16(peerBytes[i+ipSize : i+peerSize])
		peerList[i/peerSize] = Peer{
			IP:   ip,
			Port: port,
		}
	}
	return peerList, nil
}

func prettyTrackerBencode(ben *Bencode) (*TrackerResponse, error) {
	dic := ben.Dict
	if dic == nil {
		return nil, errors.New("Tracker response has no dictionary")
	}

	if failure, ok := dic["failure reason"]; ok {
		return nil, fmt.Errorf("Tracker response responded with failure: %s", failure.Str)
	}

	interval, ok := dic["interval"]
	if !ok || interval.Int == 0 {
		return nil, errors.New("Tracker response missing interval key")
	}

	peers, ok := dic["peers"]
	if !ok || peers.Str == "" {
		return nil, errors.New("Tracker response missing peers key")
	}

	peerList, err := parsePeerList(peers.Str, false)
	if err != nil {
		return nil, err
	}

	var ip6Peers []Peer = nil
	if peers6, ok := dic["peers6"]; ok && peers6.Str != "" {
		if parsed, err := parsePeerList(peers6.Str, true); err == nil {
			ip6Peers = parsed
		}
	}

	return &TrackerResponse{
		Interval: uint64(interval.Int),
		Peers:    peerList,
		Peers6:   ip6Peers,
	}, nil
}

// GetTrackerResponse performs the get announce call
func GetTrackerResponse(announceURL string) (*TrackerResponse, error) {
	response, err := http.Get(announceURL)
	if err != nil {
		return nil, err
	}
	if response.StatusCode != 200 {
		return nil, fmt.Errorf("Received a non 200 code from the tracker: %s", response.Status)
	}

	bencode, err := decode(bufio.NewReader(response.Body), new(bytes.Buffer), false)
	if err != nil {
		return nil, err
	}

	parsedResponse, err := prettyTrackerBencode(bencode)
	if err != nil {
		return nil, err
	}
	return parsedResponse, nil
}
