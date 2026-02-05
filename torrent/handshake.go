package torrent

// Protocol is the used protocol in our communications
const Protocol string = "BitTorrent protocol"

// HandshakeSize is the size of a handshake message
// length of protocol + protocol + extensions + metadata + id
const HandshakeSize int = 1 + len(Protocol) + 8 + 20 + 20

// Extension bits
const (
	ExtensionDHT      = 0x01 // reserved[7] bit 0 - BEP 5
	ExtensionExtended = 0x10 // reserved[5] bit 4 - BEP 10
)

// Handshake returns the handshake message
func Handshake(metadataHash, id [20]byte) []byte {
	protocolLen := len(Protocol)
	res := make([]byte, HandshakeSize)
	// format is:
	// length of the protocol
	res[0] = byte(protocolLen)
	// protocol
	copy(res[1:], Protocol)

	// 8 bytes for implemented extensions
	extensions := make([]byte, 8)
	// support extensions (BEP 10)
	extensions[5] = 0x10
	// support DHT (BEP 5)
	extensions[7] = 0x01
	copy(res[1+protocolLen:], extensions)

	// 20 bytes for the the hash of the metadata of the torrent
	copy(res[1+protocolLen+8:], metadataHash[:])
	// 20 bytes for the client id
	copy(res[1+protocolLen+8+20:], id[:])
	return res
}

// ParseHandshakeExtensions extracts extension flags from a handshake response
// The handshake must be at least HandshakeSize bytes
func ParseHandshakeExtensions(handshake []byte) (supportsDHT, supportsExtended bool) {
	if len(handshake) < HandshakeSize {
		return false, false
	}
	protocolLen := int(handshake[0])
	reserved := handshake[1+protocolLen : 1+protocolLen+8]
	supportsDHT = reserved[7]&ExtensionDHT != 0
	supportsExtended = reserved[5]&ExtensionExtended != 0
	return
}
