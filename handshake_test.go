package main

import (
	"bytes"
	"testing"
)

func TestHandshake(t *testing.T) {
	metadata := [20]byte{'m', 'e', 't', 'a', 'd', 'a', 't', 'a', ' ', 'f', 'o', 'r', ' ', 't', 'o', 'r', 'r', 'e', 'n', 't'}
	id := [20]byte{'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h', 'i', 'j', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9'}
	handshake := Handshake(metadata, id)
	// Reserved bytes: [5]=0x10 (extensions), [7]=0x01 (DHT)
	expected := append(
		append(
			[]byte{'\x13',
				'B', 'i', 't', 'T', 'o', 'r', 'r', 'e', 'n', 't', ' ', 'p', 'r', 'o', 't', 'o', 'c', 'o', 'l',
				'\x00', '\x00', '\x00', '\x00', '\x00', '\x10', '\x00', '\x01'},
			metadata[:]...),
		id[:]...)
	if !bytes.Equal(handshake, expected) {
		t.Errorf("Expected handshake\n%v but got\n%v instead", expected, handshake)
	}
}

func TestParseHandshakeExtensions(t *testing.T) {
	metadata := [20]byte{}
	id := [20]byte{}
	handshake := Handshake(metadata, id)

	supportsDHT, supportsExtended := ParseHandshakeExtensions(handshake)
	if !supportsDHT {
		t.Error("Expected DHT support")
	}
	if !supportsExtended {
		t.Error("Expected extension support")
	}

	// Test handshake without DHT
	handshake[1+len(Protocol)+7] = 0x00
	supportsDHT, supportsExtended = ParseHandshakeExtensions(handshake)
	if supportsDHT {
		t.Error("Should not have DHT support")
	}
	if !supportsExtended {
		t.Error("Should still have extension support")
	}
}
