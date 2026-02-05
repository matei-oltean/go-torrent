package torrent

import (
	"context"
	"crypto/rand"
	"crypto/sha1"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"

	"github.com/matei-oltean/go-torrent/dht"
)

const (
	notificationStep = 5
)

// ProgressCallback is called during download with progress information
type ProgressCallback func(completedPieces, totalPieces int, downloadedBytes, totalBytes int64)

// DownloadOptions configures download behavior
type DownloadOptions struct {
	RarestFirst bool              // Use rarest-first piece selection (better for swarm health)
	OnProgress  ProgressCallback  // Progress callback
}

// fileDescriptor is a file writer plus the remaining bytes to be written
type fileDescriptor struct {
	FileWriter *os.File
	Remaining  int
}

// clientID returns '-', the id 'GT' followed by the version number, '-' and 12 random bytes
func clientID() ([20]byte, error) {
	id := [20]byte{'-', 'G', 'T', '0', '1', '0', '4', '-'}
	_, err := rand.Read(id[8:])
	return id, err
}

// downloadPiecesWithContext retrieves the file as a byte array
// from torrent file, a list of peers and a client ID
// and writes them to the file system. Supports cancellation via context.
// If state is provided, it will be used to skip already downloaded pieces and track progress.
func downloadPiecesWithContext(ctx context.Context, inf *TorrentInfo, peersAddr []string, clientID [20]byte, outDir string, state *DownloadState, opts *DownloadOptions) error {
	fileLen := inf.Length
	pieceLen := inf.PieceLength
	numPieces := len(inf.Pieces)
	files := inf.Files
	numFiles := len(files)
	
	// Create or use provided state
	if state == nil {
		state = NewDownloadState(inf.Hash, inf.Name, outDir, numPieces, pieceLen, fileLen)
		state.AddPeers(peersAddr)
	}
	
	// pieceToFile maps a piece index to the indices of the files it corresponds to
	pieceToFile := make(map[int][]int, numPieces)
	// fWriters maps a file index to its file descriptor
	fWriters := make(map[int]*fileDescriptor, numFiles)
	
	// Cleanup function to close all open file handles and save state
	cleanup := func() {
		for _, val := range fWriters {
			val.FileWriter.Close()
		}
		// Save state on cleanup
		if err := state.Save(); err != nil {
			log.Printf("Failed to save download state: %v", err)
		}
	}
	defer cleanup()
	
	// Check if we're resuming (some pieces already downloaded)
	resuming := state.CompletedPieces() > 0
	
	for i, f := range inf.Files {
		path := filepath.Join(outDir, f.Path)
		os.MkdirAll(filepath.Dir(path), os.ModePerm)
		
		var fd *os.File
		var err error
		
		if resuming {
			// Open existing file for writing
			fd, err = os.OpenFile(path, os.O_RDWR, 0644)
			if err != nil {
				// File doesn't exist, create it
				fd, err = os.Create(path)
				if err != nil {
					return err
				}
				_, err = fd.Seek(int64(f.Length-1), 0)
				if err != nil {
					return err
				}
				_, err = fd.Write([]byte{0})
				if err != nil {
					return err
				}
			}
		} else {
			fd, err = os.Create(path)
			if err != nil {
				return err
			}
			_, err = fd.Seek(int64(f.Length-1), 0)
			if err != nil {
				return err
			}
			_, err = fd.Write([]byte{0})
			if err != nil {
				return err
			}
		}
		fWriters[i] = &fileDescriptor{fd, f.Length}
	}

	// If resuming, verify completed pieces against file data
	if resuming {
		log.Printf("Verifying %d completed pieces...", state.CompletedPieces())
		invalidated := 0
		for i, expectedHash := range inf.Pieces {
			if !state.IsPieceComplete(i) {
				continue
			}
			length := pieceLen
			if i == numPieces-1 && fileLen%pieceLen != 0 {
				length = fileLen % pieceLen
			}
			pieceData := make([]byte, length)
			pieceStart := i * pieceLen
			// Read piece data from file(s)
			for fi, f := range files {
				fileEnd := f.CumStart + f.Length
				if pieceStart+length <= f.CumStart || pieceStart >= fileEnd {
					continue
				}
				fd, ok := fWriters[fi]
				if !ok {
					continue
				}
				resOffset := 0
				fileOffset := pieceStart - f.CumStart
				if fileOffset < 0 {
					resOffset = -fileOffset
					fileOffset = 0
				}
				end := length
				if end+pieceStart > fileEnd {
					end = fileEnd - pieceStart
				}
				fd.FileWriter.ReadAt(pieceData[resOffset:end], int64(fileOffset))
			}
			h := sha1.Sum(pieceData)
			if h != expectedHash {
				state.ClearPiece(i)
				invalidated++
			}
		}
		if invalidated > 0 {
			log.Printf("Invalidated %d corrupted pieces", invalidated)
		}
	}
	
	// Count pieces we need to download (skip already completed ones)
	piecesToDownload := 0
	for i := range numPieces {
		if !state.IsPieceComplete(i) {
			piecesToDownload++
		}
	}
	
	// If already complete, we're done
	if piecesToDownload == 0 {
		log.Printf("Download already complete")
		return nil
	}
	
	log.Printf("Resuming download: %d/%d pieces remaining", piecesToDownload, numPieces)
	
	// Build list of pieces to download
	allPieces := make([]*Piece, numPieces)
	pos := 0
	fileIndex := 0
	for i, hash := range inf.Pieces {
		length := pieceLen
		// The last piece might be shorter
		if i == numPieces-1 && fileLen%pieceLen != 0 {
			length = fileLen % pieceLen
		}
		
		allPieces[i] = &Piece{
			Index:  i,
			Hash:   hash,
			Length: length,
		}
		
		pos += length
		var f []int
		for ; fileIndex < numFiles && files[fileIndex].CumStart+files[fileIndex].Length < pos; fileIndex++ {
			f = append(f, fileIndex)
		}
		f = append(f, fileIndex)
		pieceToFile[i] = f
	}

	// Create chan of results to collect
	results := make(chan *Result)
	
	// done channel signals workers to stop
	done := make(chan struct{})
	defer close(done)

	// Choose piece selection strategy
	useRarestFirst := opts != nil && opts.RarestFirst
	
	if useRarestFirst {
		// Rarest-first: use PieceQueue
		queue := NewPieceQueue(allPieces, state.Downloaded)
		for _, peerAddress := range peersAddr {
			go DownloadPiecesWithQueue(inf.Hash, clientID, peerAddress, queue, results, done)
		}
	} else {
		// Sequential/random: use channel
		pieces := make(chan *Piece)
		info := make(chan *TorrentInfo) // unused but required by DownloadPieces
		for _, peerAddress := range peersAddr {
			go DownloadPieces(inf.Hash, clientID, peerAddress, pieces, info, results)
		}
		// Send pieces that need downloading
		go func() {
			for _, p := range allPieces {
				if !state.IsPieceComplete(p.Index) {
					select {
					case pieces <- p:
					case <-done:
						return
					}
				}
			}
			close(pieces)
		}()
	}

	// Parse the results as they come and copy them to file
	nextNotification := notificationStep
	completedInSession := 0
	for completedInSession < piecesToDownload {
		// Check for cancellation
		select {
		case <-ctx.Done():
			log.Printf("Download cancelled/paused, saving state...")
			return ctx.Err()
		case result := <-results:
			// Mark piece as complete in state
			state.MarkPieceComplete(result.Index)
			completedInSession++
			
			// write to the associated files
			for _, i := range pieceToFile[result.Index] {
				f := files[i]
				pieceStart := result.Index * pieceLen
				// start writing in the file at fileOffset
				// start reading the result at resOffset
				resOffset, fileOffset := 0, pieceStart-f.CumStart
				if fileOffset < 0 {
					resOffset, fileOffset = -fileOffset, 0
				}
				// write the result till end
				end := len(result.Value)
				if end+pieceStart > f.CumStart+f.Length {
					end = f.CumStart + f.Length - pieceStart
				}
				fd := fWriters[i]
				n, err := fd.FileWriter.WriteAt(result.Value[resOffset:end], int64(fileOffset))
				if err != nil {
					return err
				}
				fd.Remaining -= n
				if fd.Remaining == 0 {
					fd.FileWriter.Close()
					delete(fWriters, i)
					log.Printf("Finished downloading %s", filepath.Base(f.Path))
				}
			}

			// Progress based on total pieces (including already downloaded)
			totalCompleted := state.CompletedPieces()
			
			// Call progress callback if provided
			if opts != nil && opts.OnProgress != nil {
				downloadedBytes := min(int64(totalCompleted)*int64(pieceLen), int64(fileLen))
				opts.OnProgress(totalCompleted, numPieces, downloadedBytes, int64(fileLen))
			}
			
			for p := float64(totalCompleted) / float64(numPieces) * 100; p > float64(nextNotification); nextNotification += notificationStep {
				log.Printf("Progress (%.2f%%)", p)
			}
			if completedInSession%10 == 0 {
				log.Printf("Downloaded %d/%d pieces", totalCompleted, numPieces)
				// Save state periodically
				if err := state.Save(); err != nil {
					log.Printf("Warning: failed to save state: %v", err)
				}
			}
		}
	}
	
	// Download complete - delete state file
	if err := state.Delete(); err != nil {
		log.Printf("Warning: failed to delete state file: %v", err)
	}
	
	return nil
}

// DownloadWithContext retrieves the file and saves it to the specified path
// if the path is empty, saves it to the folder of the torrent file
// with the default name coming from the torrent file
// Supports cancellation via context.
func DownloadWithContext(ctx context.Context, torrentPath, outputPath string) error {
	id, err := clientID()
	if err != nil {
		return err
	}
	t, err := OpenTorrent(torrentPath)
	if err != nil {
		return err
	}
	outDir := outputPath
	if outDir == "" {
		outDir = filepath.Dir(torrentPath)
	}
	// If there are multiple files, create a containing folder
	if t.Info.Multi() {
		outDir = filepath.Join(outDir, t.Info.Name)
		os.MkdirAll(outDir, os.ModePerm)
	}
	
	// Try to load existing state for resuming
	state, err := LoadState(t.Info.Hash)
	if err != nil {
		// No existing state, will create new one
		state = nil
	} else {
		log.Printf("Found existing state, resuming download...")
	}
	
	peers, err := t.GetPeers(id)
	if err != nil {
		return err
	}
	log.Printf("Received %d peers from tracker", len(peers.PeersAddresses))
	
	// Create state if not resuming
	if state == nil {
		state = NewDownloadState(t.Info.Hash, t.Info.Name, outDir, len(t.Info.Pieces), t.Info.PieceLength, t.Info.Length)
	}
	state.SetTorrentPath(torrentPath)
	state.AddPeers(peers.PeersAddresses)
	
	return downloadPiecesWithContext(ctx, t.Info, peers.PeersAddresses, id, outDir, state, nil)
}

// Download retrieves the file and saves it to the specified path
// if the path is empty, saves it to the folder of the torrent file
// with the default name coming from the torrent file
func Download(torrentPath, outputPath string) error {
	return DownloadWithContext(context.Background(), torrentPath, outputPath)
}

// DownloadWithProgress downloads a torrent file with progress callback
func DownloadWithProgress(ctx context.Context, torrentPath, outputPath string, opts *DownloadOptions) error {
	id, err := clientID()
	if err != nil {
		return err
	}
	t, err := OpenTorrent(torrentPath)
	if err != nil {
		return err
	}
	outDir := outputPath
	if outDir == "" {
		outDir = filepath.Dir(torrentPath)
	}
	if t.Info.Multi() {
		outDir = filepath.Join(outDir, t.Info.Name)
		os.MkdirAll(outDir, os.ModePerm)
	}
	
	state, err := LoadState(t.Info.Hash)
	if err != nil {
		state = nil
	} else {
		log.Printf("Found existing state, resuming download...")
	}
	
	peers, err := t.GetPeers(id)
	if err != nil {
		return err
	}
	log.Printf("Received %d peers from tracker", len(peers.PeersAddresses))
	
	if state == nil {
		state = NewDownloadState(t.Info.Hash, t.Info.Name, outDir, len(t.Info.Pieces), t.Info.PieceLength, t.Info.Length)
	}
	state.SetTorrentPath(torrentPath)
	state.AddPeers(peers.PeersAddresses)
	
	return downloadPiecesWithContext(ctx, t.Info, peers.PeersAddresses, id, outDir, state, opts)
}

// DownloadMagnetWithProgress downloads a magnet link with progress callback and shared DHT
func DownloadMagnetWithProgress(ctx context.Context, magnetLink, outputPath string, sharedDHT *dht.DHT, opts *DownloadOptions) error {
	magnet, err := ParseMagnet(magnetLink)
	if err != nil {
		return fmt.Errorf("failed to parse magnet link: %w", err)
	}

	id, err := clientID()
	if err != nil {
		return err
	}

	log.Printf("Downloading: %s", magnet.DisplayName())
	log.Printf("Info hash: %s", magnet.InfoHashHex())

	collector := NewPeerCollector()

	if magnet.HasPeers() {
		added := collector.Add(magnet.PeerAddresses, "magnet link")
		if added > 0 {
			log.Printf("Added %d peers from magnet link", added)
		}
	}

	var d *dht.DHT
	dhtCtx, dhtCancel := context.WithCancel(ctx)
	defer dhtCancel()

	if sharedDHT != nil {
		d = sharedDHT
		log.Printf("DHT: using shared node on port %d", d.Port())
	} else {
		d, err = dht.New()
		if err != nil {
			log.Printf("DHT: failed to create: %v", err)
		} else {
			defer d.Stop()
			if err := d.Start(dhtCtx); err != nil {
				log.Printf("DHT: failed to start: %v", err)
				d = nil
			} else {
				log.Printf("DHT: started, bootstrapping...")
				d.Bootstrap()
			}
		}
	}

	if d != nil {
		for _, addr := range magnet.PeerAddresses {
			if udpAddr, err := net.ResolveUDPAddr("udp", addr); err == nil {
				go d.Ping(udpAddr)
			}
		}

		log.Printf("DHT: searching for peers...")
		dhtPeers, err := d.GetPeers(magnet.Hash)
		if err != nil {
			log.Printf("DHT: get_peers failed: %v", err)
		} else {
			added := collector.Add(dhtPeers, "DHT")
			if added > 0 {
				log.Printf("Added %d peers from DHT", added)
			}
		}
	}

	if magnet.HasTrackers() {
		log.Printf("Querying %d trackers...", len(magnet.TrackersURL))
		trackerPeers := QueryTrackers(magnet.TrackersURL, magnet.Hash, id)
		added := collector.Add(trackerPeers, "trackers")
		if added > 0 {
			log.Printf("Added %d peers from trackers", added)
		}
	}

	if collector.Count() == 0 {
		return fmt.Errorf("no peers found from any source")
	}

	log.Printf("Total peers: %d", collector.Count())

	return downloadFromPeersWithContext(ctx, magnet.Hash, id, collector.Peers(), outputPath, magnetLink, opts)
}

// DownloadMagnetWithContext downloads a torrent from a magnet link using DHT and trackers
// Supports cancellation via context.
func DownloadMagnetWithContext(ctx context.Context, magnetLink, outputPath string) error {
	return DownloadMagnetWithDHT(ctx, magnetLink, outputPath, nil)
}

// DownloadMagnetWithDHT downloads a torrent from a magnet link using an optional shared DHT node.
// If sharedDHT is nil, an ephemeral DHT node is created for this download.
func DownloadMagnetWithDHT(ctx context.Context, magnetLink, outputPath string, sharedDHT *dht.DHT) error {
	magnet, err := ParseMagnet(magnetLink)
	if err != nil {
		return fmt.Errorf("failed to parse magnet link: %w", err)
	}

	id, err := clientID()
	if err != nil {
		return err
	}

	log.Printf("Downloading: %s", magnet.DisplayName())
	log.Printf("Info hash: %s", magnet.InfoHashHex())

	// Use peer collector to deduplicate peers
	collector := NewPeerCollector()

	// Add peers from magnet link (x.pe parameter)
	if magnet.HasPeers() {
		added := collector.Add(magnet.PeerAddresses, "magnet link")
		if added > 0 {
			log.Printf("Added %d peers from magnet link", added)
		}
	}

	// Start DHT for peer discovery
	var d *dht.DHT
	// Use the passed context for DHT and download
	dhtCtx, dhtCancel := context.WithCancel(ctx)
	defer dhtCancel()

	if sharedDHT != nil {
		// Reuse shared DHT node
		d = sharedDHT
		log.Printf("DHT: using shared node on port %d", d.Port())
	} else {
		d, err = dht.New()
		if err != nil {
			log.Printf("DHT: failed to create: %v", err)
		} else {
			defer d.Stop()

			if err := d.Start(dhtCtx); err != nil {
				log.Printf("DHT: failed to start: %v", err)
				d = nil
			} else {
				log.Printf("DHT: started, bootstrapping...")
				d.Bootstrap()
			}
		}
	}

	if d != nil {
		// Add magnet peer addresses to DHT routing table
		for _, addr := range magnet.PeerAddresses {
			if udpAddr, err := net.ResolveUDPAddr("udp", addr); err == nil {
				go d.Ping(udpAddr)
			}
		}

		log.Printf("DHT: searching for peers...")
		dhtPeers, err := d.GetPeers(magnet.Hash)
		if err != nil {
			log.Printf("DHT: get_peers failed: %v", err)
		} else {
			added := collector.Add(dhtPeers, "DHT")
			if added > 0 {
				log.Printf("Added %d peers from DHT", added)
			}
		}
	}

	// Query trackers from magnet link in parallel
	if magnet.HasTrackers() {
		log.Printf("Querying %d trackers...", len(magnet.TrackersURL))
		trackerPeers := QueryTrackers(magnet.TrackersURL, magnet.Hash, id)
		added := collector.Add(trackerPeers, "trackers")
		if added > 0 {
			log.Printf("Added %d peers from trackers", added)
		}
	}

	if collector.Count() == 0 {
		return fmt.Errorf("no peers found from any source")
	}

	log.Printf("Total peers: %d", collector.Count())

	// Fetch metadata and download file
	return downloadFromPeersWithContext(ctx, magnet.Hash, id, collector.Peers(), outputPath, magnetLink, nil)
}

// DownloadMagnet downloads a torrent from a magnet link using DHT and trackers
func DownloadMagnet(magnetLink, outputPath string) error {
	return DownloadMagnetWithContext(context.Background(), magnetLink, outputPath)
}

// downloadFromPeersWithContext fetches metadata from peers and downloads the torrent
// Supports cancellation via context.
func downloadFromPeersWithContext(ctx context.Context, infoHash, clientID [20]byte, peers []string, outputPath string, magnetLink string, opts *DownloadOptions) error {
	// Try to load existing state for resuming
	state, err := LoadState(infoHash)
	if err != nil {
		state = nil
	} else {
		log.Printf("Found existing state, resuming download...")
	}
	
	// Create channels for metadata exchange
	pieces := make(chan *Piece)
	info := make(chan *TorrentInfo)
	results := make(chan *Result)

	// Start workers to get metadata
	for _, peerAddress := range peers {
		go DownloadPieces(infoHash, clientID, peerAddress, pieces, info, results)
	}

	// Wait for metadata from any peer
	log.Printf("Fetching torrent metadata from peers...")
	select {
	case <-ctx.Done():
		return ctx.Err()
	case torrentInfo := <-info:
		log.Printf("Received metadata: %s (%d pieces)", torrentInfo.Name, len(torrentInfo.Pieces))

		// Set up output directory
		outDir := outputPath
		if torrentInfo.Multi() {
			outDir = filepath.Join(outDir, torrentInfo.Name)
			os.MkdirAll(outDir, os.ModePerm)
		}

		// Create state if not resuming
		if state == nil {
			state = NewDownloadState(torrentInfo.Hash, torrentInfo.Name, outDir, len(torrentInfo.Pieces), torrentInfo.PieceLength, torrentInfo.Length)
		}
		state.SetMagnetLink(magnetLink)
		state.AddPeers(peers)

		// Download the actual file
		return downloadPiecesWithContext(ctx, torrentInfo, peers, clientID, outDir, state, opts)
	}
}
