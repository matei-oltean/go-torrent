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
