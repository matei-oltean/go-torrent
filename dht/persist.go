package dht

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"
)

// DefaultNodesFile is the default filename for persisted nodes
const DefaultNodesFile = ".dht_nodes.json"

// nodeJSON is the JSON representation of a DHT node
type nodeJSON struct {
	ID   string `json:"id"`   // hex-encoded node ID
	Addr string `json:"addr"` // "ip:port"
}

// nodesFile is the JSON file structure
type nodesFile struct {
	Version int        `json:"version"`
	Nodes   []nodeJSON `json:"nodes"`
}

// SaveNodes persists the routing table nodes to a JSON file
func (rt *RoutingTable) SaveNodes(path string) error {
	nodes := rt.AllNodes()
	if len(nodes) == 0 {
		return nil // Nothing to save
	}

	// Create parent directory if needed
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Convert to JSON format
	file := nodesFile{
		Version: 1,
		Nodes:   make([]nodeJSON, len(nodes)),
	}
	for i, node := range nodes {
		file.Nodes[i] = nodeJSON{
			ID:   fmt.Sprintf("%x", node.ID),
			Addr: node.Addr.String(),
		}
	}

	// Write JSON
	data, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// LoadNodes loads nodes from a JSON file and adds them to the routing table
// Returns the number of nodes loaded
func (rt *RoutingTable) LoadNodes(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil // No file yet
		}
		return 0, fmt.Errorf("failed to read file: %w", err)
	}

	var file nodesFile
	if err := json.Unmarshal(data, &file); err != nil {
		return 0, fmt.Errorf("failed to parse JSON: %w", err)
	}

	// Load each node
	loaded := 0
	for _, n := range file.Nodes {
		node, err := parseNodeJSON(n)
		if err != nil {
			continue // Skip invalid entries
		}
		if rt.AddNode(node) {
			loaded++
		}
	}

	return loaded, nil
}

func parseNodeJSON(n nodeJSON) (*NodeInfo, error) {
	// Parse node ID from hex
	var id NodeID
	if len(n.ID) != 40 {
		return nil, fmt.Errorf("invalid node ID length")
	}
	for i := range 20 {
		var b byte
		_, err := fmt.Sscanf(n.ID[i*2:i*2+2], "%02x", &b)
		if err != nil {
			return nil, fmt.Errorf("invalid node ID hex: %w", err)
		}
		id[i] = b
	}

	// Parse address
	addr, err := net.ResolveUDPAddr("udp", n.Addr)
	if err != nil {
		return nil, fmt.Errorf("invalid address: %w", err)
	}

	return &NodeInfo{
		ID:       id,
		Addr:     addr,
		LastSeen: time.Now(),
	}, nil
}
