package peer

// bitfield represents a bitfield
type bitfield []byte

// set returns the value of the bitfield at a certain index
func (bf bitfield) get(index int) bool {
	bucket := index / 8
	if bucket >= len(bf) {
		return false
	}
	return bf[bucket]>>(7-index%8)&1 != 0
}

// set sets bitfield value at a certain index to true
func (bf bitfield) set(index int) {
	bucket := index / 8
	if bucket >= len(bf) {
		return
	}
	bf[bucket] |= 1 << (7 - index%8)
}
