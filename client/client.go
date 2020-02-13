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

// clientID returns the id -GT0000- followed by 12 random bytes
func clientID() [20]byte {
	id := [20]byte{'-', 'G', 'T', '0', '1', '0', '0', '-'}
	rand.Read(id[8:])
	return id
}

// getPeers returns the list of peers from a torrent file and client ID
func getPeers(torrentFile *fileutils.TorrentFile, clientID [20]byte) ([]string, error) {
	var response *fileutils.TrackerResponse
	var err error
	// Try ports from 6881 till 6889 in accordance with the specifications
	for port := 6881; port < 6890 && response == nil; port++ {
		trackerURL, err := torrentFile.GetAnnounceURL(clientID, port)
		if err != nil {
			return nil, err
		}
		response, err = fileutils.GetTrackerResponse(trackerURL)
		if err == nil {
			return response.PeersAddresses, nil
		}
	}
	return nil, err
}

// downloadPieces retrieves the file as a byte array
// from torrent file, a list of peers and a client ID
func downloadPieces(torrentFile *fileutils.TorrentFile, peersAddr []string, clientID [20]byte) ([]byte, error) {
	fileLen := torrentFile.Length
	pieceLen := torrentFile.PieceLength
	numPieces := len(torrentFile.Pieces)
	file := make([]byte, fileLen)
	// Create chan of pieces to download
	pieces := make(chan *peer.Piece, fileLen)
	// Create chan of results to collect
	results := make(chan *peer.Result)
	for i, hash := range torrentFile.Pieces {
		length := pieceLen
		// The last piece is shorter
		if i == numPieces-1 && fileLen%pieceLen != 0 {
			i = fileLen % pieceLen
		}
		pieces <- &peer.Piece{
			Index:  i,
			Hash:   hash,
			Length: length,
		}
	}

	handshake := messaging.GenerateHandshake(torrentFile.Hash, clientID)

	// Create workers to download the pieces
	for _, peerAddress := range peersAddr {
		go peer.Download(handshake, peerAddress, pieces, results)
	}

	// Parse the results as they come and copy them to file
	done := 0
	for done < numPieces {
		result := <-results
		copy(file[result.Index*pieceLen:], result.Value)
		done++
		log.Printf("Downloaded %d/%d pieces (%.2f%%)", done, numPieces, float64(done)/float64(numPieces)*100)
	}

	return file, nil
}

// Download retrieves the file and saves it to the specified path
// if the path is empty, saves it to the folder of the torrent file
// with the default name coming from the torrent file
func Download(torrentPath, outputPath string) error {
	id := clientID()
	torrentFile, err := fileutils.OpenTorrent(torrentPath)
	if err != nil {
		return err
	}
	outPath := outputPath
	if outPath == "" {
		outPath = filepath.Join(filepath.Dir(torrentPath), torrentFile.Name)
	}
	outFile, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer outFile.Close()
	peers, err := getPeers(torrentFile, id)
	if err != nil {
		return err
	}
	file, err := downloadPieces(torrentFile, peers, id)
	if err != nil {
		return err
	}
	_, err = outFile.Write(file)
	if err != nil {
		return err
	}
	log.Printf("Successfully saved file at %s", outPath)
	return nil
}
