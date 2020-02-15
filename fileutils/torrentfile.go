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

// SubFile represents a subfile in the case of multi file torrents
type SubFile struct {
	Length int
	Path   []string
}

// TorrentFile represents a flattened torrent file
type TorrentFile struct {
	Announce    string
	Hash        [20]byte
	Length      int
	Files       []SubFile
	Name        string
	PieceLength int
	Pieces      [][20]byte
}

// splitPieces splits the concatenated hashes of the files into a list of hashes
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

// parseFiles parses the files into a slice of subFile
func parseFiles(files []bencode) ([]SubFile, error) {
	res := make([]SubFile, len(files))
	for i, file := range files {
		dic := file.Dict
		if dic == nil {
			return nil, fmt.Errorf("file %d has no dictionary", i)
		}
		length, ok := dic["length"]
		if !ok {
			return nil, fmt.Errorf("file %d missing key length", i)
		}
		if length.Int <= 0 {
			return nil, fmt.Errorf("file %d has a negative value for length: %d", i, length.Int)
		}

		path, ok := dic["path"]
		if !ok || path.List == nil || len(path.List) == 0 {
			return nil, fmt.Errorf("file %d missing key path", i)
		}
		paths := make([]string, len(path.List))
		for i, p := range path.List {
			paths[i] = p.Str
		}
		res[i] = SubFile{
			Length: length.Int,
			Path:   paths,
		}
	}
	return res, nil
}

func prettyTorrentBencode(ben *bencode) (*TorrentFile, error) {
	dic := ben.Dict
	if dic == nil {
		return nil, errors.New("Torrent file has no dictionary")
	}
	announce, ok := dic["announce"]
	if !ok || announce.Str == "" {
		return nil, errors.New("Torrent file missing announce key")
	}
	info, ok := dic["info"]
	if !ok || info.Dict == nil {
		return nil, errors.New("Torrent file missing info key")
	}
	dict := info.Dict
	for _, key := range [2]string{"name", "pieces"} {
		if elem, ok := dict[key]; !ok || elem.Str == "" {
			return nil, fmt.Errorf("Info dictionary missing key %s", key)
		}
	}

	pieceLen, ok := dict["piece length"]
	if !ok {
		return nil, errors.New("Info dictionary missing key piece length")
	}
	if pieceLen.Int <= 0 {
		return nil, fmt.Errorf("Negative value for piece length: %d", pieceLen.Int)
	}

	finalLen := 0
	name := ""
	var subFiles []SubFile
	// in case of single file, there is a length key
	length, ok := dict["length"]
	if ok {
		if length.Int < 0 {
			return nil, fmt.Errorf("Negative value for length: %d", length.Int)
		}
		finalLen = length.Int
		name = dict["name"].Str
	} else {
		files, ok := dict["files"]
		if !ok || files.List == nil || len(files.List) == 0 {
			return nil, errors.New("Info dictionary missing keys length and files")
		}
		parsedFiles, err := parseFiles(files.List)
		subFiles = parsedFiles
		if err != nil {
			return nil, err
		}
	}

	pieces, err := splitPieces(dict["pieces"].Str)
	if err != nil {
		return nil, err
	}
	return &TorrentFile{
		Announce:    announce.Str,
		Hash:        ben.Hash,
		Length:      finalLen,
		Files:       subFiles,
		Name:        name,
		PieceLength: pieceLen.Int,
		Pieces:      pieces,
	}, nil
}

// OpenTorrent returns a TorrentFile by reading a file at a certain path
func OpenTorrent(path string) (*TorrentFile, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	bencode, err := decode(bufio.NewReader(file), new(bytes.Buffer), false)
	if err != nil {
		return nil, err
	}
	return prettyTorrentBencode(bencode)
}

// AnnounceURL builds the url to call the announcer from a peer id and a port number
func (t *TorrentFile) AnnounceURL(id [20]byte, port int) (string, error) {
	announceURL, err := url.Parse(t.Announce)
	if err != nil {
		return "", err
	}
	parameters := url.Values{
		"info_hash":  []string{string(t.Hash[:])},
		"peer_id":    []string{string(string(id[:]))},
		"port":       []string{strconv.Itoa(port)},
		"uploaded":   []string{"0"},
		"downloaded": []string{"0"},
		"left":       []string{strconv.Itoa(t.Length)},
		"compact":    []string{"1"},
	}
	announceURL.RawQuery = parameters.Encode()
	return announceURL.String(), nil
}
