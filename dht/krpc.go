package dht

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"
)

// KRPC message types
const (
	QueryType    = "q"
	ResponseType = "r"
	ErrorType    = "e"
)

// KRPC query methods
const (
	MethodPing     = "ping"
	MethodFindNode = "find_node"
	MethodGetPeers = "get_peers"
	MethodAnnounce = "announce_peer"
)

// KRPC error codes
const (
	ErrorGeneric       = 201
	ErrorServer        = 202
	ErrorProtocol      = 203
	ErrorMethodUnknown = 204
)

// QueryTimeout is the default timeout for KRPC queries
const QueryTimeout = 15 * time.Second

// Message represents a KRPC message (query, response, or error)
type Message struct {
	TransactionID string            // "t" - transaction ID
	Type          string            // "y" - message type: q, r, or e
	Query         string            // "q" - query method name (for queries)
	Args          map[string]string // "a" - query arguments
	Response      map[string]string // "r" - response values
	Error         []any             // "e" - error [code, message]
}

// PendingQuery tracks an outgoing query waiting for response
type PendingQuery struct {
	TransactionID string
	Method        string
	Target        *net.UDPAddr
	SentAt        time.Time
	ResponseChan  chan *Message
}

// TransactionManager manages KRPC transaction IDs and pending queries
type TransactionManager struct {
	pending map[string]*PendingQuery
	mu      sync.RWMutex
	counter uint16
}

// NewTransactionManager creates a new transaction manager
func NewTransactionManager() *TransactionManager {
	return &TransactionManager{
		pending: make(map[string]*PendingQuery),
	}
}

// NewTransactionID generates a new 2-byte transaction ID
func (tm *TransactionManager) NewTransactionID() string {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.counter++
	return string([]byte{byte(tm.counter >> 8), byte(tm.counter)})
}

// AddPending registers a pending query
func (tm *TransactionManager) AddPending(txID, method string, target *net.UDPAddr) *PendingQuery {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	pq := &PendingQuery{
		TransactionID: txID,
		Method:        method,
		Target:        target,
		SentAt:        time.Now(),
		ResponseChan:  make(chan *Message, 1),
	}
	tm.pending[txID] = pq
	return pq
}

// GetPending retrieves and removes a pending query
func (tm *TransactionManager) GetPending(txID string) *PendingQuery {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	pq := tm.pending[txID]
	delete(tm.pending, txID)
	return pq
}

// CleanupExpired removes expired pending queries
func (tm *TransactionManager) CleanupExpired(timeout time.Duration) []*PendingQuery {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	var expired []*PendingQuery
	now := time.Now()
	for txID, pq := range tm.pending {
		if now.Sub(pq.SentAt) > timeout {
			expired = append(expired, pq)
			delete(tm.pending, txID)
			close(pq.ResponseChan)
		}
	}
	return expired
}

// PendingCount returns the number of pending queries
func (tm *TransactionManager) PendingCount() int {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return len(tm.pending)
}

// EncodePing creates a ping query message
func EncodePing(txID string, nodeID NodeID) []byte {
	msg := map[string]any{
		"t": txID,
		"y": QueryType,
		"q": MethodPing,
		"a": map[string]any{
			"id": string(nodeID[:]),
		},
	}
	return encodeMessage(msg)
}

// EncodePingResponse creates a ping response message
func EncodePingResponse(txID string, nodeID NodeID) []byte {
	msg := map[string]any{
		"t": txID,
		"y": ResponseType,
		"r": map[string]any{
			"id": string(nodeID[:]),
		},
	}
	return encodeMessage(msg)
}

// EncodeFindNode creates a find_node query message
func EncodeFindNode(txID string, nodeID, target NodeID) []byte {
	msg := map[string]any{
		"t": txID,
		"y": QueryType,
		"q": MethodFindNode,
		"a": map[string]any{
			"id":     string(nodeID[:]),
			"target": string(target[:]),
		},
	}
	return encodeMessage(msg)
}

// EncodeFindNodeResponse creates a find_node response message
func EncodeFindNodeResponse(txID string, nodeID NodeID, nodes []byte) []byte {
	msg := map[string]any{
		"t": txID,
		"y": ResponseType,
		"r": map[string]any{
			"id":    string(nodeID[:]),
			"nodes": string(nodes),
		},
	}
	return encodeMessage(msg)
}

// EncodeGetPeers creates a get_peers query message
func EncodeGetPeers(txID string, nodeID NodeID, infoHash [20]byte) []byte {
	msg := map[string]any{
		"t": txID,
		"y": QueryType,
		"q": MethodGetPeers,
		"a": map[string]any{
			"id":        string(nodeID[:]),
			"info_hash": string(infoHash[:]),
		},
	}
	return encodeMessage(msg)
}

// EncodeGetPeersResponseNodes creates a get_peers response with nodes (no peers found)
func EncodeGetPeersResponseNodes(txID string, nodeID NodeID, token string, nodes []byte) []byte {
	msg := map[string]any{
		"t": txID,
		"y": ResponseType,
		"r": map[string]any{
			"id":    string(nodeID[:]),
			"token": token,
			"nodes": string(nodes),
		},
	}
	return encodeMessage(msg)
}

// EncodeGetPeersResponsePeers creates a get_peers response with peers
func EncodeGetPeersResponsePeers(txID string, nodeID NodeID, token string, peers []string) []byte {
	peerList := make([]any, len(peers))
	for i, p := range peers {
		peerList[i] = p
	}
	msg := map[string]any{
		"t": txID,
		"y": ResponseType,
		"r": map[string]any{
			"id":     string(nodeID[:]),
			"token":  token,
			"values": peerList,
		},
	}
	return encodeMessage(msg)
}

// EncodeError creates an error response message
func EncodeError(txID string, code int, message string) []byte {
	msg := map[string]any{
		"t": txID,
		"y": ErrorType,
		"e": []any{code, message},
	}
	return encodeMessage(msg)
}

// encodeMessage converts a message map to bencoded bytes
func encodeMessage(msg map[string]any) []byte {
	return encodeBencode(msg)
}

// encodeBencode encodes a Go value to bencode format
func encodeBencode(v any) []byte {
	var buf bytes.Buffer
	encodeBencodeTo(&buf, v)
	return buf.Bytes()
}

func encodeBencodeTo(buf *bytes.Buffer, v any) {
	switch val := v.(type) {
	case string:
		buf.WriteString(fmt.Sprintf("%d:%s", len(val), val))
	case int:
		buf.WriteString(fmt.Sprintf("i%de", val))
	case []any:
		buf.WriteByte('l')
		for _, item := range val {
			encodeBencodeTo(buf, item)
		}
		buf.WriteByte('e')
	case map[string]any:
		buf.WriteByte('d')
		// Keys must be sorted
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		sortStringSlice(keys)
		for _, k := range keys {
			buf.WriteString(fmt.Sprintf("%d:%s", len(k), k))
			encodeBencodeTo(buf, val[k])
		}
		buf.WriteByte('e')
	}
}

func sortStringSlice(s []string) {
	for i := 1; i < len(s); i++ {
		j := i
		for j > 0 && s[j] < s[j-1] {
			s[j], s[j-1] = s[j-1], s[j]
			j--
		}
	}
}

// DecodeMessage parses a bencoded KRPC message
func DecodeMessage(data []byte) (*Message, error) {
	ben, err := decodeBencode(data)
	if err != nil {
		return nil, err
	}
	dict, ok := ben.(map[string]any)
	if !ok {
		return nil, errors.New("KRPC message must be a dictionary")
	}

	msg := &Message{}

	// Transaction ID
	if t, ok := dict["t"].(string); ok {
		msg.TransactionID = t
	} else {
		return nil, errors.New("missing transaction ID")
	}

	// Message type
	if y, ok := dict["y"].(string); ok {
		msg.Type = y
	} else {
		return nil, errors.New("missing message type")
	}

	switch msg.Type {
	case QueryType:
		if q, ok := dict["q"].(string); ok {
			msg.Query = q
		}
		if a, ok := dict["a"].(map[string]any); ok {
			msg.Args = make(map[string]string)
			for k, v := range a {
				if s, ok := v.(string); ok {
					msg.Args[k] = s
				}
			}
		}
	case ResponseType:
		if r, ok := dict["r"].(map[string]any); ok {
			msg.Response = make(map[string]string)
			for k, v := range r {
				if s, ok := v.(string); ok {
					msg.Response[k] = s
				}
			}
		}
	case ErrorType:
		if e, ok := dict["e"].([]any); ok {
			msg.Error = e
		}
	}

	return msg, nil
}

// decodeBencode parses bencoded data into Go values
func decodeBencode(data []byte) (any, error) {
	reader := bufio.NewReader(bytes.NewReader(data))
	return decodeBencodeValue(reader)
}

func decodeBencodeValue(reader *bufio.Reader) (any, error) {
	b, err := reader.ReadByte()
	if err != nil {
		return nil, err
	}

	switch b {
	case 'd':
		dict := make(map[string]any)
		for {
			peek, err := reader.Peek(1)
			if err != nil {
				return nil, err
			}
			if peek[0] == 'e' {
				reader.ReadByte()
				return dict, nil
			}
			key, err := decodeBencodeValue(reader)
			if err != nil {
				return nil, err
			}
			keyStr, ok := key.(string)
			if !ok {
				return nil, errors.New("dict key must be string")
			}
			val, err := decodeBencodeValue(reader)
			if err != nil {
				return nil, err
			}
			dict[keyStr] = val
		}
	case 'l':
		var list []any
		for {
			peek, err := reader.Peek(1)
			if err != nil {
				return nil, err
			}
			if peek[0] == 'e' {
				reader.ReadByte()
				return list, nil
			}
			val, err := decodeBencodeValue(reader)
			if err != nil {
				return nil, err
			}
			list = append(list, val)
		}
	case 'i':
		numStr, err := reader.ReadString('e')
		if err != nil {
			return nil, err
		}
		var num int
		fmt.Sscanf(numStr[:len(numStr)-1], "%d", &num)
		return num, nil
	default:
		reader.UnreadByte()
		lenStr, err := reader.ReadString(':')
		if err != nil {
			return nil, err
		}
		var length int
		fmt.Sscanf(lenStr[:len(lenStr)-1], "%d", &length)
		buf := make([]byte, length)
		_, err = reader.Read(buf)
		if err != nil {
			return nil, err
		}
		return string(buf), nil
	}
}

// GenerateToken creates a random token for announce validation (8 hex chars)
func GenerateToken() (string, error) {
	return rand.Text()[:8], nil
}

// ExtractNodeID extracts the node ID from a KRPC message
func (m *Message) ExtractNodeID() (NodeID, error) {
	var id NodeID
	var idStr string

	if m.Type == QueryType && m.Args != nil {
		idStr = m.Args["id"]
	} else if m.Type == ResponseType && m.Response != nil {
		idStr = m.Response["id"]
	}

	if len(idStr) != 20 {
		return id, fmt.Errorf("invalid node ID length: %d", len(idStr))
	}
	copy(id[:], idStr)
	return id, nil
}

// ExtractNodes extracts compact node info from a find_node or get_peers response
func (m *Message) ExtractNodes(ipv6 bool) ([]*NodeInfo, error) {
	if m.Response == nil {
		return nil, errors.New("no response data")
	}

	key := "nodes"
	if ipv6 {
		key = "nodes6"
	}

	nodesStr, ok := m.Response[key]
	if !ok {
		return nil, nil // No nodes in response
	}

	return ParseCompactNodes([]byte(nodesStr), ipv6)
}
