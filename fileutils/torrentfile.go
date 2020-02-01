package fileutils

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
)

// TorrentFile represents a flattened torrent file
type TorrentFile struct {
	Announce    string
	Hash        [20]byte
	Length      uint64
	Name        string
	PieceLength uint64
	Pieces      [][20]byte
}

func splitPieces(pieces string) ([][20]byte, error) {
	buff := []byte(pieces)
	if len(buff)%20 != 0 {
		return nil, fmt.Errorf("Pieces has a length not divisible by 20: %d", len(buff))
	}
	hashes := make([][20]byte, len(buff)/20)
	for i := range hashes {
		copy(hashes[i][:], buff[i*20:(i+1)*20])
	}
	return hashes, nil
}

func prettyBencode(ben *Bencode) (*TorrentFile, error) {
	dic := ben.Dict
	if dic == nil {
		return nil, errors.New("Torrent file has no dictionary")
	}
	announce, ok := dic["announce"]
	if !ok || announce.String == "" {
		return nil, errors.New("Torrent file missing announce key")
	}
	info, ok := dic["info"]
	if !ok || info.Dict == nil {
		return nil, errors.New("Torrent file missing info key")
	}
	dict := info.Dict
	for _, key := range [2]string{"name", "pieces"} {
		if elem, ok := dict[key]; !ok || elem.String == "" {
			return nil, fmt.Errorf("Info dictionary missing key %s", key)
		}
	}

	for _, key := range [2]string{"piece length", "length"} {
		elem, ok := dict[key]
		if !ok {
			return nil, fmt.Errorf("Info dictionary missing key %s", key)
		}
		if elem.Int < 0 {
			return nil, fmt.Errorf("Negative value for %s: %d", key, elem.Int)
		}
	}
	pieces, err := splitPieces(dict["pieces"].String)
	if err != nil {
		return nil, err
	}
	return &TorrentFile{
		Announce:    announce.String,
		Hash:        ben.Hash,
		Length:      uint64(dict["length"].Int),
		Name:        dict["name"].String,
		PieceLength: uint64(dict["piece length"].Int),
		Pieces:      pieces,
	}, nil
}

// OpenTorrent returns a TorrentFile by reading a file at a certain path
func OpenTorrent(path string) (*TorrentFile, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	bencode, err := Decode(bufio.NewReader(file), new(bytes.Buffer), false)
	if err != nil {
		return nil, err
	}
	return prettyBencode(bencode)
}

// GetAnnounceURL builds the url to call the announcer from a peer id and a port number
func (t *TorrentFile) GetAnnounceURL(id [20]byte, port uint16) (string, error) {
	announceURL, err := url.Parse(t.Announce)
	if err != nil {
		return "", err
	}
	parameters := url.Values{
		"info_hash":  []string{string(t.Hash[:])},
		"peer_id":    []string{string(string(id[:]))},
		"port":       []string{strconv.Itoa(int(port))},
		"uploaded":   []string{"0"},
		"downloaded": []string{"0"},
		"left":       []string{strconv.Itoa(int(t.Length))},
		"compact":    []string{"1"},
	}
	announceURL.RawQuery = parameters.Encode()
	return announceURL.String(), nil
}
