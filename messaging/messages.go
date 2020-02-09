package messaging

import (
	"encoding/binary"
	"fmt"
	"io"

	"github.com/matei-oltean/go-torrent/utils"
)

// MessageType represent the different types of peer messages
type MessageType uint8

const (
	choke MessageType = iota
	unchoke
	interested
	notInterested
	have
	bitfield
	request
	piece
	cancel
)

// Message represents a Message: its type and payload
type Message struct {
	Type    MessageType
	Payload []byte
}

// readMessage reads and parses a Message coming from a connection
func readMessage(reader io.Reader) (*Message, error) {
	// A Message is composed as follows:
	// Message length (4 bytes big endian)
	// Message type (1 byte)
	// Message payload if any

	binLength := make([]byte, 4)
	_, err := io.ReadFull(reader, binLength)
	if err != nil {
		return nil, err
	}
	msgLen := binary.BigEndian.Uint32(binLength)
	// a msgLen of zero means it is a keepalive Message
	if msgLen == 0 {
		return nil, nil
	}

	msgBuff := make([]byte, msgLen)
	_, err = io.ReadFull(reader, msgBuff)
	if err != nil {
		return nil, err
	}
	return &Message{
		Type:    MessageType(msgBuff[0]),
		Payload: msgBuff[1:],
	}, nil
}

// Read is a wrapper around readMessage
// it reads and parses messages from the connection
// until an non keepalive Message is received
// which is then returned
func Read(reader io.Reader) (*Message, error) {
	var Message *Message = nil
	var err error = nil
	for Message == nil && err == nil {
		Message, err = readMessage(reader)
	}
	if err != nil {
		return nil, err
	}
	return Message, nil
}

// ReadBitfield reads a Message and returns its bitfield
// If the Message is not a bitfield Message, returns an error
func ReadBitfield(reader io.Reader) (utils.Bitfield, error) {
	Message, err := Read(reader)
	if err != nil {
		return nil, err
	}
	if Message.Type != bitfield {
		return nil, fmt.Errorf("expected a bitfield got a Message of type %d instead", Message.Type)
	}
	return Message.Payload, nil
}

// serialise returns the byte array representing a Message to be sent
func (msg *Message) serialise() []byte {
	// +1 to account for the Message id
	payLen := uint32(len(msg.Payload) + 1)
	serialised := make([]byte, 4+payLen)
	binary.BigEndian.PutUint32(serialised, payLen)
	serialised[4] = byte(msg.Type)
	copy(serialised[5:], msg.Payload)
	return serialised
}

// Unchoke returns a serialised unchoke Message
func Unchoke() []byte {
	msg := &Message{
		Type:    unchoke,
		Payload: []byte{},
	}
	return msg.serialise()
}

// Interested returns a serialised unchoke Message
func Interested() []byte {
	msg := &Message{
		Type:    interested,
		Payload: []byte{},
	}
	return msg.serialise()
}
