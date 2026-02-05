package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"strconv"
)

// BitTorrent client port range (BEP 3 recommends 6881-6889)
const (
	portRangeStart = 6881
	portRangeEnd   = 6889
)

// SubFile represents a subfile in the case of multi file torrents
type SubFile struct {
	CumStart int    // start of the file
	Length   int    // length of the file
	Path     string // path to the file
}

// TorrentInfo represents the info dictionary for a torrent
type TorrentInfo struct {
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
		return nil, fmt.Errorf("pieces has a length not divisible by 20: %d", len(buff))
	}
	hashes := make([][20]byte, len(buff)/20)
	for i := range hashes {
		copy(hashes[i][:], buff[i*20:(i+1)*20])
	}
	return hashes, nil
}

// parseFiles parses the files into a slice of subFile
// also returns the total file length
func parseFiles(files []bencode) ([]SubFile, int, error) {
	res := make([]SubFile, len(files))
	totalLen := 0
	for i, file := range files {
		dic := file.Dict
		if dic == nil {
			return nil, 0, fmt.Errorf("file %d has no dictionary", i)
		}
		length, ok := dic["length"]
		if !ok {
			return nil, 0, fmt.Errorf("file %d missing key length", i)
		}
		if length.Int <= 0 {
			return nil, 0, fmt.Errorf("file %d has a negative value for length: %d", i, length.Int)
		}

		path, ok := dic["path"]
		if !ok || path.List == nil || len(path.List) == 0 {
			return nil, 0, fmt.Errorf("file %d missing key path", i)
		}
		pathParts := make([]string, len(path.List))
		for j, p := range path.List {
			pathParts[j] = p.Str
		}
		paths := filepath.Join(pathParts...)
		res[i] = SubFile{
			CumStart: totalLen,
			Length:   length.Int,
			Path:     paths,
		}
		totalLen += length.Int
	}
	return res, totalLen, nil
}

// prettyBencodeInfo parses the bencode as an TorrentInfo
func prettyBencodeInfo(info *bencode, hash [20]byte) (*TorrentInfo, error) {
	dict := info.Dict
	piece, ok := dict["pieces"]
	if !ok || piece.Str == "" {
		return nil, errors.New("info dictionary missing key pieces")
	}

	name, ok := dict["name"]
	if !ok || name.Str == "" {
		return nil, errors.New("info dictionary missing key name")
	}

	pieceLen, ok := dict["piece length"]
	if !ok {
		return nil, errors.New("info dictionary missing key piece length")
	}
	if pieceLen.Int <= 0 {
		return nil, fmt.Errorf("negative value for piece length: %d", pieceLen.Int)
	}

	finalLen := 0
	var subFiles []SubFile
	var err error
	// in case of single file, there is a length key
	length, ok := dict["length"]
	if ok {
		if length.Int < 0 {
			return nil, fmt.Errorf("negative value for length: %d", length.Int)
		}
		finalLen = length.Int
		file := SubFile{
			Length: finalLen,
			Path:   name.Str,
		}
		subFiles = append(subFiles, file)
	} else {
		files, ok := dict["files"]
		if !ok || files.List == nil || len(files.List) == 0 {
			return nil, errors.New("info dictionary missing keys length and files")
		}
		subFiles, finalLen, err = parseFiles(files.List)
		if err != nil {
			return nil, err
		}
	}

	pieces, err := splitPieces(piece.Str)
	if err != nil {
		return nil, err
	}
	return &TorrentInfo{
		Hash:        hash,
		Length:      finalLen,
		Files:       subFiles,
		Name:        name.Str,
		PieceLength: pieceLen.Int,
		Pieces:      pieces}, nil
}

// Multi returns true if there are multiple files
func (inf *TorrentInfo) Multi() bool {
	return len(inf.Files) > 1
}

// getPeersHTTPS returns the list of peers using https from an info dictionary and client ID
func (inf *TorrentInfo) getPeersHTTPS(clientID [20]byte, url *url.URL) (*TrackerResponse, error) {
	var res *TrackerResponse
	var err error
	// Try ports in the standard BitTorrent range
	for port := portRangeStart; port <= portRangeEnd && res == nil; port++ {
		trackerURL := inf.announceURL(clientID, url, port)
		res, err = getTrackerResponse(trackerURL)
		if err == nil {
			return res, nil
		}
	}
	return nil, err
}

// announceURL builds the url to call the announcer from a peer id and a port number
func (inf *TorrentInfo) announceURL(id [20]byte, u *url.URL, port int) string {
	param := url.Values{
		"info_hash":  []string{string(inf.Hash[:])},
		"peer_id":    []string{string(string(id[:]))},
		"port":       []string{strconv.Itoa(port)},
		"uploaded":   []string{"0"},
		"downloaded": []string{"0"},
		"left":       []string{strconv.Itoa(inf.Length)},
		"compact":    []string{"1"},
	}
	u.RawQuery = param.Encode()
	return u.String()
}

// ParseInfo parses a bencoded dictionary as an TorrentInfo struct
func ParseInfo(info []byte, hash [20]byte) (*TorrentInfo, error) {
	ben, err := decode(bufio.NewReader(bytes.NewReader(info)), new(bytes.Buffer), false)
	if err != nil {
		return nil, err
	}
	return prettyBencodeInfo(ben, hash)
}
