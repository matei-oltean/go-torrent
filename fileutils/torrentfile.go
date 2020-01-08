package fileutils

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
)

type bencode struct {
	Dict   map[string]bencode
	List   []bencode
	String string
	Int    int64
}

// TorrentFile represents a flattened torrent file
// TODO Add hash for the information struct
type TorrentFile struct {
	Announce    string
	Length      uint64
	Name        string
	PieceLength uint64
	Pieces      [][20]byte
}

func decode(reader *bufio.Reader) (*bencode, error) {
	ben := &bencode{}
	char, err := reader.ReadByte()
	if err != nil {
		return nil, err
	}
	switch char {
	case 'd':
		dict := make(map[string]bencode)
		for {
			ch, err := reader.ReadByte()
			if err != nil {
				return nil, err
			}
			if ch == 'e' {
				ben.Dict = dict
				return ben, nil
			}
			reader.UnreadByte()
			val, err := decode(reader)
			if err != nil {
				return nil, err
			}
			key := val.String
			if key == "" {
				return nil, errors.New("Dictionary has a non string key")
			}

			if val, err = decode(reader); err != nil {
				return nil, err
			}
			dict[key] = *val
		}
	case 'i':
		intStr, err := reader.ReadString('e')
		if err != nil {
			return nil, err
		}
		intStr = intStr[:len(intStr)-1]
		integer, err := strconv.ParseInt(intStr, 10, 64)
		if err != nil {
			return nil, err
		}
		ben.Int = integer
		return ben, nil
	case 'l':
		list := make([]bencode, 5)
		for {
			ch, err := reader.ReadByte()
			if err != nil {
				return nil, err
			}
			if ch == 'e' {
				ben.List = list
				return ben, nil
			}
			reader.UnreadByte()
			value, err := decode(reader)
			if err != nil {
				return nil, err
			}
			list = append(list, *value)
		}
	default:
		reader.UnreadByte()
		strLen, err := reader.ReadString(':')
		if err != nil {
			return nil, err
		}
		strLen = strLen[:len(strLen)-1]
		length, err := strconv.ParseUint(strLen, 10, 64)
		if err != nil {
			return nil, err
		}
		buff := make([]byte, length)
		n, err := io.ReadFull(reader, buff)
		if err != nil {
			return nil, err
		}
		if n != int(length) {
			return nil, errors.New("String of the wrong length")
		}
		ben.String = string(buff)
		return ben, nil
	}
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

func prettyBencode(ben *bencode) (*TorrentFile, error) {
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
	bencode, err := decode(bufio.NewReader(file))
	if err != nil {
		return nil, err
	}
	return prettyBencode(bencode)
}
