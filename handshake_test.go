package main

import (
	"bytes"
	"testing"
)

func TestHandshake(t *testing.T) {
	metadata := [20]byte{'m', 'e', 't', 'a', 'd', 'a', 't', 'a', ' ', 'f', 'o', 'r', ' ', 't', 'o', 'r', 'r', 'e', 'n', 't'}
	id := [20]byte{'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h', 'i', 'j', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9'}
	handshake := Handshake(metadata, id)
	expected := append(
		append(
			[]byte{'\x13',
				'B', 'i', 't', 'T', 'o', 'r', 'r', 'e', 'n', 't', ' ', 'p', 'r', 'o', 't', 'o', 'c', 'o', 'l',
				'\x00', '\x00', '\x00', '\x00', '\x00', '\x00', '\x00', '\x00'},
			metadata[:]...),
		id[:]...)
	if !bytes.Equal(handshake, expected) {
		t.Errorf("Expected handshake\n%v but got\n%v instead", expected, handshake)
	}
}
