package fileutils

import (
	"encoding/base32"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"
)

// Magnet represents the parsing of a magnet link
// only Hash is assured to be present
// see http://bittorrent.org/beps/bep_0009.html
type Magnet struct {
	Hash          [20]byte
	Name          string
	TrackersURL   []*url.URL
	PeerAddresses []string
}

// ParseMagnet parses a magnet link
func ParseMagnet(m string) (*Magnet, error) {
	link, err := url.Parse(m)
	if err != nil {
		return nil, err
	}
	query := link.Query()
	xts, ok := query["xt"]
	if !ok {
		return nil, fmt.Errorf("magnet link %s is missing parameter \"xt\"", m)
	}
	xt := strings.Split(xts[0], "urn:btih:")
	if len(xt) != 2 {
		return nil, fmt.Errorf("magnet link %s is missing parameter \"urn:btih:\"", m)
	}
	encHash := xt[1]
	var hash [20]byte
	if len(encHash) == 40 {
		// hex encoded
		decHash, err := hex.DecodeString(encHash)
		if err != nil {
			return nil, err
		}
		copy(hash[:], decHash)
	} else if len(encHash) == 32 {
		// base 32 encoded
		decHash, err := base32.StdEncoding.DecodeString(encHash)
		if err != nil {
			return nil, err
		}
		copy(hash[:], decHash)
	} else {
		return nil, fmt.Errorf("magnet link %s has hash %s of incorrect size %d (expected 32 or 40)", m, encHash, len(encHash))
	}
	var name string
	n, ok := query["dn"]
	if ok {
		name = n[0]
	}
	var trackers []*url.URL
	tr, ok := query["tr"]
	if ok {
		for _, t := range tr {
			url, err := url.Parse(t)
			if err == nil {
				trackers = append(trackers, url)
			}
		}
	}
	var peerAddresses []string
	addr, ok := query["x.pe"]
	if ok {
		peerAddresses = addr
	}
	return &Magnet{
		Hash:          hash,
		Name:          name,
		TrackersURL:   trackers,
		PeerAddresses: peerAddresses,
	}, nil
}
