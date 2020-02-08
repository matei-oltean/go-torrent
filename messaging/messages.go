package messaging

import (
	"encoding/binary"
	"fmt"
	"io"

	"github.com/matei-oltean/go-torrent/utils"
)

// messageType represent the different types of peer messages
type messageType uint8

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

// readMessage is a wrapper around read
// it reads and parses messages from the connection
// until an non keepalive message is received
// which is then returned
func readMessage(reader io.Reader) (*message, error) {
	var message *message = nil
	var err error = nil
	for message == nil && err == nil {
		message, err = read(reader)
	}
	if err != nil {
		return nil, err
	}
	return message, nil
}

// ReadBitfield reads a message and returns its bitfield
// If the message is not a bitfield message, returns an error
func ReadBitfield(reader io.Reader) (utils.Bitfield, error) {
	message, err := readMessage(reader)
	if err != nil {
		return nil, err
	}
	if message.Type != bitfield {
		return nil, fmt.Errorf("expected a bitfield got a message of type %d instead", message.Type)
	}
	return message.Payload, nil
}

// serialise returns the byte array representing a message to be sent
func (msg *message) serialise() []byte {
	// +1 to account for the message id
	payLen := uint32(len(msg.Payload) + 1)
	serialised := make([]byte, 4+payLen)
	binary.BigEndian.PutUint32(serialised, payLen)
	serialised[4] = byte(msg.Type)
	copy(serialised[5:], msg.Payload)
	return serialised
}

// Unchoke returns a serialised unchoke message
func Unchoke() []byte {
	msg := &message{
		Type:    unchoke,
		Payload: []byte{},
	}
	return msg.serialise()
}

// Interested returns a serialised unchoke message
func Interested() []byte {
	msg := &message{
		Type:    interested,
		Payload: []byte{},
	}
	return msg.serialise()
}
