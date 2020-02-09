package client

import (
	"github.com/matei-oltean/go-torrent/fileutils"
	"github.com/matei-oltean/go-torrent/utils"
)

// Client represents a client that wants to download a single file
type Client struct {
	ID       [20]byte
	File     *fileutils.TorrentFile
	PeerAddr *fileutils.TrackerResponse
}

// New gets a new client from a torrent path
func New(filePath string) (*Client, error) {
	torrentFile, err := fileutils.OpenTorrent(filePath)
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
	}, nil
}
