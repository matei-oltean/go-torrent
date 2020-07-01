package main

import (
	"encoding/json"
	"io/ioutil"
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
		ioutil.WriteFile(referencePath, serialised, 0644)
	}

	expected := &Magnet{}
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
	if !reflect.DeepEqual(magnet, expected) {
		t.Error("Parsed torrentfile is not equal to the reference.")
	}
}
