package fileutils

// TorrentInfo represents the torrent information
// Currently only supports single file content
type TorrentInfo struct {
	Name        string
	PieceLength uint32
	Pieces      string
	Length      uint32
}

// TorrentFile represents a torrent file
type TorrentFile struct {
	Announce string
	Info     TorrentInfo
}
