package utils

import "crypto/rand"

// ClientID returns the id -GT0000- followed by 12 random bytes
func ClientID() [20]byte {
	id := [20]byte{'-', 'G', 'T', '0', '1', '0', '0', '-'}
	rand.Read(id[8:])
	return id
}
