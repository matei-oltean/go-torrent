package torrent

import (
	"testing"
)

func TestPieceQueueRarestFirst(t *testing.T) {
	// Create 5 pieces
	pieces := []*Piece{
		{Index: 0, Length: 100},
		{Index: 1, Length: 100},
		{Index: 2, Length: 100},
		{Index: 3, Length: 100},
		{Index: 4, Length: 100},
	}
	
	// No pieces completed
	completed := make(bitfield, 1)
	
	queue := NewPieceQueue(pieces, completed)
	
	// Register a peer that has pieces 0, 1, 2 (not 3, 4)
	peer1bf := make(bitfield, 1)
	peer1bf.set(0)
	peer1bf.set(1)
	peer1bf.set(2)
	queue.RegisterPeer(peer1bf)
	
	// Register a peer that has pieces 1, 2, 3 (not 0, 4)
	peer2bf := make(bitfield, 1)
	peer2bf.set(1)
	peer2bf.set(2)
	peer2bf.set(3)
	queue.RegisterPeer(peer2bf)
	
	// Availability: 0->1, 1->2, 2->2, 3->1, 4->0
	
	// A peer with all pieces should get piece 4 first (rarest, 0 availability)
	allbf := make(bitfield, 1)
	allbf.set(0)
	allbf.set(1)
	allbf.set(2)
	allbf.set(3)
	allbf.set(4)
	
	piece := queue.GetPiece(allbf)
	if piece == nil {
		t.Fatal("expected a piece, got nil")
	}
	if piece.Index != 4 {
		t.Errorf("expected piece 4 (rarest), got piece %d", piece.Index)
	}
	queue.Complete(piece.Index)
	
	// Next should be piece 0 or 3 (both have availability 1)
	piece = queue.GetPiece(allbf)
	if piece == nil {
		t.Fatal("expected a piece, got nil")
	}
	if piece.Index != 0 && piece.Index != 3 {
		t.Errorf("expected piece 0 or 3, got piece %d", piece.Index)
	}
}

func TestPieceQueuePeerBitfieldFilter(t *testing.T) {
	pieces := []*Piece{
		{Index: 0, Length: 100},
		{Index: 1, Length: 100},
		{Index: 2, Length: 100},
	}
	
	completed := make(bitfield, 1)
	queue := NewPieceQueue(pieces, completed)
	
	// Peer only has piece 2
	peerbf := make(bitfield, 1)
	peerbf.set(2)
	
	piece := queue.GetPiece(peerbf)
	if piece == nil {
		t.Fatal("expected a piece, got nil")
	}
	if piece.Index != 2 {
		t.Errorf("expected piece 2 (only one peer has), got piece %d", piece.Index)
	}
	
	// No more pieces for this peer
	piece = queue.GetPiece(peerbf)
	if piece != nil {
		t.Errorf("expected nil (no more pieces for this peer), got piece %d", piece.Index)
	}
}

func TestPieceQueueReturn(t *testing.T) {
	pieces := []*Piece{
		{Index: 0, Length: 100},
	}
	
	completed := make(bitfield, 1)
	queue := NewPieceQueue(pieces, completed)
	
	allbf := make(bitfield, 1)
	allbf.set(0)
	
	// Get the piece
	piece := queue.GetPiece(allbf)
	if piece == nil || piece.Index != 0 {
		t.Fatal("expected piece 0")
	}
	
	// No more pieces available
	if queue.GetPiece(allbf) != nil {
		t.Fatal("expected nil (piece in progress)")
	}
	
	// Return the piece
	queue.Return(0)
	
	// Now it should be available again
	piece = queue.GetPiece(allbf)
	if piece == nil || piece.Index != 0 {
		t.Fatal("expected piece 0 to be available again")
	}
}

func TestPieceQueueCompletedSkipped(t *testing.T) {
	pieces := []*Piece{
		{Index: 0, Length: 100},
		{Index: 1, Length: 100},
		{Index: 2, Length: 100},
	}
	
	// Piece 1 is already completed
	completed := make(bitfield, 1)
	completed.set(1)
	
	queue := NewPieceQueue(pieces, completed)
	
	allbf := make(bitfield, 1)
	allbf.set(0)
	allbf.set(1)
	allbf.set(2)
	
	// Should get piece 0 or 2, not 1
	piece := queue.GetPiece(allbf)
	if piece == nil {
		t.Fatal("expected a piece")
	}
	if piece.Index == 1 {
		t.Error("should not return already completed piece 1")
	}
	queue.Complete(piece.Index)
	
	piece = queue.GetPiece(allbf)
	if piece == nil {
		t.Fatal("expected a piece")
	}
	if piece.Index == 1 {
		t.Error("should not return already completed piece 1")
	}
	queue.Complete(piece.Index)
	
	// No more pieces
	piece = queue.GetPiece(allbf)
	if piece != nil {
		t.Errorf("expected nil, got piece %d", piece.Index)
	}
}
