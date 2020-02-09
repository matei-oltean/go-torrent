package utils

// Bitfield represents a bitfield
type Bitfield []byte

// Get returns the value of the bitfield at a certain index
func (bf Bitfield) Get(index int) bool {
	bucket := index / 8
	if bucket >= len(bf) {
		return false
	}
	return bf[bucket]>>(7-index%8)&1 != 0
}

// True sets bitfield value at a certain index to true
func (bf Bitfield) True(index int) {
	bucket := index / 8
	if bucket >= len(bf) {
		return
	}
	bf[bucket] |= 1 << (7 - index%8)
}

// Set sets bitfield value at a certain index
func (bf Bitfield) Set(index int, value bool) {
	if value {
		bf.True(index)
		return
	}
	bucket := index / 8
	if bucket >= len(bf) {
		return
	}
	// clear the offset's bit
	bf[bucket] &^= 1 << (7 - index%8)
}
