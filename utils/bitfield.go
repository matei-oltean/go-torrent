package utils

// Bitfield represents a bitfield
type Bitfield []byte

// Get returns the value of the bitfield at a certain index
func (bf Bitfield) Get(index uint) bool {
	bucket := int(index / 8)
	if bucket >= len(bf) {
		return false
	}
	return bf[bucket]>>(7-index%8)&1 != 0
}

// True sets bitfield value at a certain index to true
func (bf Bitfield) True(index uint) {
	bucket := int(index / 8)
	if int(bucket) >= len(bf) {
		return
	}
	bf[bucket] |= 1 << (7 - index%8)
}

// Set sets bitfield value at a certain index
func (bf Bitfield) Set(index uint, value bool) {
	if value {
		bf.True(index)
		return
	}
	bucket := int(index / 8)
	if bucket >= len(bf) {
		return
	}
	// clear the offset's bit
	bf[bucket] &^= 1 << (7 - index%8)
}
