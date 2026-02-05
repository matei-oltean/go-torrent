package torrent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// set writeMagnet to true to rewrite over the reference
const writeMagnet bool = false

const magnet string = "magnet:?xt=urn:btih:dd8255ecdc7ca55fb0bbf81323d87062db1f6d1c&dn=Big+Buck+Bunny&tr=udp%3A%2F%2Fexplodie.org%3A6969&tr=udp%3A%2F%2Ftracker.coppersurfer.tk%3A6969&tr=udp%3A%2F%2Ftracker.empire-js.us%3A1337&tr=udp%3A%2F%2Ftracker.leechers-paradise.org%3A6969&tr=udp%3A%2F%2Ftracker.opentrackr.org%3A1337&tr=wss%3A%2F%2Ftracker.btorrent.xyz&tr=wss%3A%2F%2Ftracker.fastcast.nz&tr=wss%3A%2F%2Ftracker.openwebtorrent.com&ws=https%3A%2F%2Fwebtorrent.io%2Ftorrents%2F&xs=https%3A%2F%2Fwebtorrent.io%2Ftorrents%2Fbig-buck-bunny.torrent"
const referenceMagnetPath string = "parsedMagnet"

func TestParseMagnet(t *testing.T) {
	referencePath := filepath.Join(testFolder, referenceMagnetPath)

	magnet, err := ParseMagnet(magnet)
	if err != nil {
		t.Error(err)
		return
	}

	if writeMagnet {
		serialised, _ := json.MarshalIndent(magnet, "", " ")
		os.WriteFile(referencePath, serialised, 0644)
	}

	expected := &Magnet{}
	reference, err := os.ReadFile(referencePath)
	if err != nil {
		t.Error(err)
		return
	}
	err = json.Unmarshal(reference, &expected)
	if err != nil {
		t.Error(err)
		return
	}
	if !reflect.DeepEqual(magnet, expected) {
		t.Error("Parsed torrentfile is not equal to the reference.")
	}
}

func TestParseMagnetHex(t *testing.T) {
	m, err := ParseMagnet("magnet:?xt=urn:btih:dd8255ecdc7ca55fb0bbf81323d87062db1f6d1c")
	if err != nil {
		t.Fatal(err)
	}
	expected := [20]byte{0xdd, 0x82, 0x55, 0xec, 0xdc, 0x7c, 0xa5, 0x5f, 0xb0, 0xbb,
		0xf8, 0x13, 0x23, 0xd8, 0x70, 0x62, 0xdb, 0x1f, 0x6d, 0x1c}
	if m.Hash != expected {
		t.Errorf("hash mismatch: got %x, want %x", m.Hash, expected)
	}
}

func TestParseMagnetBase32(t *testing.T) {
	// Base32 uses A-Z and 2-7, must be 32 chars for 20 bytes
	// Test case-insensitive parsing
	m, err := ParseMagnet("magnet:?xt=urn:btih:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA")
	if err != nil {
		t.Fatal(err)
	}
	// All A's in base32 decodes to all zeros
	expected := [20]byte{}
	if m.Hash != expected {
		t.Errorf("hash mismatch: got %x, want %x", m.Hash, expected)
	}

	// Test lowercase also works
	m2, err := ParseMagnet("magnet:?xt=urn:btih:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	if err != nil {
		t.Fatal(err)
	}
	if m.Hash != m2.Hash {
		t.Error("case-insensitive base32 should produce same hash")
	}
}

func TestParseMagnetInvalid(t *testing.T) {
	tests := []struct {
		name   string
		magnet string
	}{
		{"no prefix", "xt=urn:btih:abc123"},
		{"missing xt", "magnet:?dn=test"},
		{"invalid xt format", "magnet:?xt=invalid"},
		{"wrong hash length", "magnet:?xt=urn:btih:abc123"},
		{"invalid hex", "magnet:?xt=urn:btih:zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseMagnet(tt.magnet)
			if err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}

func TestMagnetHelpers(t *testing.T) {
	m := &Magnet{
		Hash: [20]byte{0xdd, 0x82, 0x55, 0xec, 0xdc, 0x7c, 0xa5, 0x5f, 0xb0, 0xbb,
			0xf8, 0x13, 0x23, 0xd8, 0x70, 0x62, 0xdb, 0x1f, 0x6d, 0x1c},
		Name:          "Test",
		PeerAddresses: []string{"1.2.3.4:6881"},
	}

	if !m.HasPeers() {
		t.Error("expected HasPeers() to be true")
	}
	if m.HasTrackers() {
		t.Error("expected HasTrackers() to be false")
	}
	if m.DisplayName() != "Test" {
		t.Errorf("expected DisplayName() 'Test', got '%s'", m.DisplayName())
	}
	if m.InfoHashHex() != "dd8255ecdc7ca55fb0bbf81323d87062db1f6d1c" {
		t.Errorf("unexpected InfoHashHex: %s", m.InfoHashHex())
	}

	// Test DisplayName fallback
	m.Name = ""
	if m.DisplayName() != "dd8255ecdc7ca55f..." {
		t.Errorf("expected fallback display name, got '%s'", m.DisplayName())
	}
}
