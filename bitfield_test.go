package main

import (
	crand "crypto/rand"
	"math/rand"
	"testing"
)

const ntests int = 1000

func TestGet(t *testing.T) {
	bitfield := bitfield{0b11001100, 0b10101010}
	expected := []bool{true, true, false, false, true, true, false, false, true, false, true, false, true, false, true, false}
	for index, exp := range expected {
		assertGet(t, exp, bitfield, index)
	}
}

func TestGetRandomised(t *testing.T) {
	for range ntests {
		bf := generateBitfield(t)
		var expected []bool

		for _, byte := range bf {
			for j := 7; j >= 0; j-- {
				bit := (byte & (1 << j)) != 0
				expected = append(expected, bit)
			}
		}
		assertBitfield(t, bf, expected)
	}
}

func TestSet(t *testing.T) {
	bitfield := bitfield{0b00000000, 00000000}
	for index := range len(bitfield) * 8 {
		assertGet(t, false, bitfield, index)
		bitfield.set(index)
		assertGet(t, true, bitfield, index)
	}
}

func TestSetRandomised(t *testing.T) {
	for range ntests {
		bf := generateBitfield(t)
		bfn := len(bf) * 8
		idx := rand.Intn(bfn)

		expected := make([]bool, bfn)
		for i := range expected {
			expected[i] = bf.get(i)
		}

		if !bf.get(idx) {
			bf.set(idx)
		} else {
			bf.unset(idx)
		}

		expected[idx] = !expected[idx]
		assertBitfield(t, bf, expected)
	}
}

func TestEmptyBitfield(t *testing.T) {
	var emptyBf []bitfield
	emptyBf = append(emptyBf, nil) // nil case
	var bf bitfield
	emptyBf = append(emptyBf, bf) // zero value case

	for _, bf := range emptyBf {
		for i := -2; i < 3; i++ {
			assertGet(t, false, bf, i)
			bf.set(i)
			bf.unset(i)
		}
	}
}

func assertGet(t *testing.T, expected bool, bitfield bitfield, index int) {
	result := bitfield.get(index)
	if expected != result {
		t.Errorf("Expected %t at index %d, got %t instead", expected, index, result)
	}
}

func generateBitfield(t *testing.T) bitfield {
	bytes := make([]byte, 5)
	if _, err := crand.Read(bytes); err != nil {
		t.Fatal("rand", err)
	}
	return bytes
}

func assertBitfield(t *testing.T, bf bitfield, expected []bool) {
	if len(expected) != len(bf)*8 {
		t.Fatal("assertBitfield: invalid arguments")
	}
	for index := -5; index < len(expected)+5; index++ {
		exp := 0 <= index && index < len(expected) && expected[index]
		assertGet(t, exp, bf, index)
	}

}
