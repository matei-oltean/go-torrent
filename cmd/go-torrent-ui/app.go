package main

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/matei-oltean/go-torrent/torrent"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// TorrentStatus represents the status of a torrent download
type TorrentStatus struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Progress    float64 `json:"progress"`
	DownSpeed   int64   `json:"downSpeed"`
	UpSpeed     int64   `json:"upSpeed"`
	Peers       int     `json:"peers"`
	Seeds       int     `json:"seeds"`
	Size        int64   `json:"size"`
	Downloaded  int64   `json:"downloaded"`
	Status      string  `json:"status"` // "downloading", "paused", "completed", "error"
	Error       string  `json:"error,omitempty"`
	
	// Internal fields for pause/resume (not exposed to JSON)
	torrentPath string
	magnetLink  string
	outputPath  string
}

// App struct
type App struct {
	ctx         context.Context
	torrents    map[string]*TorrentStatus
	cancelFuncs map[string]context.CancelFunc
	mu          sync.RWMutex
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{
		torrents:    make(map[string]*TorrentStatus),
		cancelFuncs: make(map[string]context.CancelFunc),
	}
}

// startup is called when the app starts
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	log.Println("Go Torrent UI started")
}

// GetTorrents returns all torrents
func (a *App) GetTorrents() []TorrentStatus {
	a.mu.RLock()
	defer a.mu.RUnlock()
	
	result := make([]TorrentStatus, 0, len(a.torrents))
	for _, t := range a.torrents {
		result = append(result, *t)
	}
	return result
}

// AddMagnet adds a magnet link for download
func (a *App) AddMagnet(magnetLink string, outputPath string) (string, error) {
	magnet, err := torrent.ParseMagnet(magnetLink)
	if err != nil {
		return "", fmt.Errorf("invalid magnet link: %w", err)
	}

	id := magnet.InfoHashHex()
	
	// Create a cancellable context for this download
	ctx, cancel := context.WithCancel(context.Background())
	
	a.mu.Lock()
	a.torrents[id] = &TorrentStatus{
		ID:         id,
		Name:       magnet.DisplayName(),
		Status:     "starting",
		magnetLink: magnetLink,
		outputPath: outputPath,
	}
	a.cancelFuncs[id] = cancel
	a.mu.Unlock()

	// Start download in background
	go func() {
		err := torrent.DownloadMagnetWithContext(ctx, magnetLink, outputPath)
		a.mu.Lock()
		// Check if torrent still exists (might have been removed)
		if t, ok := a.torrents[id]; ok {
			if err != nil {
				if err == context.Canceled {
					t.Status = "paused"
					t.Error = ""
				} else {
					t.Status = "error"
					t.Error = err.Error()
				}
			} else {
				t.Status = "completed"
				t.Progress = 100
			}
		}
		// Clean up cancel func
		delete(a.cancelFuncs, id)
		a.mu.Unlock()
	}()

	return id, nil
}

// AddTorrentFile adds a .torrent file for download
func (a *App) AddTorrentFile(filePath string, outputPath string) (string, error) {
	tf, err := torrent.OpenTorrent(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open torrent: %w", err)
	}

	id := fmt.Sprintf("%x", tf.Info.Hash)
	
	// Create a cancellable context for this download
	ctx, cancel := context.WithCancel(context.Background())
	
	a.mu.Lock()
	a.torrents[id] = &TorrentStatus{
		ID:          id,
		Name:        tf.Info.Name,
		Size:        int64(tf.Info.Length),
		Status:      "starting",
		torrentPath: filePath,
		outputPath:  outputPath,
	}
	a.cancelFuncs[id] = cancel
	a.mu.Unlock()

	// Start download in background
	go func() {
		err := torrent.DownloadWithContext(ctx, filePath, outputPath)
		a.mu.Lock()
		// Check if torrent still exists (might have been removed)
		if t, ok := a.torrents[id]; ok {
			if err != nil {
				if err == context.Canceled {
					t.Status = "paused"
					t.Error = ""
				} else {
					t.Status = "error"
					t.Error = err.Error()
				}
			} else {
				t.Status = "completed"
				t.Progress = 100
			}
		}
		// Clean up cancel func
		delete(a.cancelFuncs, id)
		a.mu.Unlock()
	}()

	return id, nil
}

// PauseTorrent pauses a downloading torrent
func (a *App) PauseTorrent(id string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	
	t, ok := a.torrents[id]
	if !ok {
		return fmt.Errorf("torrent not found")
	}
	
	if t.Status == "paused" || t.Status == "completed" {
		return nil // Already paused or completed
	}
	
	// Cancel the download (will be marked as paused)
	if cancel, ok := a.cancelFuncs[id]; ok {
		cancel()
		delete(a.cancelFuncs, id)
	}
	
	return nil
}

// ResumeTorrent resumes a paused torrent
func (a *App) ResumeTorrent(id string) error {
	a.mu.Lock()
	
	t, ok := a.torrents[id]
	if !ok {
		a.mu.Unlock()
		return fmt.Errorf("torrent not found")
	}
	
	if t.Status != "paused" && t.Status != "error" {
		a.mu.Unlock()
		return nil // Not paused or error, nothing to resume
	}
	
	// Create new context for resumed download
	ctx, cancel := context.WithCancel(context.Background())
	a.cancelFuncs[id] = cancel
	t.Status = "downloading"
	t.Error = ""
	
	// Store values before unlocking
	magnetLink := t.magnetLink
	torrentPath := t.torrentPath
	outputPath := t.outputPath
	
	a.mu.Unlock()
	
	// Start download in background
	go func() {
		var err error
		if magnetLink != "" {
			err = torrent.DownloadMagnetWithContext(ctx, magnetLink, outputPath)
		} else if torrentPath != "" {
			err = torrent.DownloadWithContext(ctx, torrentPath, outputPath)
		} else {
			err = fmt.Errorf("no source available for resume")
		}
		
		a.mu.Lock()
		if t, ok := a.torrents[id]; ok {
			if err != nil {
				if err == context.Canceled {
					t.Status = "paused"
					t.Error = ""
				} else {
					t.Status = "error"
					t.Error = err.Error()
				}
			} else {
				t.Status = "completed"
				t.Progress = 100
			}
		}
		delete(a.cancelFuncs, id)
		a.mu.Unlock()
	}()
	
	return nil
}

// RemoveTorrent removes a torrent from the list and cancels any ongoing download
func (a *App) RemoveTorrent(id string) {
	a.mu.Lock()
	// Cancel the download if it's still running
	if cancel, ok := a.cancelFuncs[id]; ok {
		cancel()
		delete(a.cancelFuncs, id)
	}
	delete(a.torrents, id)
	a.mu.Unlock()
	
	// Delete the state file (outside lock to avoid blocking)
	if err := torrent.DeleteStateByHex(id); err != nil {
		log.Printf("Failed to delete state file: %v", err)
	}
}

// SelectTorrentFile opens a file dialog to select a .torrent file
func (a *App) SelectTorrentFile() (string, error) {
	selection, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select Torrent File",
		Filters: []runtime.FileFilter{
			{
				DisplayName: "Torrent Files (*.torrent)",
				Pattern:     "*.torrent",
			},
		},
	})
	if err != nil {
		return "", err
	}
	return selection, nil
}

// SelectOutputFolder opens a folder dialog to select an output directory
func (a *App) SelectOutputFolder() (string, error) {
	selection, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select Download Folder",
	})
	if err != nil {
		return "", err
	}
	return selection, nil
}
