package main

import (
	"testing"
)

func TestPeerCollector(t *testing.T) {
	c := NewPeerCollector()

	// Add first batch
	added := c.Add([]string{"1.2.3.4:6881", "5.6.7.8:6882"}, "source1")
	if added != 2 {
		t.Errorf("expected 2 added, got %d", added)
	}
	if c.Count() != 2 {
		t.Errorf("expected count 2, got %d", c.Count())
	}

	// Add with duplicates
	added = c.Add([]string{"1.2.3.4:6881", "9.10.11.12:6883"}, "source2")
	if added != 1 {
		t.Errorf("expected 1 added (duplicate filtered), got %d", added)
	}
	if c.Count() != 3 {
		t.Errorf("expected count 3, got %d", c.Count())
	}

	// Add all duplicates
	added = c.Add([]string{"1.2.3.4:6881", "5.6.7.8:6882"}, "source3")
	if added != 0 {
		t.Errorf("expected 0 added (all duplicates), got %d", added)
	}

	// Check peers list
	peers := c.Peers()
	if len(peers) != 3 {
		t.Errorf("expected 3 peers, got %d", len(peers))
	}
}

func TestPeerCollectorEmpty(t *testing.T) {
	c := NewPeerCollector()

	if c.Count() != 0 {
		t.Errorf("expected count 0, got %d", c.Count())
	}

	peers := c.Peers()
	if len(peers) != 0 {
		t.Errorf("expected 0 peers, got %d", len(peers))
	}

	// Add empty list
	added := c.Add(nil, "empty")
	if added != 0 {
		t.Errorf("expected 0 added from nil, got %d", added)
	}

	added = c.Add([]string{}, "empty2")
	if added != 0 {
		t.Errorf("expected 0 added from empty slice, got %d", added)
	}
}
