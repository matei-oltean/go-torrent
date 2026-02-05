package dht

import (
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSaveLoadNodes(t *testing.T) {
	// Create a temp file
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "nodes.json")

	// Create routing table with some nodes
	var selfID NodeID
	copy(selfID[:], "self_node_id_1234567")
	rt := NewRoutingTable(selfID)

	nodes := []*NodeInfo{
		{ID: NodeID{1, 2, 3}, Addr: &net.UDPAddr{IP: net.IPv4(192, 168, 1, 1), Port: 6881}},
		{ID: NodeID{4, 5, 6}, Addr: &net.UDPAddr{IP: net.IPv4(10, 0, 0, 1), Port: 6882}},
		{ID: NodeID{7, 8, 9}, Addr: &net.UDPAddr{IP: net.ParseIP("::1"), Port: 6883}},
	}
	for _, n := range nodes {
		rt.AddNode(n)
	}

	// Save
	err := rt.SaveNodes(path)
	if err != nil {
		t.Fatalf("SaveNodes failed: %v", err)
	}

	// Verify file exists and is valid JSON
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}
	if !strings.Contains(string(data), `"version"`) {
		t.Error("File should contain version field")
	}

	// Create new routing table and load
	rt2 := NewRoutingTable(selfID)
	loaded, err := rt2.LoadNodes(path)
	if err != nil {
		t.Fatalf("LoadNodes failed: %v", err)
	}

	if loaded != 3 {
		t.Errorf("Expected to load 3 nodes, got %d", loaded)
	}

	if rt2.Size() != 3 {
		t.Errorf("Expected routing table size 3, got %d", rt2.Size())
	}
}

func TestLoadNodesNonExistent(t *testing.T) {
	var selfID NodeID
	rt := NewRoutingTable(selfID)

	loaded, err := rt.LoadNodes("/nonexistent/path/nodes.json")
	if err != nil {
		t.Errorf("LoadNodes should not error on missing file: %v", err)
	}
	if loaded != 0 {
		t.Errorf("Expected 0 loaded, got %d", loaded)
	}
}

func TestSaveNodesEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "empty.json")

	var selfID NodeID
	rt := NewRoutingTable(selfID)

	// Save empty routing table - should not create file
	err := rt.SaveNodes(path)
	if err != nil {
		t.Fatalf("SaveNodes failed: %v", err)
	}

	// File should not exist (nothing to save)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("Empty routing table should not create file")
	}
}

func TestLoadNodesInvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "invalid.json")

	// Write invalid JSON
	err := os.WriteFile(path, []byte("not valid json"), 0644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	var selfID NodeID
	rt := NewRoutingTable(selfID)

	_, err = rt.LoadNodes(path)
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestSaveLoadIPv6(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "ipv6.json")

	var selfID NodeID
	rt := NewRoutingTable(selfID)

	// Add IPv6 node
	rt.AddNode(&NodeInfo{
		ID:   NodeID{0xAB, 0xCD},
		Addr: &net.UDPAddr{IP: net.ParseIP("2001:db8::1"), Port: 6881},
	})

	err := rt.SaveNodes(path)
	if err != nil {
		t.Fatalf("SaveNodes failed: %v", err)
	}

	rt2 := NewRoutingTable(selfID)
	loaded, err := rt2.LoadNodes(path)
	if err != nil {
		t.Fatalf("LoadNodes failed: %v", err)
	}
	if loaded != 1 {
		t.Fatalf("Expected 1 node, got %d", loaded)
	}

	nodes := rt2.AllNodes()
	if len(nodes) != 1 {
		t.Fatal("Expected 1 node in table")
	}

	// Check IPv6 address preserved
	if nodes[0].Addr.IP.To4() != nil {
		t.Error("Expected IPv6 address, got IPv4")
	}
	if nodes[0].Addr.Port != 6881 {
		t.Errorf("Expected port 6881, got %d", nodes[0].Addr.Port)
	}
}
