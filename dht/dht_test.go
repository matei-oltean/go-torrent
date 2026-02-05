package dht

import (
	"bytes"
	"net"
	"testing"
)

func TestGenerateNodeID(t *testing.T) {
	id1, err := GenerateNodeID()
	if err != nil {
		t.Fatalf("GenerateNodeID failed: %v", err)
	}
	id2, err := GenerateNodeID()
	if err != nil {
		t.Fatalf("GenerateNodeID failed: %v", err)
	}
	if id1 == id2 {
		t.Error("Generated IDs should be different")
	}
}

func TestDistance(t *testing.T) {
	var a, b NodeID
	a[0] = 0xFF
	b[0] = 0x0F

	dist := Distance(a, b)
	if dist[0] != 0xF0 {
		t.Errorf("Expected 0xF0, got 0x%02X", dist[0])
	}

	// Distance to self should be zero
	distSelf := Distance(a, a)
	var zero NodeID
	if distSelf != zero {
		t.Error("Distance to self should be zero")
	}
}

func TestLeadingZeros(t *testing.T) {
	tests := []struct {
		id       NodeID
		expected int
	}{
		{NodeID{0xFF}, 0},
		{NodeID{0x7F}, 1},
		{NodeID{0x01}, 7},
		{NodeID{0x00, 0xFF}, 8},
		{NodeID{0x00, 0x01}, 15},
		{NodeID{}, 160}, // All zeros
	}

	for _, tc := range tests {
		result := tc.id.LeadingZeros()
		if result != tc.expected {
			t.Errorf("LeadingZeros(%v) = %d, expected %d", tc.id[:4], result, tc.expected)
		}
	}
}

func TestBucketIndex(t *testing.T) {
	var self NodeID
	self[0] = 0x80 // 10000000

	// Node with distance 0x40 (01000000) -> 1 leading zero -> bucket 1
	var other1 NodeID
	other1[0] = 0xC0 // XOR = 0x40
	if idx := BucketIndex(self, other1); idx != 1 {
		t.Errorf("Expected bucket 1, got %d", idx)
	}

	// Node with very different ID -> bucket 0
	var other2 NodeID
	other2[0] = 0x00 // XOR = 0x80 -> 0 leading zeros
	if idx := BucketIndex(self, other2); idx != 0 {
		t.Errorf("Expected bucket 0, got %d", idx)
	}
}

func TestCompactIPv4(t *testing.T) {
	node := &NodeInfo{
		ID:   NodeID{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20},
		Addr: &net.UDPAddr{IP: net.IPv4(192, 168, 1, 1), Port: 6881},
	}

	compact, err := node.CompactIPv4()
	if err != nil {
		t.Fatalf("CompactIPv4 failed: %v", err)
	}
	if len(compact) != 26 {
		t.Fatalf("Expected 26 bytes, got %d", len(compact))
	}

	// Parse it back
	parsed, err := ParseCompactIPv4(compact)
	if err != nil {
		t.Fatalf("ParseCompactIPv4 failed: %v", err)
	}
	if parsed.ID != node.ID {
		t.Error("ID mismatch")
	}
	if !parsed.Addr.IP.Equal(node.Addr.IP) {
		t.Errorf("IP mismatch: %v != %v", parsed.Addr.IP, node.Addr.IP)
	}
	if parsed.Addr.Port != node.Addr.Port {
		t.Errorf("Port mismatch: %d != %d", parsed.Addr.Port, node.Addr.Port)
	}
}

func TestCompactIPv6(t *testing.T) {
	node := &NodeInfo{
		ID:   NodeID{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20},
		Addr: &net.UDPAddr{IP: net.ParseIP("2001:db8::1"), Port: 6881},
	}

	compact, err := node.CompactIPv6()
	if err != nil {
		t.Fatalf("CompactIPv6 failed: %v", err)
	}
	if len(compact) != 38 {
		t.Fatalf("Expected 38 bytes, got %d", len(compact))
	}

	// Parse it back
	parsed, err := ParseCompactIPv6(compact)
	if err != nil {
		t.Fatalf("ParseCompactIPv6 failed: %v", err)
	}
	if parsed.ID != node.ID {
		t.Error("ID mismatch")
	}
	if !parsed.Addr.IP.Equal(node.Addr.IP) {
		t.Errorf("IP mismatch: %v != %v", parsed.Addr.IP, node.Addr.IP)
	}
	if parsed.Addr.Port != node.Addr.Port {
		t.Errorf("Port mismatch: %d != %d", parsed.Addr.Port, node.Addr.Port)
	}
}

func TestParseCompactNodes(t *testing.T) {
	// Create 3 nodes
	nodes := make([]*NodeInfo, 3)
	for i := range nodes {
		var id NodeID
		id[0] = byte(i + 1)
		nodes[i] = &NodeInfo{
			ID:   id,
			Addr: &net.UDPAddr{IP: net.IPv4(192, 168, 1, byte(i+1)), Port: 6881 + i},
		}
	}

	// Encode all
	var data []byte
	for _, n := range nodes {
		compact, _ := n.CompactIPv4()
		data = append(data, compact...)
	}

	// Parse back
	parsed, err := ParseCompactNodes(data, false)
	if err != nil {
		t.Fatalf("ParseCompactNodes failed: %v", err)
	}
	if len(parsed) != 3 {
		t.Fatalf("Expected 3 nodes, got %d", len(parsed))
	}

	for i, p := range parsed {
		if p.ID != nodes[i].ID {
			t.Errorf("Node %d: ID mismatch", i)
		}
	}
}

func TestRoutingTableAddRemove(t *testing.T) {
	self, _ := GenerateNodeID()
	rt := NewRoutingTable(self)

	// Create a node
	var nodeID NodeID
	nodeID[0] = self[0] ^ 0x80 // Ensure it's in a different bucket
	node := &NodeInfo{
		ID:   nodeID,
		Addr: &net.UDPAddr{IP: net.IPv4(192, 168, 1, 1), Port: 6881},
	}

	// Add it
	if !rt.AddNode(node) {
		t.Error("Failed to add node")
	}
	if rt.Size() != 1 {
		t.Errorf("Expected size 1, got %d", rt.Size())
	}

	// Find it
	found := rt.FindNode(nodeID)
	if found == nil {
		t.Error("Failed to find node")
	}

	// Remove it
	rt.RemoveNode(nodeID)
	if rt.Size() != 0 {
		t.Errorf("Expected size 0, got %d", rt.Size())
	}
}

func TestRoutingTableClosestNodes(t *testing.T) {
	self, _ := GenerateNodeID()
	rt := NewRoutingTable(self)

	// Add 20 nodes with varying distances
	for i := range 20 {
		var nodeID NodeID
		nodeID[0] = byte(i)
		nodeID[19] = byte(i) // Make them all different
		node := &NodeInfo{
			ID:   nodeID,
			Addr: &net.UDPAddr{IP: net.IPv4(192, 168, 1, byte(i+1)), Port: 6881},
		}
		rt.AddNode(node)
	}

	// Find 8 closest to a target
	var target NodeID
	target[0] = 5
	closest := rt.ClosestNodes(target, 8)

	if len(closest) != 8 {
		t.Fatalf("Expected 8 nodes, got %d", len(closest))
	}

	// Verify they're sorted by distance
	for i := 1; i < len(closest); i++ {
		if compareDistance(closest[i].ID, closest[i-1].ID, target) < 0 {
			t.Error("Nodes not sorted by distance")
		}
	}
}

func TestRoutingTableBucketFull(t *testing.T) {
	self, _ := GenerateNodeID()
	rt := NewRoutingTable(self)

	// Fill one bucket with K nodes
	for i := range K + 2 {
		var nodeID NodeID
		nodeID[0] = self[0] ^ 0x80 // Same bucket (bucket 0)
		nodeID[19] = byte(i)       // Different IDs
		node := &NodeInfo{
			ID:   nodeID,
			Addr: &net.UDPAddr{IP: net.IPv4(192, 168, 1, byte(i+1)), Port: 6881},
		}
		added := rt.AddNode(node)
		if i < K && !added {
			t.Errorf("Should have added node %d", i)
		}
		if i >= K && added {
			t.Errorf("Should not have added node %d (bucket full)", i)
		}
	}

	if rt.Size() != K {
		t.Errorf("Expected %d nodes, got %d", K, rt.Size())
	}
}

func TestNodeInfoString(t *testing.T) {
	node := &NodeInfo{
		ID:   NodeID{0xDE, 0xAD, 0xBE, 0xEF, 0xCA, 0xFE, 0xBA, 0xBE},
		Addr: &net.UDPAddr{IP: net.IPv4(192, 168, 1, 1), Port: 6881},
	}
	s := node.String()
	if !bytes.Contains([]byte(s), []byte("deadbeef")) {
		t.Errorf("String should contain node ID prefix: %s", s)
	}
}

// DHT server tests

func TestDHTNew(t *testing.T) {
	dht, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	if dht.ID == (NodeID{}) {
		t.Error("DHT should have a node ID")
	}
	if dht.routingTable == nil {
		t.Error("DHT should have a routing table")
	}
	if dht.transactions == nil {
		t.Error("DHT should have a transaction manager")
	}
}

func TestDHTGenerateToken(t *testing.T) {
	dht, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	token1 := dht.generateToken()
	token2 := dht.generateToken()

	if len(token1) == 0 {
		t.Error("Token should not be empty")
	}
	if token1 == token2 {
		t.Error("Tokens should be unique")
	}
}

func TestDHTRoutingTableIntegration(t *testing.T) {
	dht, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	// Add some nodes to the routing table
	nodes := []*NodeInfo{
		{ID: NodeID{1}, Addr: &net.UDPAddr{IP: net.IPv4(192, 168, 1, 1), Port: 6881}},
		{ID: NodeID{2}, Addr: &net.UDPAddr{IP: net.IPv4(192, 168, 1, 2), Port: 6882}},
	}
	for _, n := range nodes {
		dht.routingTable.AddNode(n)
	}

	// Verify nodes are in routing table
	if dht.routingTable.Size() != 2 {
		t.Errorf("Expected 2 nodes, got %d", dht.routingTable.Size())
	}

	// Test compact encoding of nodes
	for _, n := range nodes {
		compact, err := n.CompactIPv4()
		if err != nil {
			t.Errorf("CompactIPv4 failed: %v", err)
		}
		// Each node: 20 bytes ID + 4 bytes IP + 2 bytes port = 26 bytes
		if len(compact) != 26 {
			t.Errorf("Expected 26 bytes, got %d", len(compact))
		}
	}
}

func TestDHTPeerStore(t *testing.T) {
	dht, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	infoHash := [20]byte{0xDE, 0xAD, 0xBE, 0xEF}

	// Initially empty
	dht.peerStoreMu.RLock()
	peers := dht.peerStore[infoHash]
	dht.peerStoreMu.RUnlock()
	if len(peers) != 0 {
		t.Error("Peer store should be empty initially")
	}

	// Add a peer
	dht.peerStoreMu.Lock()
	dht.peerStore[infoHash] = []string{"192.168.1.1:6881"}
	dht.peerStoreMu.Unlock()

	dht.peerStoreMu.RLock()
	peers = dht.peerStore[infoHash]
	dht.peerStoreMu.RUnlock()
	if len(peers) != 1 {
		t.Error("Should have 1 peer")
	}
}

func TestParsePeerList(t *testing.T) {
	// Create compact peer data: 192.168.1.1:6881
	data := string([]byte{192, 168, 1, 1, 0x1A, 0xE1})
	peers := parsePeerList(data)
	if len(peers) != 1 {
		t.Fatalf("Expected 1 peer, got %d", len(peers))
	}
	if peers[0] != "192.168.1.1:6881" {
		t.Errorf("Expected 192.168.1.1:6881, got %s", peers[0])
	}
}

func TestParsePeerListMultiple(t *testing.T) {
	// Create compact peer data: 2 peers
	data := string([]byte{
		192, 168, 1, 1, 0x1A, 0xE1, // 192.168.1.1:6881
		10, 0, 0, 1, 0x1A, 0xE2, // 10.0.0.1:6882
	})
	peers := parsePeerList(data)
	if len(peers) != 2 {
		t.Fatalf("Expected 2 peers, got %d", len(peers))
	}
	if peers[0] != "192.168.1.1:6881" {
		t.Errorf("Expected 192.168.1.1:6881, got %s", peers[0])
	}
	if peers[1] != "10.0.0.1:6882" {
		t.Errorf("Expected 10.0.0.1:6882, got %s", peers[1])
	}
}

func TestRandomIDInBucket(t *testing.T) {
	dht, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	// Generate IDs for different buckets and verify distance
	for _, bucketIdx := range []int{0, 50, 100, 159} {
		target := dht.randomIDInBucket(bucketIdx)
		dist := Distance(dht.ID, target)
		gotBucket := dist.LeadingZeros()
		if gotBucket != bucketIdx {
			t.Errorf("Bucket %d: expected leading zeros %d, got %d", bucketIdx, bucketIdx, gotBucket)
		}
	}
}
