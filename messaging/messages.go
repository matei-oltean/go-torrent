package messaging

import (
	"encoding/binary"
	"fmt"
	"io"

	"github.com/matei-oltean/go-torrent/utils"
)

// messageType represent the different types of peer messages
type messageType int

const (
	choke messageType = iota
	unchoke
	interested
	notInterested
	have
	bitfield
	request
	piece
	cancel
)

// message represents a message: its type and payload
type message struct {
	Type    messageType
	Payload []byte
}

// read reads and parses a message coming from a connection
func read(reader io.Reader) (*message, error) {
	// A message is composed as follows:
	// message length (4 bytes big endian)
	// message type (1 byte)
	// message payload if any

	binLength := make([]byte, 4)
	_, err := io.ReadFull(reader, binLength)
	if err != nil {
		return nil, err
	}
	msgLen := binary.BigEndian.Uint32(binLength)
	// a msgLen of zero means it is a keepalive message
	if msgLen == 0 {
		return nil, nil
	}

	msgBuff := make([]byte, msgLen)
	_, err = io.ReadFull(reader, msgBuff)
	if err != nil {
		return nil, err
	}
	return &message{
		Type:    messageType(msgBuff[0]),
		Payload: msgBuff[1:],
	}, nil
}

// ReadBitfield reads a message and returns its bitfield
// If the message is not a bitfield message, returns an error
func ReadBitfield(reader io.Reader) (utils.Bitfield, error) {
	message, err := read(reader)
	if err != nil {
		return nil, err
	}
	if message.Type != bitfield {
		return nil, fmt.Errorf("expected a bitfield got a message of type %d instead", message.Type)
	}
	return message.Payload, nil
}
