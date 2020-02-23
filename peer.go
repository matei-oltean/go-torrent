package main

import (
	"bytes"
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"time"
)

// chunkSize is the max length that can be downloaded at once
// it is also the size of the payload for a metadata message
const chunkSize int = 1 << 14

// maxRequests is the max number of requests that can be queued up at once
const maxRequests int = 5

type chunkType int

const (
	cFile chunkType = iota // a piece of the file
	cInfo                  // a piece of the info dictionary
)

// chunk of a piece
type chunk struct {
	chunkType chunkType // indicates the type of chunk
	index     int
	begin     int
	value     []byte
}

// Piece represents a piece to be downloaded:
// its index, hash and length
type Piece struct {
	Index  int
	Hash   [20]byte
	Length int
}

// Result is a downloaded chunk of the file:
// its index and value
type Result struct {
	Index int
	Value []byte
}

// peer represents a connection to a peer
type peer struct {
	conn         net.Conn
	bitfield     bitfield
	choked       bool
	extensions   map[string]uint8
	metadataSize int
}

// newPeer creates a new peer from a handshake and a peer address
func newPeer(handshake []byte, address string) (*peer, error) {
	conn, err := net.DialTimeout("tcp", address, 5*time.Second)
	if err != nil {
		return nil, err
	}
	// Performing the handshake
	_, err = conn.Write(handshake)
	if err != nil {
		return nil, err
	}
	// We should get a handshake back
	received := make([]byte, HandshakeSize)
	n, err := conn.Read(received)
	if err != nil {
		conn.Close()
		return nil, err
	}
	if n != HandshakeSize {
		conn.Close()
		return nil, fmt.Errorf("received handshake with length %d instead of %d", n, HandshakeSize)
	}

	// It should have the same protocol
	startLen := 1 + len(Protocol)
	if !bytes.Equal(received[:startLen], handshake[:startLen]) {
		conn.Close()
		return nil, fmt.Errorf("received handshake with the wrong protocol: %s", received[:startLen])
	}
	// And same metadataHash
	if !bytes.Equal(received[startLen+8:startLen+28], handshake[startLen+8:startLen+28]) {
		conn.Close()
		return nil, fmt.Errorf("expected handshake with metadata\n%v got\n%v instead", handshake[startLen+8:startLen+28], received[startLen+8:startLen+28])
	}

	// check for extensions
	var ext map[string]uint8
	size := 0
	extensions := received[startLen : startLen+8]
	if extensions[5]&0x10 != 0 {
		payload, err := ReadExtensions(conn)
		if err != nil {
			conn.Close()
			return nil, err
		}
		msgType := payload[0]
		if msgType == 0 { // handshake
			ext, size, err = ParseExtensionsHandshake(payload[1:])
			if err != nil {
				conn.Close()
				return nil, err
			}
		}
	}

	// next we should receive a bitfield message
	bitfield, err := ReadBitfield(conn)
	if err != nil {
		conn.Close()
		return nil, err
	}

	return &peer{
		conn:         conn,
		bitfield:     bitfield,
		choked:       true,
		extensions:   ext,
		metadataSize: size,
	}, nil
}

// unchoke sends a unchoke message
func (p *peer) unchoke() error {
	unchokeMsg := Unchoke()
	_, err := p.conn.Write(unchokeMsg)
	return err
}

// startConn sends an unchoke message followed by an interested
func (p *peer) startConn() error {
	err := p.unchoke()
	if err != nil {
		return err
	}
	interestedMsg := Interested()
	_, err = p.conn.Write(interestedMsg)
	return err
}

// parsePiece parse a parse message as a chunk
func parsePiece(payload []byte) (*chunk, error) {
	// a piece message has the following format:
	// index of the message (4 byte big endian)
	// beginning of the chunk (4 byte big endian)
	// payload
	if len(payload) < 8 {
		return nil, fmt.Errorf("expected message of length at least 8 got %d instead", len(payload))
	}
	index := binary.BigEndian.Uint32(payload[:4])
	begin := binary.BigEndian.Uint32(payload[4:8])
	return &chunk{
		chunkType: cFile,
		index:     int(index),
		begin:     int(begin),
		value:     payload[8:],
	}, nil
}

// parseExtended parse an extended message as a chunk
func (p *peer) parseExtended(payload []byte) (*chunk, error) {
	// an extended message has the following format:
	// uint8 extended message id
	// bencoded dictionary
	// piece data (16 KiB unless it is the last piece)
	if len(payload) < 1 {
		return nil, fmt.Errorf("expected message of length at least 1 got %d instead", len(payload))
	}
	parsed, index, err := ParseExtensionsMetadata(payload[1:])
	if err != nil || parsed == nil {
		return nil, err
	}
	return &chunk{
		chunkType: cInfo,
		index:     index,
		begin:     index * chunkSize,
		value:     parsed,
	}, nil
}

// read reads and parses the first non keepalive message from the connection
// fills the argument and the length of the piece in case of a piece message
func (p *peer) read() (*chunk, error) {
	msg, err := ReadMessage(p.conn)
	if err != nil {
		return nil, err
	}
	switch msg.Type {
	case MChoke:
		p.choked = true
		p.unchoke()
	case MUnchoke:
		p.choked = false
	case MHave:
		if len(msg.Payload) != 4 {
			return nil, fmt.Errorf("expected payload length 4 got %d instead", len(msg.Payload))
		}
		p.bitfield.set(int(binary.BigEndian.Uint32(msg.Payload)))
	case MPiece:
		return parsePiece(msg.Payload)
	case MExtended:
		return p.parseExtended(msg.Payload)
	}
	return nil, nil
}

// downloadPiece attempts to download a piece from the peer
// info is true if we want to download the metadata instead of the file
func (p *peer) downloadPiece(piece *Piece, info bool) ([]byte, error) {
	downloaded := 0
	start := 0
	inQueue := 0
	res := make([]byte, piece.Length)
	i := 0
	// Add a deadline so that we do not wait for stuck peers
	p.conn.SetDeadline(time.Now().Add(20 * time.Second))
	defer p.conn.SetDeadline(time.Time{})

	for downloaded < piece.Length {
		for ; !p.choked && inQueue < maxRequests && start < piece.Length; inQueue++ {
			// request the next piece
			// last piece might be shorter
			length := chunkSize
			if start+length > piece.Length {
				length = piece.Length - start
			}
			var req []byte
			if info {
				req = RequestMetaData(p.extensions["ut_metadata"], i)
			} else {
				req = RequestPiece(piece.Index, start, length)
			}
			_, err := p.conn.Write(req)
			if err != nil {
				return nil, err
			}
			start += length
			i++
		}
		// we want to read all the buffered messages
		// and at least one in case we are waiting for unchoking
		for ok := true; ok; ok = inQueue > 0 {
			chunk, err := p.read()
			if err != nil {
				return nil, err
			}
			// if it is not a chunk or if the chunk has the wrong index, continue
			if chunk == nil {
				continue
			}
			if (info && chunk.chunkType != cInfo) ||
				(!info && (chunk.chunkType != cFile || chunk.index != piece.Index)) {
				continue
			}
			// if the chunk is too long, return an error
			if chunk.begin+len(chunk.value) > piece.Length {
				return nil,
					fmt.Errorf("received a chunk too long: bound %d for piece of size %d",
						chunk.begin+len(chunk.value), piece.Length)
			}
			downloaded += copy(res[chunk.begin:], chunk.value)
			inQueue--
		}
	}
	return res, nil
}

// DownloadPieces creates a new peer that downloads pieces from a file
func DownloadPieces(hash, clientID [20]byte, address string, pieces chan *Piece, info chan<- *TorrentInfo, results chan<- *Result) {
	handshake := Handshake(hash, clientID)
	peer, err := newPeer(handshake, address)
	if err != nil {
		log.Printf("Could not connect to peer at %s: %s", address, err)
		return
	}
	defer peer.conn.Close()
	err = peer.startConn()
	if err != nil {
		log.Printf("Could not connect to peer at %s: %s", address, err)
		return
	}
	log.Printf("Connected to peer at %s", address)

	// if there are pieces, download them
	// if not, we should download the info file
	needsInfo := true
	for {
		select {
		case piece := <-pieces:
			needsInfo = false // there must already been an info file provided
			// check if this peer has that piece; put it back if not
			if !peer.bitfield.get(piece.Index) {
				pieces <- piece
				continue
			}

			res, err := peer.downloadPiece(piece, false)
			if err != nil {
				log.Printf("Disconnecting from peer at %s: %s", address, err)
				pieces <- piece
				return
			}

			// check for the piece integrity
			h := sha1.Sum(res)
			if !bytes.Equal(h[:], piece.Hash[:]) {
				log.Printf("Piece %d has the wrong sum: expected\n%v got\n%v instead", piece.Index, piece.Hash, hash)
				pieces <- piece
				continue
			}

			peer.conn.Write(Have(piece.Index))
			results <- &Result{Index: piece.Index, Value: res}
		default:
			if !needsInfo {
				return
			}
			// we must download the metadata
			res, err := peer.downloadPiece(&Piece{Length: peer.metadataSize}, true)
			if err != nil {
				log.Printf("Disconnecting from peer at %s: %s", address, err)
				return
			}
			h := sha1.Sum(res)
			if !bytes.Equal(hash[:], h[:]) {
				continue
			}
			inf, err := ParseInfo(res, hash)
			if err != nil {
				log.Printf("Disconnecting from peer at %s: %s", address, err)
				return
			}
			needsInfo = false
			info <- inf
		}
	}
}
