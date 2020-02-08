package peer

import (
	"fmt"
	"net"
	"reflect"
	"time"

	"github.com/matei-oltean/go-torrent/messaging"
	"github.com/matei-oltean/go-torrent/utils"
)

// Peer represents a connection to a peer
type Peer struct {
	Conn     net.Conn
	ID       [20]byte
	Bitfield utils.Bitfield
}

// New creates a new peer from a metadata hash, client id and peer address
func New(metadataHash, id [20]byte, address string) (*Peer, error) {
	conn, err := net.DialTimeout("tcp", address, 3*time.Second)
	if err != nil {
		return nil, err
	}
	// Performing the handshake
	handshake := messaging.GenerateHandshake(metadataHash, id)
	_, err = conn.Write(handshake)
	if err != nil {
		return nil, err
	}
	// We should get a handshake back
	received := make([]byte, messaging.HandshakeSize)
	n, err := conn.Read(received)
	if err != nil {
		conn.Close()
		return nil, err
	}
	if n != messaging.HandshakeSize {
		conn.Close()
		return nil, fmt.Errorf("received handshake with length %d instead of %d", n, messaging.HandshakeSize)
	}

	// It should have the same protocol
	startLen := 1 + len(messaging.Protocol)
	if !reflect.DeepEqual(received[:startLen], handshake[:startLen]) {
		conn.Close()
		return nil, fmt.Errorf("received handshake with the wrong protocol: %v", received[:startLen])
	}
	// And same metadataHash
	if !reflect.DeepEqual(received[startLen+8:startLen+28], handshake[startLen+8:startLen+28]) {
		conn.Close()
		return nil, fmt.Errorf("expected handshake with metadata\n%v got\n%v instead", handshake[startLen+8:startLen+28], received[startLen+8:startLen+28])
	}

	peerID := [20]byte{}
	copy(peerID[:], received[len(received)-20:])

	// next we should receive a bitfield message
	bitfield, err := messaging.ReadBitfield(conn)
	if err != nil {
		conn.Close()
		return nil, err
	}

	return &Peer{
		Conn:     conn,
		ID:       peerID,
		Bitfield: bitfield,
	}, nil
}
