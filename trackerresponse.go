package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"
)

// httpTimeout is the timeout for HTTP tracker requests
const httpTimeout = 30 * time.Second

// TrackerResponse represents the tracker response to a get message
type TrackerResponse struct {
	Interval       int
	PeersAddresses []string
}

func parsePeerList(peers string, ipv6 bool) ([]string, error) {
	peerBytes := []byte(peers)
	ipSize := net.IPv4len
	if ipv6 {
		ipSize = net.IPv6len
	}
	peerSize := 2 + ipSize
	if len(peerBytes)%peerSize != 0 {
		return nil, fmt.Errorf("Peers has a length not divisible by %d: %d", peerSize, len(peerBytes))
	}
	peerList := make([]string, len(peerBytes)/peerSize)
	for i := 0; i < len(peerBytes); i += peerSize {
		ip := net.IP(peerBytes[i : i+ipSize])
		port := int(binary.BigEndian.Uint16(peerBytes[i+ipSize : i+peerSize]))
		peerList[i/peerSize] = net.JoinHostPort(ip.String(), strconv.Itoa(port))
	}
	return peerList, nil
}

func prettyTrackerBencode(ben *bencode) (*TrackerResponse, error) {
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

	if peers6, ok := dic["peers6"]; ok && peers6.Str != "" {
		if parsed, err := parsePeerList(peers6.Str, true); err == nil {
			peerList = append(peerList, parsed...)
		}
	}

	return &TrackerResponse{
		Interval:       interval.Int,
		PeersAddresses: peerList,
	}, nil
}

// getTrackerResponse performs the get announce call
func getTrackerResponse(announceURL string) (*TrackerResponse, error) {
	client := &http.Client{Timeout: httpTimeout}
	res, err := client.Get(announceURL)
	if err != nil {
		return nil, err
	}

	defer res.Body.Close()

	if res.StatusCode != 200 {
		return nil, fmt.Errorf("Received a non 200 code from the tracker: %s", res.Status)
	}

	bencode, err := decode(bufio.NewReader(res.Body), new(bytes.Buffer), false)
	if err != nil {
		return nil, err
	}

	return prettyTrackerBencode(bencode)
}
