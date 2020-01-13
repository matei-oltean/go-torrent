package fileutils

import (
	"bufio"
	"bytes"
	"crypto/sha1"
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
	Hash   [20]byte
}

// TorrentFile represents a flattened torrent file
type TorrentFile struct {
	Announce    string
	Hash        [20]byte
	Length      uint64
	Name        string
	PieceLength uint64
	Pieces      [][20]byte
}

// decode decodes a bencode file to a bencode object
// buff represents the 'info' table from the torrent file
// infoMap indicates bytes are to be appendended to buff
func decode(reader *bufio.Reader, buff *bytes.Buffer, infoMap bool) (*bencode, error) {
	ben := &bencode{}
	char, err := reader.ReadByte()
	if err != nil {
		return nil, err
	}
	if infoMap {
		buff.WriteByte(char)
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
				if infoMap {
					buff.WriteByte(ch)
				}
				ben.Dict = dict
				return ben, nil
			}
			reader.UnreadByte()
			val, err := decode(reader, buff, infoMap)
			if err != nil {
				return nil, err
			}
			key := val.String
			if key == "" {
				return nil, errors.New("Dictionary has a non string key")
			}

			// We want to hash the info struct
			if key == "info" {
				infoMap = true
			}

			if val, err = decode(reader, buff, infoMap); err != nil {
				return nil, err
			}

			if key == "info" {
				infoMap = false
				ben.Hash = sha1.Sum(buff.Bytes())
				buff.Reset()
			}

			dict[key] = *val
		}
	case 'i':
		intStr, err := reader.ReadString('e')
		if err != nil {
			return nil, err
		}
		if infoMap {
			buff.WriteString(intStr)
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
				if infoMap {
					buff.WriteByte(ch)
				}
				ben.List = list
				return ben, nil
			}
			reader.UnreadByte()
			value, err := decode(reader, buff, infoMap)
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
		if infoMap {
			buff.WriteString(strLen[1:])
		}
		strLen = strLen[:len(strLen)-1]
		length, err := strconv.ParseUint(strLen, 10, 64)
		if err != nil {
			return nil, err
		}
		buf := make([]byte, length)
		n, err := io.ReadFull(reader, buf)
		if err != nil {
			return nil, err
		}
		if n != int(length) {
			return nil, errors.New("String of the wrong length")
		}
		if infoMap {
			buff.Write(buf)
		}
		ben.String = string(buf)
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
	bencode, err := decode(bufio.NewReader(file), new(bytes.Buffer), false)
	if err != nil {
		return nil, err
	}
	return prettyBencode(bencode)
}
