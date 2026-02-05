package dht

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"sync"
	"time"
)

// Default DHT configuration
const (
	DefaultPort       = 6881
	MaxPort           = 6889
	MaxPacketSize     = 1500
	BootstrapInterval = 5 * time.Minute
)

// Bootstrap nodes - well-known DHT entry points
var BootstrapNodes = []string{
	"router.bittorrent.com:6881",
	"router.utorrent.com:6881",
	"dht.transmissionbt.com:6881",
}

// DHT represents a DHT node
type DHT struct {
	ID           NodeID
	conn         *net.UDPConn
	port         int
	routingTable *RoutingTable
	transactions *TransactionManager
	peerStore    map[[20]byte][]string // info_hash -> peer addresses
	peerStoreMu  sync.RWMutex

	// Channels for communication
	shutdown chan struct{}
	wg       sync.WaitGroup
}

// New creates a new DHT node
func New() (*DHT, error) {
	nodeID, err := GenerateNodeID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate node ID: %w", err)
	}

	return &DHT{
		ID:           nodeID,
		routingTable: NewRoutingTable(nodeID),
		transactions: NewTransactionManager(),
		peerStore:    make(map[[20]byte][]string),
		shutdown:     make(chan struct{}),
	}, nil
}

// Start starts the DHT node
func (d *DHT) Start(ctx context.Context) error {
	// Try to bind to a port in the standard range
	var conn *net.UDPConn
	var err error
	for port := DefaultPort; port <= MaxPort; port++ {
		addr := &net.UDPAddr{Port: port}
		conn, err = net.ListenUDP("udp", addr)
		if err == nil {
			d.port = port
			break
		}
	}
	if conn == nil {
		return fmt.Errorf("failed to bind to any port in range %d-%d: %w", DefaultPort, MaxPort, err)
	}
	d.conn = conn
	log.Printf("DHT listening on port %d", d.port)

	// Start background goroutines
	d.wg.Go(func() { d.readLoop(ctx) })
	d.wg.Go(func() { d.bootstrapLoop(ctx) })

	return nil
}

// Stop gracefully shuts down the DHT node
func (d *DHT) Stop() {
	close(d.shutdown)
	if d.conn != nil {
		d.conn.Close()
	}
	d.wg.Wait()
}

// Port returns the port the DHT is listening on
func (d *DHT) Port() int {
	return d.port
}

// RoutingTable returns the routing table
func (d *DHT) RoutingTable() *RoutingTable {
	return d.routingTable
}

// readLoop reads incoming UDP packets
func (d *DHT) readLoop(ctx context.Context) {
	buf := make([]byte, MaxPacketSize)

	for {
		select {
		case <-ctx.Done():
			return
		case <-d.shutdown:
			return
		default:
		}

		d.conn.SetReadDeadline(time.Now().Add(1 * time.Second))
		n, addr, err := d.conn.ReadFromUDP(buf)
		if err != nil {
			var netErr net.Error
			if errors.As(err, &netErr) && netErr.Timeout() {
				continue
			}
			select {
			case <-d.shutdown:
				return
			default:
				log.Printf("DHT read error: %v", err)
				continue
			}
		}

		// Handle the message in a goroutine
		data := make([]byte, n)
		copy(data, buf[:n])
		go d.handleMessage(data, addr)
	}
}

// bootstrapLoop periodically refreshes the routing table
func (d *DHT) bootstrapLoop(ctx context.Context) {
	// Initial bootstrap
	d.Bootstrap()

	ticker := time.NewTicker(BootstrapInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-d.shutdown:
			return
		case <-ticker.C:
			// Refresh stale buckets
			stale := d.routingTable.StaleBuckets()
			for _, idx := range stale {
				// Generate a random ID in this bucket and search for it
				target := d.randomIDInBucket(idx)
				d.FindNode(target)
			}
		}
	}
}

// handleMessage processes an incoming KRPC message
func (d *DHT) handleMessage(data []byte, addr *net.UDPAddr) {
	msg, err := DecodeMessage(data)
	if err != nil {
		log.Printf("DHT: failed to decode message from %s: %v", addr, err)
		return
	}

	switch msg.Type {
	case QueryType:
		d.handleQuery(msg, addr)
	case ResponseType:
		d.handleResponse(msg, addr)
	case ErrorType:
		log.Printf("DHT: received error from %s: %v", addr, msg.Error)
	}
}

// handleQuery handles incoming queries
func (d *DHT) handleQuery(msg *Message, addr *net.UDPAddr) {
	// Extract sender's node ID and add to routing table
	senderID, err := msg.ExtractNodeID()
	if err == nil {
		d.routingTable.AddNode(&NodeInfo{
			ID:       senderID,
			Addr:     addr,
			LastSeen: time.Now(),
		})
	}

	var response []byte
	switch msg.Query {
	case MethodPing:
		response = EncodePingResponse(msg.TransactionID, d.ID)

	case MethodFindNode:
		target := msg.Args["target"]
		if len(target) != 20 {
			response = EncodeError(msg.TransactionID, ErrorProtocol, "invalid target")
			break
		}
		var targetID NodeID
		copy(targetID[:], target)
		closest := d.routingTable.ClosestNodes(targetID, K)
		nodes := d.encodeNodes(closest, false)
		response = EncodeFindNodeResponse(msg.TransactionID, d.ID, nodes)

	case MethodGetPeers:
		infoHashStr := msg.Args["info_hash"]
		if len(infoHashStr) != 20 {
			response = EncodeError(msg.TransactionID, ErrorProtocol, "invalid info_hash")
			break
		}
		var infoHash [20]byte
		copy(infoHash[:], infoHashStr)

		token := d.generateToken()

		// Check if we have peers for this info_hash
		d.peerStoreMu.RLock()
		peers := d.peerStore[infoHash]
		d.peerStoreMu.RUnlock()

		if len(peers) > 0 {
			response = EncodeGetPeersResponsePeers(msg.TransactionID, d.ID, token, peers)
		} else {
			// Return closest nodes
			closest := d.routingTable.ClosestNodes(NodeID(infoHash), K)
			nodes := d.encodeNodes(closest, false)
			response = EncodeGetPeersResponseNodes(msg.TransactionID, d.ID, token, nodes)
		}

	default:
		response = EncodeError(msg.TransactionID, ErrorMethodUnknown, "unknown method")
	}

	if response != nil {
		d.conn.WriteToUDP(response, addr)
	}
}

// handleResponse handles incoming responses
func (d *DHT) handleResponse(msg *Message, addr *net.UDPAddr) {
	// Find the pending query
	pq := d.transactions.GetPending(msg.TransactionID)
	if pq == nil {
		return // Unknown transaction, ignore
	}

	// Extract sender's node ID and add to routing table
	senderID, err := msg.ExtractNodeID()
	if err == nil {
		d.routingTable.AddNode(&NodeInfo{
			ID:       senderID,
			Addr:     addr,
			LastSeen: time.Now(),
		})
	}

	// Send response to waiting goroutine
	select {
	case pq.ResponseChan <- msg:
	default:
	}
}

// Ping sends a ping query to the given address
func (d *DHT) Ping(addr *net.UDPAddr) (*Message, error) {
	txID := d.transactions.NewTransactionID()
	query := EncodePing(txID, d.ID)

	pq := d.transactions.AddPending(txID, MethodPing, addr)
	_, err := d.conn.WriteToUDP(query, addr)
	if err != nil {
		d.transactions.GetPending(txID) // Remove pending
		return nil, err
	}

	select {
	case resp := <-pq.ResponseChan:
		return resp, nil
	case <-time.After(QueryTimeout):
		d.transactions.GetPending(txID) // Remove pending
		return nil, fmt.Errorf("ping timeout")
	}
}

// FindNode sends a find_node query
func (d *DHT) FindNode(target NodeID) ([]*NodeInfo, error) {
	// Get closest known nodes to target
	closest := d.routingTable.ClosestNodes(target, K)
	if len(closest) == 0 {
		return nil, fmt.Errorf("no nodes in routing table")
	}

	var allNodes []*NodeInfo
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, node := range closest {
		wg.Go(func() {
			nodes, err := d.findNodeQuery(node.Addr, target)
			if err != nil {
				return
			}
			mu.Lock()
			allNodes = append(allNodes, nodes...)
			mu.Unlock()
		})
	}
	wg.Wait()

	// Add discovered nodes to routing table
	for _, node := range allNodes {
		d.routingTable.AddNode(node)
	}

	return allNodes, nil
}

// findNodeQuery sends a single find_node query
func (d *DHT) findNodeQuery(addr *net.UDPAddr, target NodeID) ([]*NodeInfo, error) {
	txID := d.transactions.NewTransactionID()
	query := EncodeFindNode(txID, d.ID, target)

	pq := d.transactions.AddPending(txID, MethodFindNode, addr)
	_, err := d.conn.WriteToUDP(query, addr)
	if err != nil {
		d.transactions.GetPending(txID)
		return nil, err
	}

	select {
	case resp := <-pq.ResponseChan:
		if resp == nil {
			return nil, fmt.Errorf("nil response")
		}
		return resp.ExtractNodes(false)
	case <-time.After(QueryTimeout):
		d.transactions.GetPending(txID) // Remove pending
		return nil, fmt.Errorf("find_node timeout")
	}
}

// GetPeers searches for peers with the given info hash
func (d *DHT) GetPeers(infoHash [20]byte) ([]string, error) {
	var allPeers []string
	var mu sync.Mutex
	seen := make(map[string]bool)

	// Get closest nodes to info_hash
	closest := d.routingTable.ClosestNodes(NodeID(infoHash), K)
	if len(closest) == 0 {
		return nil, fmt.Errorf("no nodes in routing table")
	}

	var wg sync.WaitGroup
	for _, node := range closest {
		wg.Go(func() {
			peers, nodes, err := d.getPeersQuery(node.Addr, infoHash)
			if err != nil {
				return
			}

			mu.Lock()
			for _, p := range peers {
				if !seen[p] {
					seen[p] = true
					allPeers = append(allPeers, p)
				}
			}
			mu.Unlock()

			// Add discovered nodes to routing table
			for _, n := range nodes {
				d.routingTable.AddNode(n)
			}
		})
	}
	wg.Wait()

	return allPeers, nil
}

// getPeersQuery sends a single get_peers query
func (d *DHT) getPeersQuery(addr *net.UDPAddr, infoHash [20]byte) ([]string, []*NodeInfo, error) {
	txID := d.transactions.NewTransactionID()
	query := EncodeGetPeers(txID, d.ID, infoHash)

	pq := d.transactions.AddPending(txID, MethodGetPeers, addr)
	_, err := d.conn.WriteToUDP(query, addr)
	if err != nil {
		d.transactions.GetPending(txID)
		return nil, nil, err
	}

	select {
	case resp := <-pq.ResponseChan:
		if resp == nil {
			return nil, nil, fmt.Errorf("nil response")
		}

		// Check for peers (values)
		if values, ok := resp.Response["values"]; ok {
			// Parse compact peer list
			peers := parsePeerList(values)
			return peers, nil, nil
		}

		// No peers, extract nodes
		nodes, _ := resp.ExtractNodes(false)
		return nil, nodes, nil

	case <-time.After(QueryTimeout):
		d.transactions.GetPending(txID) // Remove pending
		return nil, nil, fmt.Errorf("get_peers timeout")
	}
}

// Bootstrap connects to well-known DHT nodes
func (d *DHT) Bootstrap() {
	log.Printf("DHT: bootstrapping with %d nodes", len(BootstrapNodes))

	for _, addrStr := range BootstrapNodes {
		addr, err := net.ResolveUDPAddr("udp", addrStr)
		if err != nil {
			continue
		}

		go func(a *net.UDPAddr) {
			// Ping the bootstrap node
			resp, err := d.Ping(a)
			if err != nil {
				return
			}

			// Extract node ID and add to routing table
			nodeID, err := resp.ExtractNodeID()
			if err != nil {
				return
			}
			d.routingTable.AddNode(&NodeInfo{
				ID:       nodeID,
				Addr:     a,
				LastSeen: time.Now(),
			})

			// Find nodes close to ourselves
			d.FindNode(d.ID)
		}(addr)
	}
}

// encodeNodes encodes a slice of nodes to compact format
func (d *DHT) encodeNodes(nodes []*NodeInfo, ipv6 bool) []byte {
	var buf []byte
	for _, n := range nodes {
		var compact []byte
		var err error
		if ipv6 {
			compact, err = n.CompactIPv6()
		} else {
			compact, err = n.CompactIPv4()
		}
		if err == nil {
			buf = append(buf, compact...)
		}
	}
	return buf
}

// generateToken generates a token for get_peers responses.
// We don't store tokens since we don't implement announce_peer (read-only DHT).
func (d *DHT) generateToken() string {
	token, _ := GenerateToken()
	return token
}

// randomIDInBucket generates a random node ID that would fall in the given bucket
func (d *DHT) randomIDInBucket(bucketIdx int) NodeID {
	var target NodeID
	// XOR with self to get desired distance
	copy(target[:], d.ID[:])

	// Set the bit at position bucketIdx
	byteIdx := bucketIdx / 8
	bitIdx := 7 - (bucketIdx % 8)
	target[byteIdx] ^= 1 << bitIdx

	return target
}

// parsePeerList parses compact peer format (6 bytes per peer: 4 IP + 2 port)
func parsePeerList(data string) []string {
	bytes := []byte(data)
	if len(bytes)%6 != 0 {
		return nil
	}

	var peers []string
	for i := 0; i < len(bytes); i += 6 {
		ip := net.IP(bytes[i : i+4])
		port := int(bytes[i+4])<<8 | int(bytes[i+5])
		peers = append(peers, fmt.Sprintf("%s:%d", ip, port))
	}
	return peers
}
