package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// set write to true to rewrite over the reference
const write bool = false

const testFolder string = "testData"
const torrentFile string = "debian-10.2.0-amd64-netinst.iso.torrent"

func TestOpenTorrentMultipleFiles(t *testing.T) {
	const multipleTorrentFile string = "big-buck-bunny.torrent"
	const referenceFile string = multipleTorrentFile + ".reference.json"
	torrent, err := OpenTorrent(filepath.Join(testFolder, multipleTorrentFile))
	if err != nil {
		t.Error(err)
		return
	}

	referencePath := filepath.Join(testFolder, referenceFile)
	if write {
		serialised, _ := json.MarshalIndent(torrent, "", " ")
		os.WriteFile(referencePath, serialised, 0644)
	}

	expected := &TorrentFile{}
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
	if !reflect.DeepEqual(torrent, expected) {
		t.Error("Parsed torrentfile is not equal to the reference.")
	}
}

func TestOpenTorrent(t *testing.T) {
	const referenceFile string = torrentFile + ".reference.json"
	torrent, err := OpenTorrent(filepath.Join(testFolder, torrentFile))
	if err != nil {
		t.Error(err)
		return
	}

	referencePath := filepath.Join(testFolder, referenceFile)
	if write {
		serialised, _ := json.MarshalIndent(torrent, "", " ")
		os.WriteFile(referencePath, serialised, 0644)
	}

	expected := &TorrentFile{}
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
	if !reflect.DeepEqual(torrent, expected) {
		t.Error("Parsed torrentfile is not equal to the reference.")
	}
}

func TestAnnounceURL(t *testing.T) {
	const referenceURL string = "announceURL"
	torrent, err := OpenTorrent(filepath.Join(testFolder, torrentFile))
	if err != nil {
		t.Error(err)
		return
	}
	port := 6882
	id := [20]byte{'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h', 'i', 'j', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9'}
	announceURL := buildAnnounceURL(torrent.Announce[0], torrent.Info.Hash, id, port, torrent.Info.Length)

	referencePath := filepath.Join(testFolder, referenceURL)
	expectedURL, _ := os.ReadFile(referencePath)

	if !bytes.Equal([]byte(announceURL), expectedURL) {
		t.Error("Crafted URL is not equal to the reference.")
	}
}
