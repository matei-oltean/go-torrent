package main

import (
	"bufio"
	"bytes"
	"testing"
)

func TestEncodeString(t *testing.T) {
	ben := &bencode{Str: "spam"}
	result := Encode(ben)
	expected := []byte("4:spam")
	if !bytes.Equal(result, expected) {
		t.Errorf("Expected %s, got %s", expected, result)
	}
}

func TestEncodeInt(t *testing.T) {
	ben := &bencode{Int: 42}
	result := Encode(ben)
	expected := []byte("i42e")
	if !bytes.Equal(result, expected) {
		t.Errorf("Expected %s, got %s", expected, result)
	}
}

func TestEncodeIntZero(t *testing.T) {
	ben := &bencode{Int: 0}
	result := Encode(ben)
	expected := []byte("i0e")
	if !bytes.Equal(result, expected) {
		t.Errorf("Expected %s, got %s", expected, result)
	}
}

func TestEncodeList(t *testing.T) {
	ben := &bencode{
		List: []bencode{
			{Str: "spam"},
			{Str: "eggs"},
		},
	}
	result := Encode(ben)
	expected := []byte("l4:spam4:eggse")
	if !bytes.Equal(result, expected) {
		t.Errorf("Expected %s, got %s", expected, result)
	}
}

func TestEncodeDict(t *testing.T) {
	ben := &bencode{
		Dict: map[string]bencode{
			"cow":  {Str: "moo"},
			"spam": {Str: "eggs"},
		},
	}
	result := Encode(ben)
	// Keys should be sorted: cow comes before spam
	expected := []byte("d3:cow3:moo4:spam4:eggse")
	if !bytes.Equal(result, expected) {
		t.Errorf("Expected %s, got %s", expected, result)
	}
}

func TestEncodeDictSorted(t *testing.T) {
	// Test that keys are sorted lexicographically
	ben := &bencode{
		Dict: map[string]bencode{
			"z": {Str: "last"},
			"a": {Str: "first"},
			"m": {Str: "middle"},
		},
	}
	result := Encode(ben)
	expected := []byte("d1:a5:first1:m6:middle1:z4:laste")
	if !bytes.Equal(result, expected) {
		t.Errorf("Expected %s, got %s", expected, result)
	}
}

func TestEncodeNested(t *testing.T) {
	ben := &bencode{
		Dict: map[string]bencode{
			"list": {
				List: []bencode{
					{Int: 1},
					{Int: 2},
					{Int: 3},
				},
			},
			"str": {Str: "hello"},
		},
	}
	result := Encode(ben)
	expected := []byte("d4:listli1ei2ei3ee3:str5:helloe")
	if !bytes.Equal(result, expected) {
		t.Errorf("Expected %s, got %s", expected, result)
	}
}

func TestEncodeDecodeRoundTrip(t *testing.T) {
	// Create a complex structure
	original := &bencode{
		Dict: map[string]bencode{
			"t": {Str: "aa"},
			"y": {Str: "q"},
			"q": {Str: "ping"},
			"a": {Dict: map[string]bencode{"id": {Str: "abcdefghij0123456789"}}},
		},
	}

	// Encode it
	encoded := Encode(original)

	// Decode it back
	decoded, err := decode(bufio.NewReader(bytes.NewReader(encoded)), new(bytes.Buffer), false)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	// Re-encode and compare
	reencoded := Encode(decoded)
	if !bytes.Equal(encoded, reencoded) {
		t.Errorf("Round-trip failed:\nOriginal: %s\nRe-encoded: %s", encoded, reencoded)
	}
}

func TestEncodeKRPCPing(t *testing.T) {
	// Example KRPC ping query from BEP 5
	ping := &bencode{
		Dict: map[string]bencode{
			"t": {Str: "aa"},
			"y": {Str: "q"},
			"q": {Str: "ping"},
			"a": {
				Dict: map[string]bencode{
					"id": {Str: "abcdefghij0123456789"},
				},
			},
		},
	}
	result := Encode(ping)
	// Should be a valid bencoded dictionary
	if result[0] != 'd' || result[len(result)-1] != 'e' {
		t.Error("Result should be a dictionary")
	}
	// Should contain the transaction ID
	if !bytes.Contains(result, []byte("1:t2:aa")) {
		t.Error("Should contain transaction ID")
	}
}
