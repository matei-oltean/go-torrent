package fileutils

import (
	"bytes"
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
const bigBuckBunnyMagnet string = "magnet:?xt=urn:btih:dd8255ecdc7ca55fb0bbf81323d87062db1f6d1c&dn=Big+Buck+Bunny&tr=udp%3A%2F%2Fexplodie.org%3A6969&tr=udp%3A%2F%2Ftracker.coppersurfer.tk%3A6969&tr=udp%3A%2F%2Ftracker.empire-js.us%3A1337&tr=udp%3A%2F%2Ftracker.leechers-paradise.org%3A6969&tr=udp%3A%2F%2Ftracker.opentrackr.org%3A1337&tr=wss%3A%2F%2Ftracker.btorrent.xyz&tr=wss%3A%2F%2Ftracker.fastcast.nz&tr=wss%3A%2F%2Ftracker.openwebtorrent.com&ws=https%3A%2F%2Fwebtorrent.io%2Ftorrents%2F&xs=https%3A%2F%2Fwebtorrent.io%2Ftorrents%2Fbig-buck-bunny.torrent"

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

func TestAnnounceURL(t *testing.T) {
	const referenceURL string = "announceURL"
	torrent, err := OpenTorrent(filepath.Join(testFolder, torrentFile))
	if err != nil {
		t.Error(err)
		return
	}
	port := 6882
	id := [20]byte{'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h', 'i', 'j', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9'}
	announceURL := torrent.announceURL(id, torrent.Announce[0], port)

	referencePath := filepath.Join(testFolder, referenceURL)
	expectedURL, _ := ioutil.ReadFile(referencePath)

	if !bytes.Equal([]byte(announceURL), expectedURL) {
		t.Error("Crafted URL is not equal to the reference.")
	}
}
