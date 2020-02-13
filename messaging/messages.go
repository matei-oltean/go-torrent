package messaging

import (
	"encoding/binary"
	"fmt"
	"io"
)

// MessageType represent the different types of peer messages
type MessageType uint8

// Message types
const (
	MChoke MessageType = iota
	MUnchoke
	MInterested
	MNotInterested
	MHave
	MBitfield
	MRequest
	MPiece
	MCancel
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
	var msg *Message = nil
	var err error = nil
	for msg == nil && err == nil {
		msg, err = readMessage(reader)
	}
	if err != nil {
		return nil, err
	}
	return msg, nil
}

// ReadBitfield reads a Message and returns its bitfield
// If the Message is not a bitfield Message, returns an error
func ReadBitfield(reader io.Reader) ([]byte, error) {
	msg, err := Read(reader)
	if err != nil {
		return nil, err
	}
	if msg.Type != MBitfield {
		return nil, fmt.Errorf("expected a bitfield got a message of type %d instead", msg.Type)
	}
	return msg.Payload, nil
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
		Type:    MUnchoke,
		Payload: []byte{},
	}
	return msg.serialise()
}

// Interested returns a serialised unchoke Message
func Interested() []byte {
	msg := &Message{
		Type:    MInterested,
		Payload: []byte{},
	}
	return msg.serialise()
}

// Have returns a have message for a chunk
func Have(index int) []byte {
	payload := make([]byte, 4)
	binary.BigEndian.PutUint32(payload, uint32(index))
	msg := &Message{
		Type:    MHave,
		Payload: payload,
	}
	return msg.serialise()
}

// Request returns a request message for a chunk
func Request(index, begin, length int) []byte {
	payload := make([]byte, 3*4)
	binary.BigEndian.PutUint32(payload, uint32(index))
	binary.BigEndian.PutUint32(payload[4:], uint32(begin))
	binary.BigEndian.PutUint32(payload[8:], uint32(length))
	msg := &Message{
		Type:    MRequest,
		Payload: payload,
	}
	return msg.serialise()
}
