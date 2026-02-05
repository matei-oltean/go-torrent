package main

import (
	"crypto/rand"
	"log"
	"os"
	"path/filepath"
)

const (
	notificationStep = 5
)

// fileDescriptor is a file writer plus the remaining bytes to be writtent
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

// downloadPieces retrieves the file as a byte array
// from torrent file, a list of peers and a client ID
// and writes them to the file system
func downloadPieces(inf *TorrentInfo, peersAddr []string, clientID [20]byte, outDir string) error {
	fileLen := inf.Length
	pieceLen := inf.PieceLength
	numPieces := len(inf.Pieces)
	files := inf.Files
	numFiles := len(files)
	// pieceToFile maps a piece index to the indices of the files it corresponds to
	pieceToFile := make(map[int][]int, numPieces)
	// fReaders maps a file index to its file reader
	fReaders := make(map[int]*fileDescriptor, numFiles)
	defer func() {
		for _, val := range fReaders {
			val.FileWriter.Close()
		}
	}()
	for i, f := range inf.Files {
		path := filepath.Join(outDir, f.Path)
		os.MkdirAll(filepath.Dir(path), os.ModePerm)
		fd, err := os.Create(path)
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
		fReaders[i] = &fileDescriptor{fd, f.Length}
	}
	// Create chan of pieces to download
	pieces := make(chan *Piece, fileLen)
	// Create chan of results to collect
	results := make(chan *Result)
	pos := 0
	fileIndex := 0
	for i, hash := range inf.Pieces {
		length := pieceLen
		// The last piece might be shorter
		if i == numPieces-1 && fileLen%pieceLen != 0 {
			length = fileLen % pieceLen
		}
		pieces <- &Piece{
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

	// create chan to receive the torrent metadata in case we need it
	info := make(chan *TorrentInfo)

	// Create workers to download the pieces
	for _, peerAddress := range peersAddr {
		go DownloadPieces(inf.Hash, clientID, peerAddress, pieces, info, results)
	}

	// Parse the results as they come and copy them to file
	nextNotification := notificationStep
	for done := 1; done <= numPieces; done++ {
		result := <-results
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
			fd := fReaders[i]
			n, err := fd.FileWriter.WriteAt(result.Value[resOffset:end], int64(fileOffset))
			if err != nil {
				return err
			}
			fd.Remaining -= n
			if fd.Remaining == 0 {
				fd.FileWriter.Close()
				delete(fReaders, i)
				log.Printf("Finished downloading %s", filepath.Base(f.Path))
			}
		}

		for p := float64(done) / float64(numPieces) * 100; p > float64(nextNotification); nextNotification += notificationStep {
			log.Printf("Progress (%.2f%%)", p)
		}
		if done%10 == 0 {
			log.Printf("Downloaded %d/%d pieces", done, numPieces)
		}
	}
	return nil
}

// Download retrieves the file and saves it to the specified path
// if the path is empty, saves it to the folder of the torrent file
// with the default name coming from the torrent file
func Download(torrentPath, outputPath string) error {
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
	peers, err := t.GetPeers(id)
	if err != nil {
		return err
	}
	log.Printf("Received %d peers from tracker", len(peers.PeersAddresses))
	err = downloadPieces(t.Info, peers.PeersAddresses, id, outDir)
	if err != nil {
		return err
	}
	return nil
}
