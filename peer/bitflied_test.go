package peer

import (
	"testing"
)

func TestGet(t *testing.T) {
	bitfield := bitfield{0b11001100, 0b10101010}
	expected := []bool{true, true, false, false, true, true, false, false, true, false, true, false, true, false, true, false}
	for index, res := range expected {
		result := bitfield.get(index)
		if res != result {
			t.Errorf("Expected %t at index %d, got %t instead", res, index, result)
		}
	}
}

func TestSet(t *testing.T) {
	bitfield := bitfield{0b00000000, 00000000}
	for index := 0; index < len(bitfield)*8; index++ {
		if bitfield.get(index) {
			t.Errorf("Value at index %d is true", index)
		}
		bitfield.set(index)
		if !bitfield.get(index) {
			t.Errorf("Did not manage so set value at index %d to true", index)
		}
	}
}
