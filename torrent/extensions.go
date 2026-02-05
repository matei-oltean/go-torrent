package torrent

import (
	"bufio"
	"bytes"
	"errors"
	"io"
)

// Extension messages
const (
	eRequest uint8 = iota
	eData
	eReject
)

// ParseExtensionsHandshake parses an extension handshake, returning its m map and metadata size
func ParseExtensionsHandshake(payload []byte) (map[string]uint8, int, error) {
	ben, err := decode(bufio.NewReader(bytes.NewReader(payload)), new(bytes.Buffer), false)
	if err != nil {
		return nil, 0, err
	}
	if ben.Dict == nil {
		return nil, 0, errors.New("extension message has no dictionary")
	}
	dict := ben.Dict
	m, ok := dict["m"]
	if !ok || m.Dict == nil {
		return nil, 0, errors.New("extension message has no \"m\" key")
	}

	metaSize, ok := dict["metadata_size"]
	if !ok || metaSize.Int == 0 {
		return nil, 0, errors.New("extension message has no \"metadata_size\" key")
	}

	ext := make(map[string]uint8)
	for key, val := range m.Dict {
		ext[key] = uint8(val.Int)
	}
	return ext, metaSize.Int, nil
}

// ParseExtensionsMetadata parses an extension metadata message
// returns its payload and the piece index
// a nil payload means it was a reject message
func ParseExtensionsMetadata(payload []byte) ([]byte, int, error) {
	br := bufio.NewReader(bytes.NewReader(payload))
	ben, err := decode(br, new(bytes.Buffer), false)
	if err != nil {
		return nil, 0, err
	}
	if ben.Dict == nil {
		return nil, 0, errors.New("nil dictionary in payload")
	}
	msgType, ok := ben.Dict["msg_type"]
	if !ok {
		return nil, 0, errors.New("payload missing \"msg_type\" entry")
	}
	if uint8(msgType.Int) == eReject {
		return nil, 0, nil
	}
	index, ok := ben.Dict["piece"]
	if !ok {
		return nil, 0, errors.New("payload missing \"piece\" entry")
	}
	p, err := io.ReadAll(br)
	return p, index.Int, err
}
