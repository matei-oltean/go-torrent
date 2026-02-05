package main

import (
	"bytes"
	crand "crypto/rand"
	"encoding/binary"
	"fmt"
	"io"
	"math/rand"
	"testing"
	"testing/iotest"
)

func newMock(t *testing.T, tries uint, payloadLength uint32) (io.Reader, Message) {
	var br bytes.Buffer
	header := make([]byte, 4)
	for tries > 1 {
		binary.BigEndian.PutUint32(header, 0)
		br.Write(header)
		tries--
	}

	binary.BigEndian.PutUint32(header, payloadLength+1)
	br.Write(header)
	kind := uint8(0)
	br.WriteByte(kind)

	payload := make([]byte, payloadLength)
	if _, err := crand.Read(payload); err != nil {
		t.Fatal(err)
	}
	br.Write(payload)
	expectedMessage := Message{
		Type:    MessageType(kind),
		Payload: payload,
	}
	return &br, expectedMessage
}

func id(x io.Reader) io.Reader {
	return x
}

func test1(t *testing.T, tries uint, f func(io.Reader) io.Reader) error {
	reader, expected := newMock(t, tries, uint32(rand.Int31n(15)))

	reader = f(reader)
	actual, err := ReadMessage(reader)
	if actual == nil {
		return fmt.Errorf("returned null message")
	}
	if err != nil {
		return fmt.Errorf("err: %s", err)
	}
	if !(expected.Type == actual.Type &&
		bytes.Equal(expected.Payload, actual.Payload)) {
		return fmt.Errorf("expected %v got %v", expected, *actual)
	}
	return nil
}

func TestReadMessage(t *testing.T) {
	for _, mk := range []struct {
		f    func(io.Reader) io.Reader
		name string
	}{
		{id, "id"},
		{iotest.OneByteReader, "iotest.OneByteReader"},
		{iotest.HalfReader, "iotest.HalfReader"},
		{iotest.DataErrReader, "iotest.DataErrReader"},
	} {
		for _, tries := range []uint{1, 4, 8} {
			if err := test1(t, tries, mk.f); err != nil {
				t.Errorf("%v %v: %v\n", mk.name, tries, err)

			}
		}
	}
}

func TestPortMessage(t *testing.T) {
	msg := PortMessage(6881)
	// Should be: length(4) + type(1) + port(2) = 7 bytes
	if len(msg) != 7 {
		t.Errorf("Expected 7 bytes, got %d", len(msg))
	}

	// Parse it back
	length := binary.BigEndian.Uint32(msg[0:4])
	if length != 3 { // type(1) + port(2)
		t.Errorf("Expected length 3, got %d", length)
	}

	if msg[4] != byte(MPort) {
		t.Errorf("Expected message type %d, got %d", MPort, msg[4])
	}

	port, err := ParsePortMessage(msg[5:])
	if err != nil {
		t.Fatalf("ParsePortMessage failed: %v", err)
	}
	if port != 6881 {
		t.Errorf("Expected port 6881, got %d", port)
	}
}

func TestParsePortMessageInvalid(t *testing.T) {
	_, err := ParsePortMessage([]byte{0x1A}) // Only 1 byte
	if err == nil {
		t.Error("Expected error for invalid payload length")
	}
}
