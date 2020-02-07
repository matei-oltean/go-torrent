package messaging

// PROTOCOL is the used protocol in our communications
const PROTOCOL string = "BitTorrent protocol"

// GenerateHandshake generates the handshake message
func GenerateHandshake(metadataHash, id [20]byte) []byte {
	protocolLen := len(PROTOCOL)
	handshakeLen := 1 + protocolLen + 8 + 20 + 20
	res := make([]byte, handshakeLen)
	// format is:
	// length of the protocol
	res[0] = byte(protocolLen)
	// protocol
	copy(res[1:], PROTOCOL)
	// 8 bytes for implemented extensions; will be left blank
	// 20 bytes for the the hash of the metadata of the torrent
	copy(res[1+protocolLen+8:], metadataHash[:])
	// 20 bytes for the client id
	copy(res[1+protocolLen+8+20:], id[:])
	return res
}
