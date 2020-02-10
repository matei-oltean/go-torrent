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

// Set sets bitfield value at a certain index to true
func (bf Bitfield) Set(index int) {
	bucket := index / 8
	if bucket >= len(bf) {
		return
	}
	bf[bucket] |= 1 << (7 - index%8)
}
