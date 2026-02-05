package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// Tracker timeouts and limits
const (
	TrackerQueryTimeout = 15 * time.Second // Base timeout for UDP tracker queries
	TrackerMaxRetries   = 8                // Maximum retry attempts for UDP
	httpTimeout         = 30 * time.Second // Timeout for HTTP tracker requests
	portRangeStart      = 6881             // BEP 3 recommended port range
	portRangeEnd        = 6889
)

// TrackerResponse represents the tracker response to a get message
type TrackerResponse struct {
	Interval       int
	PeersAddresses []string
}

// QueryUDPTracker queries a UDP tracker for peers given an info hash
// This is a standalone function that doesn't require a TorrentFile
func QueryUDPTracker(trackerURL *url.URL, infoHash, clientID [20]byte) (*TrackerResponse, error) {
	scheme := trackerURL.Scheme
	if scheme != "udp" && scheme != "udp4" && scheme != "udp6" {
		return nil, fmt.Errorf("invalid scheme %s for UDP tracker", scheme)
	}

	addr, err := net.ResolveUDPAddr(scheme, trackerURL.Host)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve tracker address: %w", err)
	}

	conn, err := net.DialUDP(scheme, nil, addr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to tracker: %w", err)
	}
	defer conn.Close()

	// Retry with exponential backoff
	for try := range TrackerMaxRetries {
		conn.SetDeadline(time.Now().Add(TrackerQueryTimeout * (1 << try)))

		connID, err := connectToUDP(conn)
		if err != nil {
			var netErr net.Error
			if errors.As(err, &netErr) && netErr.Timeout() {
				continue // Retry on timeout
			}
			return nil, err
		}

		return announceUDP(conn, connID, infoHash, clientID, 0, scheme == "udp6")
	}

	return nil, fmt.Errorf("tracker query timed out after %d retries", TrackerMaxRetries)
}

// announceUDP sends an announce request and parses the response
// bytesLeft is the number of bytes left to download (0 if unknown, e.g., for magnets)
func announceUDP(conn *net.UDPConn, connID uint64, infoHash, clientID [20]byte, bytesLeft int64, ipv6 bool) (*TrackerResponse, error) {
	transactionID := rand.Uint32()

	// Build announce request (98 bytes)
	req := make([]byte, 98)
	binary.BigEndian.PutUint64(req, connID)
	binary.BigEndian.PutUint32(req[8:], aAnnounce)
	binary.BigEndian.PutUint32(req[12:], transactionID)
	copy(req[16:], infoHash[:])
	copy(req[36:], clientID[:])
	binary.BigEndian.PutUint64(req[56:], 0)                 // downloaded
	binary.BigEndian.PutUint64(req[64:], uint64(bytesLeft)) // left
	binary.BigEndian.PutUint64(req[72:], 0)                 // uploaded
	binary.BigEndian.PutUint32(req[80:], 0)                 // event: none
	binary.BigEndian.PutUint32(req[84:], 0)                 // IP address
	binary.BigEndian.PutUint32(req[88:], rand.Uint32())     // key
	binary.BigEndian.PutUint32(req[92:], 0xFFFFFFFF)        // num_want: -1 (all)
	binary.BigEndian.PutUint16(req[96:], 6881)              // port

	if _, err := conn.Write(req); err != nil {
		return nil, err
	}

	// Read response
	res := make([]byte, 508)
	n, err := conn.Read(res)
	if err != nil {
		return nil, err
	}
	if n < 20 {
		return nil, fmt.Errorf("response too short: %d bytes", n)
	}
	res = res[:n]

	// Validate response
	if action := binary.BigEndian.Uint32(res); action != aAnnounce {
		return nil, fmt.Errorf("unexpected action: %d", action)
	}
	if txID := binary.BigEndian.Uint32(res[4:]); txID != transactionID {
		return nil, errors.New("transaction ID mismatch")
	}

	interval := int(binary.BigEndian.Uint32(res[8:]))

	// Parse peer list
	peers := res[20:]
	ipSize := net.IPv4len
	if ipv6 {
		ipSize = net.IPv6len
	}
	peerSize := ipSize + 2

	var peerList []string
	for i := 0; i+peerSize <= len(peers); i += peerSize {
		port := binary.BigEndian.Uint16(peers[i+ipSize:])
		if port == 0 {
			break
		}
		ip := net.IP(peers[i : i+ipSize])
		peerList = append(peerList, net.JoinHostPort(ip.String(), strconv.Itoa(int(port))))
	}

	return &TrackerResponse{interval, peerList}, nil
}

// QueryTrackers queries multiple trackers in parallel and collects peers
func QueryTrackers(trackers []*url.URL, infoHash, clientID [20]byte) []string {
	type result struct {
		peers  []string
		source string
	}

	results := make(chan result, len(trackers))

	for _, tracker := range trackers {
		go func(t *url.URL) {
			switch t.Scheme {
			case "udp", "udp4", "udp6":
				resp, err := QueryUDPTracker(t, infoHash, clientID)
				if err != nil {
					results <- result{nil, t.Host}
					return
				}
				results <- result{resp.PeersAddresses, t.Host}
			default:
				// HTTP/HTTPS trackers not supported for magnet links
				results <- result{nil, t.Host}
			}
		}(tracker)
	}

	seen := make(map[string]bool)
	var allPeers []string

	for range trackers {
		r := <-results
		for _, p := range r.peers {
			if !seen[p] {
				seen[p] = true
				allPeers = append(allPeers, p)
			}
		}
	}

	return allPeers
}

// CollectPeers is a helper to deduplicate peers from multiple sources
type PeerCollector struct {
	seen  map[string]bool
	peers []string
}

// NewPeerCollector creates a new peer collector
func NewPeerCollector() *PeerCollector {
	return &PeerCollector{
		seen:  make(map[string]bool),
		peers: nil,
	}
}

// Add adds peers from a source, returning the number of new peers added
func (c *PeerCollector) Add(peers []string, source string) int {
	added := 0
	for _, p := range peers {
		if !c.seen[p] {
			c.seen[p] = true
			c.peers = append(c.peers, p)
			added++
		}
	}
	return added
}

// Peers returns all collected peers
func (c *PeerCollector) Peers() []string {
	return c.peers
}

// Count returns the number of peers collected
func (c *PeerCollector) Count() int {
	return len(c.peers)
}

// --- HTTP Tracker Support ---

// QueryHTTPTracker queries an HTTP/HTTPS tracker for peers
func QueryHTTPTracker(trackerURL *url.URL, infoHash, clientID [20]byte, bytesLeft int) (*TrackerResponse, error) {
	// Try ports in the standard BitTorrent range
	for port := portRangeStart; port <= portRangeEnd; port++ {
		announceURL := buildAnnounceURL(trackerURL, infoHash, clientID, port, bytesLeft)
		resp, err := getTrackerResponse(announceURL)
		if err == nil {
			return resp, nil
		}
	}
	return nil, fmt.Errorf("HTTP tracker query failed on all ports")
}

// buildAnnounceURL builds the URL to call the tracker
func buildAnnounceURL(u *url.URL, infoHash, clientID [20]byte, port, bytesLeft int) string {
	params := url.Values{
		"info_hash":  []string{string(infoHash[:])},
		"peer_id":    []string{string(clientID[:])},
		"port":       []string{strconv.Itoa(port)},
		"uploaded":   []string{"0"},
		"downloaded": []string{"0"},
		"left":       []string{strconv.Itoa(bytesLeft)},
		"compact":    []string{"1"},
	}
	result := *u
	result.RawQuery = params.Encode()
	return result.String()
}

// getTrackerResponse performs the HTTP GET announce call
func getTrackerResponse(announceURL string) (*TrackerResponse, error) {
	client := &http.Client{Timeout: httpTimeout}
	res, err := client.Get(announceURL)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tracker returned status %s", res.Status)
	}

	bencode, err := decode(bufio.NewReader(res.Body), new(bytes.Buffer), false)
	if err != nil {
		return nil, err
	}

	return parseTrackerResponse(bencode)
}

// parseTrackerResponse parses a bencoded tracker response
func parseTrackerResponse(ben *bencode) (*TrackerResponse, error) {
	dic := ben.Dict
	if dic == nil {
		return nil, errors.New("tracker response has no dictionary")
	}

	if failure, ok := dic["failure reason"]; ok {
		return nil, fmt.Errorf("tracker failure: %s", failure.Str)
	}

	interval, ok := dic["interval"]
	if !ok || interval.Int == 0 {
		return nil, errors.New("tracker response missing interval")
	}

	peers, ok := dic["peers"]
	if !ok || peers.Str == "" {
		return nil, errors.New("tracker response missing peers")
	}

	peerList, err := parseCompactPeers(peers.Str, false)
	if err != nil {
		return nil, err
	}

	// Also parse IPv6 peers if present
	if peers6, ok := dic["peers6"]; ok && peers6.Str != "" {
		if parsed, err := parseCompactPeers(peers6.Str, true); err == nil {
			peerList = append(peerList, parsed...)
		}
	}

	return &TrackerResponse{
		Interval:       interval.Int,
		PeersAddresses: peerList,
	}, nil
}

// parseCompactPeers parses a compact peer list (BEP 23)
func parseCompactPeers(peers string, ipv6 bool) ([]string, error) {
	data := []byte(peers)
	ipSize := net.IPv4len
	if ipv6 {
		ipSize = net.IPv6len
	}
	peerSize := ipSize + 2

	if len(data)%peerSize != 0 {
		return nil, fmt.Errorf("invalid peer list length %d (not divisible by %d)", len(data), peerSize)
	}

	result := make([]string, 0, len(data)/peerSize)
	for i := 0; i < len(data); i += peerSize {
		ip := net.IP(data[i : i+ipSize])
		port := binary.BigEndian.Uint16(data[i+ipSize:])
		result = append(result, net.JoinHostPort(ip.String(), strconv.Itoa(int(port))))
	}
	return result, nil
}
