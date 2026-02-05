package torrent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// DownloadState represents the persistent state of a download
type DownloadState struct {
	InfoHash     [20]byte `json:"infoHash"`
	Name         string   `json:"name"`
	OutputDir    string   `json:"outputDir"`
	TotalPieces  int      `json:"totalPieces"`
	PieceLength  int      `json:"pieceLength"`
	TotalLength  int      `json:"totalLength"`
	Downloaded   bitfield `json:"downloaded"`   // Which pieces are complete
	Peers        []string `json:"peers"`        // Known peer addresses
	TorrentPath  string   `json:"torrentPath"`  // Path to .torrent file (if available)
	MagnetLink   string   `json:"magnetLink"`   // Magnet link (if available)

	mu sync.RWMutex
}

// StateDir returns the directory where state files are stored
func StateDir() string {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		cacheDir = os.TempDir()
	}
	stateDir := filepath.Join(cacheDir, "go-torrent", "state")
	os.MkdirAll(stateDir, 0755)
	return stateDir
}

// StateFile returns the path to the state file for a given info hash
func StateFile(infoHash [20]byte) string {
	return filepath.Join(StateDir(), fmt.Sprintf("%x.json", infoHash))
}

// StateFileByHex returns the path to the state file for a given hex info hash
func StateFileByHex(infoHashHex string) string {
	return filepath.Join(StateDir(), infoHashHex+".json")
}

// DeleteStateByHex deletes the state file for a given hex info hash
func DeleteStateByHex(infoHashHex string) error {
	path := StateFileByHex(infoHashHex)
	err := os.Remove(path)
	if os.IsNotExist(err) {
		return nil // Already deleted or never existed
	}
	return err
}

// NewDownloadState creates a new download state
func NewDownloadState(infoHash [20]byte, name string, outputDir string, totalPieces, pieceLength, totalLength int) *DownloadState {
	return &DownloadState{
		InfoHash:    infoHash,
		Name:        name,
		OutputDir:   outputDir,
		TotalPieces: totalPieces,
		PieceLength: pieceLength,
		TotalLength: totalLength,
		Downloaded:  make(bitfield, (totalPieces+7)/8),
		Peers:       make([]string, 0),
	}
}

// LoadState loads a download state from disk
func LoadState(infoHash [20]byte) (*DownloadState, error) {
	path := StateFile(infoHash)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read state file: %w", err)
	}

	var state DownloadState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to parse state file: %w", err)
	}

	return &state, nil
}

// Save persists the download state to disk
func (s *DownloadState) Save() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize state: %w", err)
	}

	path := StateFile(s.InfoHash)
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write state file: %w", err)
	}

	return nil
}

// Delete removes the state file from disk
func (s *DownloadState) Delete() error {
	path := StateFile(s.InfoHash)
	return os.Remove(path)
}

// MarkPieceComplete marks a piece as downloaded
func (s *DownloadState) MarkPieceComplete(index int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Downloaded.set(index)
}

// ClearPiece marks a piece as not downloaded (for re-verification)
func (s *DownloadState) ClearPiece(index int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Downloaded.Unset(index)
}

// IsPieceComplete checks if a piece has been downloaded
func (s *DownloadState) IsPieceComplete(index int) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Downloaded.get(index)
}

// CompletedPieces returns the number of completed pieces
func (s *DownloadState) CompletedPieces() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	count := 0
	for i := range s.TotalPieces {
		if s.Downloaded.get(i) {
			count++
		}
	}
	return count
}

// Progress returns the download progress as a percentage
func (s *DownloadState) Progress() float64 {
	if s.TotalPieces == 0 {
		return 0
	}
	return float64(s.CompletedPieces()) / float64(s.TotalPieces) * 100
}

// IsComplete returns true if all pieces have been downloaded
func (s *DownloadState) IsComplete() bool {
	return s.CompletedPieces() == s.TotalPieces
}

// AddPeers adds peer addresses to the state
func (s *DownloadState) AddPeers(peers []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	// Deduplicate
	existing := make(map[string]bool)
	for _, p := range s.Peers {
		existing[p] = true
	}
	for _, p := range peers {
		if !existing[p] {
			s.Peers = append(s.Peers, p)
			existing[p] = true
		}
	}
}

// SetTorrentPath sets the path to the .torrent file
func (s *DownloadState) SetTorrentPath(path string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.TorrentPath = path
}

// SetMagnetLink sets the magnet link
func (s *DownloadState) SetMagnetLink(link string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.MagnetLink = link
}
