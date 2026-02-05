package torrent

import (
	"sync"
)

// PieceQueue manages piece selection with rarest-first strategy using availability buckets.
// Pieces are grouped by how many peers have them, allowing O(maxPeers) lookup instead of O(numPieces).
type PieceQueue struct {
	mu           sync.Mutex
	pieces       []*Piece
	availability []int            // availability[pieceIndex] = number of peers that have it
	buckets      []map[int]bool   // buckets[availCount] = set of pending piece indices
	inProgress   map[int]bool     // pieces currently being downloaded
	completed    map[int]bool     // pieces that have been downloaded
}

// NewPieceQueue creates a new piece queue with the given pieces
func NewPieceQueue(pieces []*Piece, completedBitfield bitfield) *PieceQueue {
	pq := &PieceQueue{
		pieces:       pieces,
		availability: make([]int, len(pieces)),
		buckets:      []map[int]bool{make(map[int]bool)}, // Start with bucket 0
		inProgress:   make(map[int]bool),
		completed:    make(map[int]bool),
	}

	// All pending pieces start in bucket 0 (zero availability)
	for i := range pieces {
		if completedBitfield.get(i) {
			pq.completed[i] = true
		} else {
			pq.buckets[0][i] = true
		}
	}

	return pq
}

// ensureBucket makes sure bucket at given index exists
func (pq *PieceQueue) ensureBucket(avail int) {
	for len(pq.buckets) <= avail {
		pq.buckets = append(pq.buckets, make(map[int]bool))
	}
}

// RegisterPeer adds a peer's bitfield to availability tracking.
// Moves pending pieces to higher availability buckets.
func (pq *PieceQueue) RegisterPeer(bf bitfield) {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	for i := range pq.pieces {
		if bf.get(i) {
			oldAvail := pq.availability[i]
			pq.availability[i]++
			// Move pending pieces to new bucket
			if !pq.completed[i] && !pq.inProgress[i] {
				if oldAvail < len(pq.buckets) {
					delete(pq.buckets[oldAvail], i)
				}
				pq.ensureBucket(oldAvail + 1)
				pq.buckets[oldAvail+1][i] = true
			}
		}
	}
}

// UnregisterPeer removes a peer's bitfield from availability tracking.
// Moves pending pieces to lower availability buckets.
func (pq *PieceQueue) UnregisterPeer(bf bitfield) {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	for i := range pq.pieces {
		if bf.get(i) && pq.availability[i] > 0 {
			oldAvail := pq.availability[i]
			pq.availability[i]--
			// Move pending pieces to new bucket
			if !pq.completed[i] && !pq.inProgress[i] {
				if oldAvail < len(pq.buckets) {
					delete(pq.buckets[oldAvail], i)
				}
				pq.buckets[oldAvail-1][i] = true
			}
		}
	}
}

// GetPiece returns the rarest pending piece that the given peer has.
// Iterates buckets from 0 (rarest) upward - O(maxPeers) instead of O(numPieces).
func (pq *PieceQueue) GetPiece(peerBitfield bitfield) *Piece {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	// Iterate from rarest (bucket 0) to most common
	for avail := 0; avail < len(pq.buckets); avail++ {
		for pieceIdx := range pq.buckets[avail] {
			if peerBitfield.get(pieceIdx) {
				// Found a piece this peer has
				delete(pq.buckets[avail], pieceIdx)
				pq.inProgress[pieceIdx] = true
				return pq.pieces[pieceIdx]
			}
		}
	}

	return nil
}

// Complete marks a piece as successfully downloaded.
func (pq *PieceQueue) Complete(index int) {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	delete(pq.inProgress, index)
	pq.completed[index] = true
}

// Return puts a piece back in the pending queue (download failed).
func (pq *PieceQueue) Return(index int) {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	if pq.inProgress[index] {
		delete(pq.inProgress, index)
		avail := pq.availability[index]
		pq.ensureBucket(avail)
		pq.buckets[avail][index] = true
	}
}

// HasPending returns true if there are pieces waiting to be downloaded.
func (pq *PieceQueue) HasPending() bool {
	pq.mu.Lock()
	defer pq.mu.Unlock()
	for _, bucket := range pq.buckets {
		if len(bucket) > 0 {
			return true
		}
	}
	return false
}

// HasInProgress returns true if any pieces are currently being downloaded.
func (pq *PieceQueue) HasInProgress() bool {
	pq.mu.Lock()
	defer pq.mu.Unlock()
	return len(pq.inProgress) > 0
}

// AllComplete returns true if all pieces are downloaded.
func (pq *PieceQueue) AllComplete() bool {
	pq.mu.Lock()
	defer pq.mu.Unlock()
	return len(pq.completed) == len(pq.pieces)
}

// UpdateAvailability increments availability for a piece (peer sent Have message).
func (pq *PieceQueue) UpdateAvailability(index int) {
	pq.mu.Lock()
	defer pq.mu.Unlock()
	if index < 0 || index >= len(pq.availability) {
		return
	}
	oldAvail := pq.availability[index]
	pq.availability[index]++
	// Move pending pieces to new bucket
	if !pq.completed[index] && !pq.inProgress[index] {
		if oldAvail < len(pq.buckets) {
			delete(pq.buckets[oldAvail], index)
		}
		pq.ensureBucket(oldAvail + 1)
		pq.buckets[oldAvail+1][index] = true
	}
}
