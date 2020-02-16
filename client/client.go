package client

import (
	"crypto/rand"
	"log"
	"os"
	"path/filepath"

	"github.com/matei-oltean/go-torrent/fileutils"
	"github.com/matei-oltean/go-torrent/messaging"
	"github.com/matei-oltean/go-torrent/peer"
)

// clientID returns '-', the id 'GT' followed by the version number, '-' and 12 random bytes
func clientID() [20]byte {
	id := [20]byte{'-', 'G', 'T', '0', '1', '0', '3', '-'}
	rand.Read(id[8:])
	return id
}

// downloadPieces retrieves the file as a byte array
// from torrent file, a list of peers and a client ID
// and writes them to the file system
func downloadPieces(torrentFile *fileutils.TorrentFile, peersAddr []string, clientID [20]byte, outDir string) error {
	fileLen := torrentFile.Length
	pieceLen := torrentFile.PieceLength
	numPieces := len(torrentFile.Pieces)
	files := make([]byte, fileLen)
	// Create chan of pieces to download
	pieces := make(chan *peer.Piece, fileLen)
	// Create chan of results to collect
	results := make(chan *peer.Result)
	for i, hash := range torrentFile.Pieces {
		length := pieceLen
		// The last piece might be shorter
		if i == numPieces-1 && fileLen%pieceLen != 0 {
			length = fileLen % pieceLen
		}
		pieces <- &peer.Piece{
			Index:  i,
			Hash:   hash,
			Length: length,
		}
	}

	handshake := messaging.Handshake(torrentFile.Hash, clientID)

	// Create workers to download the pieces
	for _, peerAddress := range peersAddr {
		go peer.Download(handshake, peerAddress, pieces, results)
	}

	// Parse the results as they come and copy them to file
	done := 0
	for done < numPieces {
		result := <-results
		copy(files[result.Index*pieceLen:], result.Value)
		done++
		log.Printf("Downloaded %d/%d pieces (%.2f%%)", done, numPieces, float64(done)/float64(numPieces)*100)
	}
	start := 0
	for _, file := range torrentFile.Files {
		outPath := outDir
		for _, dir := range file.Path {
			outPath = filepath.Join(outPath, dir)
		}
		os.MkdirAll(filepath.Dir(outPath), os.ModePerm)
		outFile, err := os.Create(outPath)
		if err != nil {
			return err
		}
		defer outFile.Close()
		_, err = outFile.Write(files[start : start+file.Length])
		start += file.Length
		if err != nil {
			return err
		}
		log.Printf("Successfully saved file at %s", outPath)

	}
	return nil
}

// Download retrieves the file and saves it to the specified path
// if the path is empty, saves it to the folder of the torrent file
// with the default name coming from the torrent file
func Download(torrentPath, outputPath string) error {
	id := clientID()
	t, err := fileutils.OpenTorrent(torrentPath)
	if err != nil {
		return err
	}
	outDir := outputPath
	if outDir == "" {
		outDir = filepath.Dir(torrentPath)
	}
	// If there are multiple files, create a containing folder
	if t.Multi() {
		outDir = filepath.Join(outDir, t.Name)
		os.MkdirAll(outDir, os.ModePerm)
	}
	peers, err := t.GetPeers(id)
	if err != nil {
		return err
	}
	log.Printf("Received %d peers from tracker", len(peers))
	err = downloadPieces(t, peers, id, outDir)
	if err != nil {
		return err
	}
	return nil
}
