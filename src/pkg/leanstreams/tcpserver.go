package leanstreams

import (
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
)

const (
	headerByteSize = 8
)

var (
	// ErrZeroBytesReadHeader is thrown when the value parsed from the header is not valid
	ErrZeroBytesReadHeader = errors.New("0 Bytes parsed from header. Connection Closed")
	// ErrLessThanZeroBytesReadHeader is thrown when the value parsed from the header caused some kind of underrun
	ErrLessThanZeroBytesReadHeader = errors.New("Less than zero bytes parsed from header. Connection Closed")
)

// TCPServer is an abstraction over the normal net.TCPConn, but optimized for wtiting
// data encoded in a length+data format, like you would treat networked protocol
// buffer messages
type TCPServer struct {
	// General
	socket         net.Conn
	address        string
	tlsConfig      *tls.Config
	headerByteSize int
	MaxMessageSize int

	// For processing incoming data
	incomingHeaderBuffer []byte

	// For processing outgoing data
	writeLock          sync.Mutex
	outgoingDataBuffer []byte
}

// TCPServerConfig representss the information needed to begin listening for
// writing messages.
type TCPServerConfig struct {
	// Controls how large the largest Message may be. The server will reject any messages whose clients
	// header size does not match this configuration.
	MaxMessageSize int
	// Address is the address to connect to for writing streaming messages.
	Address string

	TLSConfig *tls.Config
}

func newTCPServer(cfg *TCPServerConfig) *TCPServer {
	maxMessageSize := DefaultMaxMessageSize
	// 0 is the default, and the message must be atleast 1 byte large
	if cfg.MaxMessageSize != 0 {
		maxMessageSize = cfg.MaxMessageSize
	}

	return &TCPServer{
		MaxMessageSize:       maxMessageSize,
		headerByteSize:       headerByteSize,
		address:              cfg.Address,
		incomingHeaderBuffer: make([]byte, headerByteSize),
		writeLock:            sync.Mutex{},
		outgoingDataBuffer:   make([]byte, maxMessageSize),
		tlsConfig:            cfg.TLSConfig,
	}
}

// Close will immediately call close on the connection to the remote endpoint. Per
// the golang source code for the netFD object, this call uses a special mutex to
// control access to the underlying pool of readers/writers. This call should be
// threadsafe, so that any other threads writing will finish, or be blocked, when
// this is invoked.
func (c *TCPServer) Close() error {
	return c.socket.Close()
}

func (c *TCPServer) lowLevelRead(buffer []byte) (int, error) {
	var totalBytesRead = 0
	var err error
	var bytesRead = 0
	var toRead = len(buffer)
	// This fills the buffer
	bytesRead, err = c.socket.Read(buffer)
	totalBytesRead += bytesRead
	for totalBytesRead < toRead && err == nil {
		bytesRead, err = c.socket.Read(buffer[totalBytesRead:])
		totalBytesRead += bytesRead
	}

	// Output the content of the bytes to the queue
	if totalBytesRead == 0 && err != nil && err == io.EOF {
		// "End of individual transmission"
		// We're just done reading from that conn
		return totalBytesRead, err
	} else if err != nil {
		//"Underlying network failure?"
		// Not sure what this error would be, but it could exist and i've seen it handled
		// as a general case in other networking code. Following in the footsteps of (greatness|madness)
		return totalBytesRead, err
	}
	// Read some bytes, return the length

	return totalBytesRead, nil
}

func (c *TCPServer) Read(b []byte) (int, error) {
	// Read the header
	hLength, err := c.lowLevelRead(c.incomingHeaderBuffer)
	if err != nil {
		return hLength, err
	}
	// Decode it
	msgLength, bytesParsed := byteArrayToInt64(c.incomingHeaderBuffer)
	if bytesParsed == 0 {
		// "Buffer too small"
		c.Close()
		return hLength, ErrZeroBytesReadHeader
	} else if bytesParsed < 0 {
		// "Buffer overflow"
		c.Close()
		return hLength, ErrLessThanZeroBytesReadHeader
	}

	if msgLength < 0 {
		c.Close()
		return 0, fmt.Errorf("Message length in header is invalid: %d", msgLength)
	}

	// Make sure we don't overrun the slice
	if int(msgLength) > len(b) {
		c.Close()
		return 0, fmt.Errorf("Message is too long: %d bytes", msgLength)
	}

	// Using the header, read the remaining body
	bLength, err := c.lowLevelRead(b[:msgLength])
	if err != nil {
		c.Close()
	}
	return bLength, err
}
