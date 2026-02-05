package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
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

// getPeersHTTP returns the list of peers using HTTP from an info dictionary and client ID
func (inf *TorrentInfo) getPeersHTTP(clientID [20]byte, trackerURL *url.URL) (*TrackerResponse, error) {
	return QueryHTTPTracker(trackerURL, inf.Hash, clientID, inf.Length)
}

// ParseInfo parses a bencoded dictionary as an TorrentInfo struct
func ParseInfo(info []byte, hash [20]byte) (*TorrentInfo, error) {
	ben, err := decode(bufio.NewReader(bytes.NewReader(info)), new(bytes.Buffer), false)
	if err != nil {
		return nil, err
	}
	return prettyBencodeInfo(ben, hash)
}
