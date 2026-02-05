package dht

import (
	"bytes"
	"testing"
)

func TestEncodePing(t *testing.T) {
	var nodeID NodeID
	copy(nodeID[:], "abcdefghij0123456789")

	encoded := EncodePing("aa", nodeID)

	// Should be a valid bencoded dict
	if encoded[0] != 'd' || encoded[len(encoded)-1] != 'e' {
		t.Error("Should be a bencoded dictionary")
	}

	// Decode it back
	msg, err := DecodeMessage(encoded)
	if err != nil {
		t.Fatalf("Failed to decode: %v", err)
	}

	if msg.TransactionID != "aa" {
		t.Errorf("Expected txID 'aa', got '%s'", msg.TransactionID)
	}
	if msg.Type != QueryType {
		t.Errorf("Expected type 'q', got '%s'", msg.Type)
	}
	if msg.Query != MethodPing {
		t.Errorf("Expected query 'ping', got '%s'", msg.Query)
	}
	if msg.Args["id"] != string(nodeID[:]) {
		t.Error("Node ID mismatch")
	}
}

func TestEncodePingResponse(t *testing.T) {
	var nodeID NodeID
	copy(nodeID[:], "abcdefghij0123456789")

	encoded := EncodePingResponse("aa", nodeID)
	msg, err := DecodeMessage(encoded)
	if err != nil {
		t.Fatalf("Failed to decode: %v", err)
	}

	if msg.TransactionID != "aa" {
		t.Errorf("Expected txID 'aa', got '%s'", msg.TransactionID)
	}
	if msg.Type != ResponseType {
		t.Errorf("Expected type 'r', got '%s'", msg.Type)
	}
	if msg.Response["id"] != string(nodeID[:]) {
		t.Error("Node ID mismatch")
	}
}

func TestEncodeFindNode(t *testing.T) {
	var nodeID, target NodeID
	copy(nodeID[:], "abcdefghij0123456789")
	copy(target[:], "01234567890123456789")

	encoded := EncodeFindNode("bb", nodeID, target)
	msg, err := DecodeMessage(encoded)
	if err != nil {
		t.Fatalf("Failed to decode: %v", err)
	}

	if msg.Query != MethodFindNode {
		t.Errorf("Expected query 'find_node', got '%s'", msg.Query)
	}
	if msg.Args["id"] != string(nodeID[:]) {
		t.Error("Node ID mismatch")
	}
	if msg.Args["target"] != string(target[:]) {
		t.Error("Target mismatch")
	}
}

func TestEncodeGetPeers(t *testing.T) {
	var nodeID NodeID
	var infoHash [20]byte
	copy(nodeID[:], "abcdefghij0123456789")
	copy(infoHash[:], "01onal34567890123456")

	encoded := EncodeGetPeers("cc", nodeID, infoHash)
	msg, err := DecodeMessage(encoded)
	if err != nil {
		t.Fatalf("Failed to decode: %v", err)
	}

	if msg.Query != MethodGetPeers {
		t.Errorf("Expected query 'get_peers', got '%s'", msg.Query)
	}
	if msg.Args["info_hash"] != string(infoHash[:]) {
		t.Error("Info hash mismatch")
	}
}

func TestEncodeError(t *testing.T) {
	encoded := EncodeError("dd", ErrorGeneric, "test error")
	msg, err := DecodeMessage(encoded)
	if err != nil {
		t.Fatalf("Failed to decode: %v", err)
	}

	if msg.Type != ErrorType {
		t.Errorf("Expected type 'e', got '%s'", msg.Type)
	}
	if len(msg.Error) != 2 {
		t.Fatalf("Expected 2 error elements, got %d", len(msg.Error))
	}
	if msg.Error[0].(int) != ErrorGeneric {
		t.Errorf("Expected error code %d, got %v", ErrorGeneric, msg.Error[0])
	}
	if msg.Error[1].(string) != "test error" {
		t.Errorf("Expected error message 'test error', got '%v'", msg.Error[1])
	}
}

func TestTransactionManager(t *testing.T) {
	tm := NewTransactionManager()

	// Generate IDs
	txID1 := tm.NewTransactionID()
	txID2 := tm.NewTransactionID()
	if txID1 == txID2 {
		t.Error("Transaction IDs should be unique")
	}

	// Add pending
	pq := tm.AddPending(txID1, MethodPing, nil)
	if pq == nil {
		t.Fatal("AddPending returned nil")
	}
	if tm.PendingCount() != 1 {
		t.Errorf("Expected 1 pending, got %d", tm.PendingCount())
	}

	// Get pending
	retrieved := tm.GetPending(txID1)
	if retrieved == nil {
		t.Fatal("GetPending returned nil")
	}
	if retrieved.TransactionID != txID1 {
		t.Error("Transaction ID mismatch")
	}
	if tm.PendingCount() != 0 {
		t.Errorf("Expected 0 pending after get, got %d", tm.PendingCount())
	}

	// Get non-existent
	if tm.GetPending("nonexistent") != nil {
		t.Error("Should return nil for non-existent txID")
	}
}

func TestDecodeMessage(t *testing.T) {
	// Real ping query from BEP 5 spec
	data := []byte("d1:ad2:id20:abcdefghij0123456789e1:q4:ping1:t2:aa1:y1:qe")
	msg, err := DecodeMessage(data)
	if err != nil {
		t.Fatalf("Failed to decode: %v", err)
	}

	if msg.TransactionID != "aa" {
		t.Errorf("Expected txID 'aa', got '%s'", msg.TransactionID)
	}
	if msg.Type != QueryType {
		t.Errorf("Expected type 'q', got '%s'", msg.Type)
	}
	if msg.Query != "ping" {
		t.Errorf("Expected query 'ping', got '%s'", msg.Query)
	}
}

func TestExtractNodeID(t *testing.T) {
	var nodeID NodeID
	copy(nodeID[:], "abcdefghij0123456789")

	encoded := EncodePing("aa", nodeID)
	msg, _ := DecodeMessage(encoded)

	extracted, err := msg.ExtractNodeID()
	if err != nil {
		t.Fatalf("Failed to extract: %v", err)
	}
	if extracted != nodeID {
		t.Error("Extracted node ID mismatch")
	}
}

func TestGenerateToken(t *testing.T) {
	token1, err := GenerateToken()
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}
	token2, err := GenerateToken()
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	if token1 == token2 {
		t.Error("Tokens should be unique")
	}
	if len(token1) != 8 {
		t.Errorf("Expected token length 8, got %d", len(token1))
	}
}

func TestEncodeDecodeRoundTrip(t *testing.T) {
	var nodeID NodeID
	copy(nodeID[:], "abcdefghij0123456789")

	// Test each message type
	tests := []struct {
		name    string
		encoded []byte
	}{
		{"ping", EncodePing("aa", nodeID)},
		{"ping_response", EncodePingResponse("bb", nodeID)},
		{"find_node", EncodeFindNode("cc", nodeID, nodeID)},
		{"error", EncodeError("dd", 201, "error")},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			msg, err := DecodeMessage(tc.encoded)
			if err != nil {
				t.Fatalf("Failed to decode %s: %v", tc.name, err)
			}
			if msg.TransactionID == "" {
				t.Errorf("%s: missing transaction ID", tc.name)
			}
		})
	}
}

func TestExtractNodes(t *testing.T) {
	var nodeID NodeID
	copy(nodeID[:], "abcdefghij0123456789")

	// Create compact nodes data (2 nodes)
	nodesData := make([]byte, 52) // 2 * 26 bytes
	copy(nodesData[0:20], nodeID[:])
	nodesData[20] = 192
	nodesData[21] = 168
	nodesData[22] = 1
	nodesData[23] = 1
	nodesData[24] = 0x1A // port 6881
	nodesData[25] = 0xE1

	copy(nodesData[26:46], nodeID[:])
	nodesData[46] = 10
	nodesData[47] = 0
	nodesData[48] = 0
	nodesData[49] = 1
	nodesData[50] = 0x1A
	nodesData[51] = 0xE2 // port 6882

	encoded := EncodeFindNodeResponse("aa", nodeID, nodesData)
	msg, err := DecodeMessage(encoded)
	if err != nil {
		t.Fatalf("Failed to decode: %v", err)
	}

	nodes, err := msg.ExtractNodes(false)
	if err != nil {
		t.Fatalf("Failed to extract nodes: %v", err)
	}

	if len(nodes) != 2 {
		t.Fatalf("Expected 2 nodes, got %d", len(nodes))
	}

	if !bytes.Equal(nodes[0].ID[:], nodeID[:]) {
		t.Error("First node ID mismatch")
	}
	if nodes[0].Addr.Port != 6881 {
		t.Errorf("Expected port 6881, got %d", nodes[0].Addr.Port)
	}
	if nodes[1].Addr.Port != 6882 {
		t.Errorf("Expected port 6882, got %d", nodes[1].Addr.Port)
	}
}
