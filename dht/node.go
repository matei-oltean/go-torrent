// Package dht implements the BitTorrent Distributed Hash Table (BEP 5)
package dht

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"net"
	"time"
)

// NodeID is a 160-bit identifier for a DHT node (same space as info hash)
type NodeID [20]byte

// NodeInfo represents a DHT node with its ID and network address
type NodeInfo struct {
	ID       NodeID
	Addr     *net.UDPAddr
	LastSeen time.Time
}

// GenerateNodeID creates a random 160-bit node ID
func GenerateNodeID() (NodeID, error) {
	var id NodeID
	_, err := rand.Read(id[:])
	return id, err
}

// Distance returns the XOR distance between two node IDs
// XOR distance is the metric used in Kademlia DHT
func Distance(a, b NodeID) NodeID {
	var dist NodeID
	for i := range a {
		dist[i] = a[i] ^ b[i]
	}
	return dist
}

// LeadingZeros returns the number of leading zero bits in the node ID
// Used to determine which k-bucket a node belongs to
func (id NodeID) LeadingZeros() int {
	for i, b := range id {
		if b == 0 {
			continue
		}
		// Count leading zeros in this byte
		for j := 7; j >= 0; j-- {
			if b&(1<<j) != 0 {
				return i*8 + (7 - j)
			}
		}
	}
	return 160 // All zeros
}

// BucketIndex returns the k-bucket index for a node relative to our own ID
// Bucket 0 is for nodes with distance >= 2^159 (most distant)
// Bucket 159 is for nodes with distance < 2 (closest)
func BucketIndex(self, other NodeID) int {
	dist := Distance(self, other)
	lz := dist.LeadingZeros()
	if lz >= 160 {
		return 159 // Same ID (shouldn't happen)
	}
	return lz
}

// CompactIPv4 encodes a node as 26 bytes: 20-byte ID + 4-byte IP + 2-byte port
func (n *NodeInfo) CompactIPv4() ([]byte, error) {
	ip4 := n.Addr.IP.To4()
	if ip4 == nil {
		return nil, fmt.Errorf("not an IPv4 address: %s", n.Addr.IP)
	}
	buf := make([]byte, 26)
	copy(buf[:20], n.ID[:])
	copy(buf[20:24], ip4)
	binary.BigEndian.PutUint16(buf[24:26], uint16(n.Addr.Port))
	return buf, nil
}

// CompactIPv6 encodes a node as 38 bytes: 20-byte ID + 16-byte IP + 2-byte port
func (n *NodeInfo) CompactIPv6() ([]byte, error) {
	ip6 := n.Addr.IP.To16()
	if ip6 == nil {
		return nil, fmt.Errorf("invalid IP address: %s", n.Addr.IP)
	}
	// Check it's actually IPv6, not IPv4-mapped
	if n.Addr.IP.To4() != nil {
		return nil, fmt.Errorf("not an IPv6 address: %s", n.Addr.IP)
	}
	buf := make([]byte, 38)
	copy(buf[:20], n.ID[:])
	copy(buf[20:36], ip6)
	binary.BigEndian.PutUint16(buf[36:38], uint16(n.Addr.Port))
	return buf, nil
}

// ParseCompactIPv4 decodes a 26-byte compact node info
func ParseCompactIPv4(data []byte) (*NodeInfo, error) {
	if len(data) != 26 {
		return nil, fmt.Errorf("compact IPv4 node info must be 26 bytes, got %d", len(data))
	}
	var id NodeID
	copy(id[:], data[:20])
	ip := net.IP(data[20:24])
	port := binary.BigEndian.Uint16(data[24:26])
	return &NodeInfo{
		ID:       id,
		Addr:     &net.UDPAddr{IP: ip, Port: int(port)},
		LastSeen: time.Now(),
	}, nil
}

// ParseCompactIPv6 decodes a 38-byte compact node info
func ParseCompactIPv6(data []byte) (*NodeInfo, error) {
	if len(data) != 38 {
		return nil, fmt.Errorf("compact IPv6 node info must be 38 bytes, got %d", len(data))
	}
	var id NodeID
	copy(id[:], data[:20])
	ip := net.IP(data[20:36])
	port := binary.BigEndian.Uint16(data[36:38])
	return &NodeInfo{
		ID:       id,
		Addr:     &net.UDPAddr{IP: ip, Port: int(port)},
		LastSeen: time.Now(),
	}, nil
}

// ParseCompactNodes parses a concatenated list of compact node infos
func ParseCompactNodes(data []byte, ipv6 bool) ([]*NodeInfo, error) {
	nodeSize := 26
	if ipv6 {
		nodeSize = 38
	}
	if len(data)%nodeSize != 0 {
		return nil, fmt.Errorf("compact nodes data length %d not divisible by %d", len(data), nodeSize)
	}
	nodes := make([]*NodeInfo, len(data)/nodeSize)
	for i := range nodes {
		var err error
		chunk := data[i*nodeSize : (i+1)*nodeSize]
		if ipv6 {
			nodes[i], err = ParseCompactIPv6(chunk)
		} else {
			nodes[i], err = ParseCompactIPv4(chunk)
		}
		if err != nil {
			return nil, err
		}
	}
	return nodes, nil
}

// String returns a human-readable representation of the node
func (n *NodeInfo) String() string {
	return fmt.Sprintf("%x@%s", n.ID[:8], n.Addr)
}
