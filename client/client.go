package client

import (
	"log"
	"os"
	"path/filepath"

	"github.com/matei-oltean/go-torrent/fileutils"
	"github.com/matei-oltean/go-torrent/messaging"
	"github.com/matei-oltean/go-torrent/peer"
	"github.com/matei-oltean/go-torrent/utils"
)

// Client represents a client that wants to download a single file
type Client struct {
	ID        [20]byte
	File      *fileutils.TorrentFile
	PeersAddr []string
	folder    string
}

// New gets a new client from a torrent path
func New(torrentPath string) (*Client, error) {
	// save the folder to know where to save the file
	folder := filepath.Dir(torrentPath)
	torrentFile, err := fileutils.OpenTorrent(torrentPath)
	if err != nil {
		return nil, err
	}
	id := utils.ClientID()
	trackerURL := ""
	var response *fileutils.TrackerResponse
	// Try ports from 6881 till 6889 in accordance with the specifications
	for port := 6881; port < 6890 && response == nil; port++ {
		trackerURL, err = torrentFile.GetAnnounceURL(id, port)
		if err != nil {
			return nil, err
		}
		response, err = fileutils.GetTrackerResponse(trackerURL)
	}
	if err != nil {
		return nil, err
	}
	return &Client{
		ID:        id,
		File:      torrentFile,
		PeersAddr: response.PeersAddresses,
		folder:    folder,
	}, nil
}

// downloadPieces retrieves the file as a byte array
func (client *Client) downloadPieces() ([]byte, error) {
	fileLen := client.File.Length
	pieceLen := client.File.PieceLength
	numPieces := len(client.File.Pieces)
	file := make([]byte, fileLen)
	// Create chan of pieces to download
	pieces := make(chan *peer.Piece, fileLen)
	// Create chan of results to collect
	results := make(chan *peer.Result)
	for i, hash := range client.File.Pieces {
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

	handshake := messaging.GenerateHandshake(client.File.Hash, client.ID)

	// Create workers to download the pieces
	for _, peerAddress := range client.PeersAddr {
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
func (client *Client) Download(path string) error {
	outPath := path
	if outPath == "" {
		outPath = filepath.Join(client.folder, client.File.Name)
	}
	outFile, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer outFile.Close()
	file, err := client.downloadPieces()
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
