package client

import (
	"os"
	"path/filepath"

	"github.com/matei-oltean/go-torrent/fileutils"
	"github.com/matei-oltean/go-torrent/utils"
)

// Client represents a client that wants to download a single file
type Client struct {
	ID       [20]byte
	File     *fileutils.TorrentFile
	PeerAddr *fileutils.TrackerResponse
	folder   string
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
	var port uint16 = 6881
	trackerURL := ""
	var response *fileutils.TrackerResponse
	// Try ports until 6889
	for ; port < 6890 && response == nil; port++ {
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
		ID:       id,
		File:     torrentFile,
		PeerAddr: response,
		folder:   folder,
	}, nil
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
	file := make([]byte, client.File.PieceLength)
	defer outFile.Close()
	_, err = outFile.Write(file)
	return err
}
