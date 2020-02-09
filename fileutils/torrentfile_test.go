package fileutils

import (
	"encoding/json"
	"io/ioutil"
	"path/filepath"
	"reflect"
	"testing"
)

// set write to true to rewrite over the reference
const write bool = false

const testFolder string = "testData"
const torrentFile string = "debian-10.2.0-amd64-netinst.iso.torrent"
const referenceFile string = torrentFile + ".reference.json"
const referenceURL string = "announceURL"

func TestOpenTorrent(t *testing.T) {
	torrent, err := OpenTorrent(filepath.Join(testFolder, torrentFile))
	if err != nil {
		t.Error(err)
		return
	}

	referencePath := filepath.Join(testFolder, referenceFile)
	if write {
		serialised, _ := json.Marshal(torrent)
		ioutil.WriteFile(referencePath, serialised, 0644)
	}

	expected := &TorrentFile{}
	reference, err := ioutil.ReadFile(referencePath)
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

func TestGetAnnounceURL(t *testing.T) {
	torrent, err := OpenTorrent(filepath.Join(testFolder, torrentFile))
	if err != nil {
		t.Error(err)
		return
	}
	port := 6882
	id := [20]byte{'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h', 'i', 'j', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9'}
	url, _ := torrent.GetAnnounceURL(id, port)

	referencePath := filepath.Join(testFolder, referenceURL)
	expectedURL, _ := ioutil.ReadFile(referencePath)

	if !reflect.DeepEqual([]byte(url), expectedURL) {
		t.Error("Crafted URL is not equal to the reference.")
	}
}
