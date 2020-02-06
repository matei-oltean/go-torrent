package utils

import (
	"testing"
)

func TestGet(t *testing.T) {
	bitfield := Bitfield{0b11001100, 0b10101010}
	expected := []bool{true, true, false, false, true, true, false, false, true, false, true, false, true, false, true, false}
	for index, res := range expected {
		result := bitfield.Get(uint(index))
		if res != result {
			t.Errorf("Expected %t at index %d, got %t instead", res, index, result)
		}
	}
}

func TestTrue(t *testing.T) {
	bitfield := Bitfield{0b00000000, 00000000}
	var index uint = 0
	for ; int(index) < len(bitfield)*8; index++ {
		if bitfield.Get(uint(index)) {
			t.Errorf("Value at index %d is true", index)
		}
		bitfield.True(index)
		if !bitfield.Get(index) {
			t.Errorf("Did not manage so set value at index %d to true", index)
		}
	}
}

func TestSet(t *testing.T) {
	bitfield := Bitfield{0b01100011, 0b11110001}
	expected := []bool{true, true, false, false, true, true, false, false, true, false, true, false, true, false, true, false}
	for index, res := range expected {
		bitfield.Set(uint(index), res)
		if res != bitfield.Get(uint(index)) {
			t.Errorf("Could not set index %d to %t", index, res)
		}
	}
}
