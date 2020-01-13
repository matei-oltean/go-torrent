package fileutils

import (
	"bufio"
	"bytes"
	"crypto/sha1"
	"errors"
	"io"
	"strconv"
)

// Bencode represents a generic bencoded file
type Bencode struct {
	Dict   map[string]Bencode
	List   []Bencode
	String string
	Int    int64
	Hash   [20]byte
}

// Decode decodes a bencode file to a bencode object
// buff represents the 'info' table from the torrent file
// infoMap indicates bytes are to be appendended to buff
func Decode(reader *bufio.Reader, buff *bytes.Buffer, infoMap bool) (*Bencode, error) {
	ben := &Bencode{}
	char, err := reader.ReadByte()
	if err != nil {
		return nil, err
	}
	if infoMap {
		buff.WriteByte(char)
	}
	switch char {
	case 'd':
		dict := make(map[string]Bencode)
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
			val, err := Decode(reader, buff, infoMap)
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

			if val, err = Decode(reader, buff, infoMap); err != nil {
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
		list := make([]Bencode, 5)
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
			value, err := Decode(reader, buff, infoMap)
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
