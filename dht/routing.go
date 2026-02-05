package dht

import (
	"sync"
	"time"
)

// K is the maximum number of nodes per bucket (Kademlia constant)
const K = 8

// BucketCount is the number of buckets in the routing table (160 bits)
const BucketCount = 160

// BucketRefreshInterval is how often to refresh stale buckets
const BucketRefreshInterval = 15 * time.Minute

// Bucket is a k-bucket containing up to K nodes
type Bucket struct {
	Nodes       []*NodeInfo
	LastChanged time.Time
}

// RoutingTable is the DHT routing table with 160 k-buckets
type RoutingTable struct {
	Self    NodeID
	Buckets [BucketCount]*Bucket
	mu      sync.RWMutex
}

// NewRoutingTable creates a new routing table for the given node ID
func NewRoutingTable(self NodeID) *RoutingTable {
	rt := &RoutingTable{Self: self}
	for i := range rt.Buckets {
		rt.Buckets[i] = &Bucket{
			Nodes:       make([]*NodeInfo, 0, K),
			LastChanged: time.Now(),
		}
	}
	return rt
}

// AddNode adds or updates a node in the routing table
// Returns true if the node was added, false if the bucket was full
func (rt *RoutingTable) AddNode(node *NodeInfo) bool {
	if node.ID == rt.Self {
		return false // Don't add ourselves
	}

	rt.mu.Lock()
	defer rt.mu.Unlock()

	idx := BucketIndex(rt.Self, node.ID)
	bucket := rt.Buckets[idx]

	// Check if node already exists - move to end (most recently seen)
	for i, n := range bucket.Nodes {
		if n.ID == node.ID {
			bucket.Nodes = append(bucket.Nodes[:i], bucket.Nodes[i+1:]...)
			node.LastSeen = time.Now()
			bucket.Nodes = append(bucket.Nodes, node)
			bucket.LastChanged = time.Now()
			return true
		}
	}

	// Add new node if bucket not full
	if len(bucket.Nodes) < K {
		node.LastSeen = time.Now()
		bucket.Nodes = append(bucket.Nodes, node)
		bucket.LastChanged = time.Now()
		return true
	}

	// Bucket is full - in a full implementation we would ping the oldest node
	// and replace it if it doesn't respond. For now, just reject.
	return false
}

// RemoveNode removes a node from the routing table
func (rt *RoutingTable) RemoveNode(id NodeID) {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	idx := BucketIndex(rt.Self, id)
	bucket := rt.Buckets[idx]

	for i, n := range bucket.Nodes {
		if n.ID == id {
			bucket.Nodes = append(bucket.Nodes[:i], bucket.Nodes[i+1:]...)
			bucket.LastChanged = time.Now()
			return
		}
	}
}

// FindNode returns the node with the given ID if it exists
func (rt *RoutingTable) FindNode(id NodeID) *NodeInfo {
	rt.mu.RLock()
	defer rt.mu.RUnlock()

	idx := BucketIndex(rt.Self, id)
	bucket := rt.Buckets[idx]

	for _, n := range bucket.Nodes {
		if n.ID == id {
			return n
		}
	}
	return nil
}

// ClosestNodes returns up to count nodes closest to the target ID
func (rt *RoutingTable) ClosestNodes(target NodeID, count int) []*NodeInfo {
	rt.mu.RLock()
	defer rt.mu.RUnlock()

	// Collect all nodes
	var all []*NodeInfo
	for _, bucket := range rt.Buckets {
		all = append(all, bucket.Nodes...)
	}

	// Sort by XOR distance to target
	sortByDistance(all, target)

	if len(all) > count {
		all = all[:count]
	}
	return all
}

// sortByDistance sorts nodes by XOR distance to target (in-place)
func sortByDistance(nodes []*NodeInfo, target NodeID) {
	// Simple insertion sort (good enough for small lists)
	for i := 1; i < len(nodes); i++ {
		j := i
		for j > 0 && compareDistance(nodes[j].ID, nodes[j-1].ID, target) < 0 {
			nodes[j], nodes[j-1] = nodes[j-1], nodes[j]
			j--
		}
	}
}

// compareDistance returns -1 if a is closer to target than b, 1 if b is closer, 0 if equal
func compareDistance(a, b, target NodeID) int {
	distA := Distance(a, target)
	distB := Distance(b, target)
	for i := range distA {
		if distA[i] < distB[i] {
			return -1
		}
		if distA[i] > distB[i] {
			return 1
		}
	}
	return 0
}

// Size returns the total number of nodes in the routing table
func (rt *RoutingTable) Size() int {
	rt.mu.RLock()
	defer rt.mu.RUnlock()

	count := 0
	for _, bucket := range rt.Buckets {
		count += len(bucket.Nodes)
	}
	return count
}

// AllNodes returns all nodes in the routing table
func (rt *RoutingTable) AllNodes() []*NodeInfo {
	rt.mu.RLock()
	defer rt.mu.RUnlock()

	var all []*NodeInfo
	for _, bucket := range rt.Buckets {
		all = append(all, bucket.Nodes...)
	}
	return all
}

// StaleBuckets returns indices of buckets that haven't been updated recently
func (rt *RoutingTable) StaleBuckets() []int {
	rt.mu.RLock()
	defer rt.mu.RUnlock()

	var stale []int
	threshold := time.Now().Add(-BucketRefreshInterval)
	for i, bucket := range rt.Buckets {
		if bucket.LastChanged.Before(threshold) && len(bucket.Nodes) > 0 {
			stale = append(stale, i)
		}
	}
	return stale
}
