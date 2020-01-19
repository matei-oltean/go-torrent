package messaging

// MessageType represent the different types of peer messages
type MessageType int

const (
	choke MessageType = iota
	unchoke
	interested
	notInterested
	have
	bitfield
	request
	piece
	cancel
)
