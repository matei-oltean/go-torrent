package torrent

import (
	"bufio"
	"bytes"
	"crypto/sha1"
	"errors"
	"fmt"
	"io"
	"maps"
	"slices"
	"strconv"
)

// bencode represents a generic bencoded file
type bencode struct {
	Dict map[string]bencode
	List []bencode
	Str  string
	Int  int
	Hash [20]byte
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
			key := val.Str
			if key == "" {
				return nil, errors.New("dictionary has a non string key")
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
		ben.Int = int(integer)
		return ben, nil
	case 'l':
		var list []bencode
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
			return nil, errors.New("string of the wrong length")
		}
		if infoMap {
			buff.Write(buf)
		}
		ben.Str = string(buf)
		return ben, nil
	}
}

// Encode encodes a bencode object to a byte slice
func Encode(ben *bencode) []byte {
	var buf bytes.Buffer
	encodeTo(&buf, ben)
	return buf.Bytes()
}

// encodeTo writes the bencoded representation to a buffer
func encodeTo(buf *bytes.Buffer, ben *bencode) {
	switch {
	case ben.Dict != nil:
		buf.WriteByte('d')
		// Keys must be sorted in lexicographical order
		keys := slices.Sorted(maps.Keys(ben.Dict))
		for _, k := range keys {
			v := ben.Dict[k]
			// Write key as string
			buf.WriteString(strconv.Itoa(len(k)))
			buf.WriteByte(':')
			buf.WriteString(k)
			// Write value
			encodeTo(buf, &v)
		}
		buf.WriteByte('e')
	case ben.List != nil:
		buf.WriteByte('l')
		for i := range ben.List {
			encodeTo(buf, &ben.List[i])
		}
		buf.WriteByte('e')
	case ben.Str != "":
		buf.WriteString(strconv.Itoa(len(ben.Str)))
		buf.WriteByte(':')
		buf.WriteString(ben.Str)
	case ben.Int != 0:
		buf.WriteByte('i')
		buf.WriteString(strconv.Itoa(ben.Int))
		buf.WriteByte('e')
	default:
		// Zero int or empty - encode as int 0
		buf.WriteString("i0e")
	}
}

func (ben bencode) String() string {
	if ben.Str != "" {
		return ben.Str
	}
	if ben.Int != 0 {
		return strconv.Itoa(int(ben.Int))
	}
	if ben.List != nil {
		return fmt.Sprintf("%+v", ben.List)
	}
	if ben.Dict != nil {
		return fmt.Sprintf("%+v", ben.Dict)
	}
	return "nil"
}
