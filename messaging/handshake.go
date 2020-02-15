package messaging

// Protocol is the used protocol in our communications
const Protocol string = "BitTorrent protocol"

// HandshakeSize is the size of a handshake message
// length of protocol + protocol + extensions + metadata + id
const HandshakeSize int = 1 + len(Protocol) + 8 + 20 + 20

// Handshake returns the handshake message
func Handshake(metadataHash, id [20]byte) []byte {
	protocolLen := len(Protocol)
	res := make([]byte, HandshakeSize)
	// format is:
	// length of the protocol
	res[0] = byte(protocolLen)
	// protocol
	copy(res[1:], Protocol)
	// 8 bytes for implemented extensions; will be left blank
	// 20 bytes for the the hash of the metadata of the torrent
	copy(res[1+protocolLen+8:], metadataHash[:])
	// 20 bytes for the client id
	copy(res[1+protocolLen+8+20:], id[:])
	return res
}
