package peer

import (
	"bytes"
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"net"
	"time"

	"github.com/matei-oltean/go-torrent/messaging"
	"github.com/matei-oltean/go-torrent/utils"
)

// chunkSize is the max length that can be downloaded at once
const chunkSize int = 1 << 14

// maxRequests is the max number of requests that can be queued up at once
const maxRequests int = 5

// chunk of a piece
type chunk struct {
	Index int
	Begin int
	Value []byte
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

// Peer represents a connection to a peer
type Peer struct {
	conn     net.Conn
	bitfield utils.Bitfield
	choked   bool
}

// new creates a new peer from a handshake and a peer address
func new(handshake []byte, address string) (*Peer, error) {
	conn, err := net.DialTimeout("tcp", address, 3*time.Second)
	if err != nil {
		return nil, err
	}
	// Performing the handshake
	_, err = conn.Write(handshake)
	if err != nil {
		return nil, err
	}
	// We should get a handshake back
	received := make([]byte, messaging.HandshakeSize)
	n, err := conn.Read(received)
	if err != nil {
		conn.Close()
		return nil, err
	}
	if n != messaging.HandshakeSize {
		conn.Close()
		return nil, fmt.Errorf("received handshake with length %d instead of %d", n, messaging.HandshakeSize)
	}

	// It should have the same protocol
	startLen := 1 + len(messaging.Protocol)
	if !bytes.Equal(received[:startLen], handshake[:startLen]) {
		conn.Close()
		return nil, fmt.Errorf("received handshake with the wrong protocol: %v", received[:startLen])
	}
	// And same metadataHash
	if !bytes.Equal(received[startLen+8:startLen+28], handshake[startLen+8:startLen+28]) {
		conn.Close()
		return nil, fmt.Errorf("expected handshake with metadata\n%v got\n%v instead", handshake[startLen+8:startLen+28], received[startLen+8:startLen+28])
	}

	peerID := [20]byte{}
	copy(peerID[:], received[len(received)-20:])

	// next we should receive a bitfield message
	bitfield, err := messaging.ReadBitfield(conn)
	if err != nil {
		conn.Close()
		return nil, err
	}

	return &Peer{
		conn:     conn,
		bitfield: bitfield,
		choked:   true,
	}, nil
}

// unchoke sends an unchoke message
func (peer *Peer) unchoke() error {
	unchokeMsg := messaging.Unchoke()
	_, err := peer.conn.Write(unchokeMsg)
	return err
}

// interested sends an interested message
func (peer *Peer) interested() error {
	interestedMsg := messaging.Interested()
	_, err := peer.conn.Write(interestedMsg)
	return err
}

// parsePiece parse a parse message as a chunk
func parsePiece(payload []byte) (*chunk, error) {
	// a piece message has the following format:
	// index of the message (4 byte big endian)
	// beginning of the chunk (4 byte big endian)
	// payload
	if len(payload) < 0 {
		return nil, fmt.Errorf("expected message of length at least 8 got %d instead", len(payload))
	}
	index := binary.BigEndian.Uint32(payload[:4])
	begin := binary.BigEndian.Uint32(payload[4:8])
	return &chunk{
		Index: int(index),
		Begin: int(begin),
		Value: payload[8:],
	}, nil
}

// Read reads and parses the first non keepalive message from the connection
// fills the argument and the length of the piece in case of a piece message
func (peer *Peer) read() (*chunk, error) {
	msg, err := messaging.Read(peer.conn)
	if err != nil {
		return nil, err
	}
	switch msg.Type {
	case messaging.MChoke:
		peer.choked = true
	case messaging.MUnchoke:
		peer.choked = false
	case messaging.MHave:
		if len(msg.Payload) != 4 {
			return nil, fmt.Errorf("expected payload length 4 got %d instead", len(msg.Payload))
		}
		peer.set(uint(binary.BigEndian.Uint32(msg.Payload)))
	case messaging.MPiece:
		return parsePiece(msg.Payload)
	}
	return nil, nil
}

// has return whether the peer has a certain piece
func (peer *Peer) has(index int) bool {
	return peer.bitfield.Get(uint(index))
}

// set signifies that a peer has a new piece
func (peer *Peer) set(index uint) {
	peer.bitfield.True(index)
}

// downloadPiece attempts to download a piece from the peer
func (peer *Peer) downloadPiece(piece *Piece) ([]byte, error) {
	downloaded := 0
	start := 0
	inQueue := 0
	res := make([]byte, piece.Length)

	// Add a deadline so that we do not wait for stuck peers
	peer.conn.SetDeadline(time.Now().Add(15 * time.Second))
	defer peer.conn.SetDeadline(time.Time{})

	for downloaded < piece.Length {
		if !peer.choked {
			// pipeline the requests
			for inQueue < maxRequests && start < piece.Length {
				// request the next piece
				beg := start
				length := chunkSize
				if beg+length > piece.Length {
					length = piece.Length - start
				}
				start += length
				request := messaging.Request(piece.Index, beg, length)
				_, err := peer.conn.Write(request)
				if err != nil {
					return nil, err
				}
				inQueue++
			}
		}
		chunk, err := peer.read()
		if err != nil {
			return nil, err
		}
		if chunk != nil {
			// if the chunk has the wrong index, discard it
			if chunk.Index != piece.Index {
				continue
			}
			// if the chunk is too long, return an error
			if chunk.Begin+len(chunk.Value) > piece.Length {
				return nil,
					fmt.Errorf("Received a chunk too long: bound %d for piece of size %d",
						chunk.Begin+len(chunk.Value), piece.Length)
			}
			downloaded += copy(res[chunk.Begin:], chunk.Value)
			inQueue--
		}
	}
	return res, nil
}

// Download creates a new peer that downloads pieces from a file
func Download(handshake []byte, address string, pieces chan *Piece, results chan *Result) {
	peer, err := new(handshake, address)
	if err != nil {
		fmt.Printf("Could not connect to peer at %s\n", address)
		return
	}
	fmt.Printf("Connected to peer at %s\n", address)
	defer peer.conn.Close()
	peer.unchoke()
	peer.interested()

	for piece := range pieces {
		// check if this peer has that piece; put it back if not
		if !peer.has(piece.Index) {
			pieces <- piece
			continue
		}

		res, err := peer.downloadPiece(piece)
		if err != nil {
			fmt.Printf("Could not download piece %d: %s\n", piece.Index, err.Error())
			pieces <- piece
			return
		}

		// check for the piece integrity
		hash := sha1.Sum(res)
		if !bytes.Equal(hash[:], piece.Hash[:]) {
			fmt.Printf("Piece %d has the wrong sum: expected\n%v got\n%v instead",
				piece.Index, piece.Hash, hash)
			pieces <- piece
			continue
		}

		peer.conn.Write(messaging.Have(piece.Index))
		results <- &Result{Index: piece.Index, Value: res}
	}
}
