package main

import (
	"encoding/base32"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"
)

// Magnet represents a parsed magnet link
// See BEP 9: http://bittorrent.org/beps/bep_0009.html
type Magnet struct {
	Hash          [20]byte   // xt: exact topic (info hash)
	Name          string     // dn: display name
	TrackersURL   []*url.URL // tr: tracker URLs
	PeerAddresses []string   // x.pe: peer addresses (BEP 9)
	WebSeeds      []string   // ws: web seeds (BEP 19)
	ExactSource   string     // xs: exact source (URL to .torrent)
}

// ParseMagnet parses a magnet link into a Magnet struct
func ParseMagnet(m string) (*Magnet, error) {
	if !strings.HasPrefix(m, "magnet:?") {
		return nil, fmt.Errorf("invalid magnet link: must start with 'magnet:?'")
	}

	link, err := url.Parse(m)
	if err != nil {
		return nil, fmt.Errorf("failed to parse magnet URL: %w", err)
	}

	query := link.Query()

	// Parse info hash (required)
	hash, err := parseInfoHash(query)
	if err != nil {
		return nil, err
	}

	// Parse display name (optional)
	name := ""
	if dn, ok := query["dn"]; ok && len(dn) > 0 {
		name = dn[0]
	}

	// Parse trackers (optional)
	var trackers []*url.URL
	if tr, ok := query["tr"]; ok {
		for _, t := range tr {
			if u, err := url.Parse(t); err == nil {
				trackers = append(trackers, u)
			}
		}
	}

	// Parse peer addresses (optional, BEP 9 extension)
	var peerAddresses []string
	if pe, ok := query["x.pe"]; ok {
		peerAddresses = pe
	}

	// Parse web seeds (optional, BEP 19)
	var webSeeds []string
	if ws, ok := query["ws"]; ok {
		webSeeds = ws
	}

	// Parse exact source (optional)
	exactSource := ""
	if xs, ok := query["xs"]; ok && len(xs) > 0 {
		exactSource = xs[0]
	}

	return &Magnet{
		Hash:          hash,
		Name:          name,
		TrackersURL:   trackers,
		PeerAddresses: peerAddresses,
		WebSeeds:      webSeeds,
		ExactSource:   exactSource,
	}, nil
}

// parseInfoHash extracts the 20-byte info hash from the magnet query
func parseInfoHash(query url.Values) ([20]byte, error) {
	var hash [20]byte

	xts, ok := query["xt"]
	if !ok || len(xts) == 0 {
		return hash, fmt.Errorf("magnet link missing 'xt' parameter")
	}

	xt := xts[0]

	// Handle different xt formats
	var encHash string
	switch {
	case strings.HasPrefix(xt, "urn:btih:"):
		encHash = strings.TrimPrefix(xt, "urn:btih:")
	case strings.HasPrefix(xt, "urn:btmh:"):
		// Multihash format (BEP 52) - extract the hash portion
		return hash, fmt.Errorf("multihash (urn:btmh) not yet supported")
	default:
		return hash, fmt.Errorf("unsupported xt format: %s", xt)
	}

	// Decode the hash (hex or base32)
	switch len(encHash) {
	case 40:
		// Hex encoded (40 chars = 20 bytes)
		decoded, err := hex.DecodeString(encHash)
		if err != nil {
			return hash, fmt.Errorf("invalid hex hash: %w", err)
		}
		copy(hash[:], decoded)
	case 32:
		// Base32 encoded (32 chars = 20 bytes)
		decoded, err := base32.StdEncoding.DecodeString(strings.ToUpper(encHash))
		if err != nil {
			return hash, fmt.Errorf("invalid base32 hash: %w", err)
		}
		copy(hash[:], decoded)
	default:
		return hash, fmt.Errorf("invalid hash length %d (expected 32 or 40)", len(encHash))
	}

	return hash, nil
}

// HasTrackers returns true if the magnet has any tracker URLs
func (m *Magnet) HasTrackers() bool {
	return len(m.TrackersURL) > 0
}

// HasPeers returns true if the magnet has any peer addresses
func (m *Magnet) HasPeers() bool {
	return len(m.PeerAddresses) > 0
}

// InfoHashHex returns the info hash as a hex string
func (m *Magnet) InfoHashHex() string {
	return hex.EncodeToString(m.Hash[:])
}

// DisplayName returns the display name, or a fallback based on the hash
func (m *Magnet) DisplayName() string {
	if m.Name != "" {
		return m.Name
	}
	return m.InfoHashHex()[:16] + "..."
}
