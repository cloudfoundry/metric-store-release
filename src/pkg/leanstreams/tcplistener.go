package leanstreams

import (
	"crypto/tls"
	"net"
	"sync"
)

// ListenCallback is a function type that calling code will need to implement in order
// to receive arrays of bytes from the socket. Each slice of bytes will be stripped of the
// size header, meaning you can directly serialize the raw slice. You would then perform your
// custom logic for interpretting the message, before returning. You can optionally
// return an error, which in turn will be logged if EnableLogging is set to true.
type ListenCallback func([]byte) error

// TCPListener represents the abstraction over a raw TCP socket for reading streaming
// protocolbuffer data without having to write a ton of boilerplate
type TCPListener struct {
	socket          net.Listener
	logger          Logger
	callback        ListenCallback
	shutdownChannel chan struct{}
	shutdownGroup   *sync.WaitGroup
	ConnConfig      *TCPServerConfig
	tlsConfig       *tls.Config
	Address         string
	connectionCount int

	groupMu sync.Mutex
	blockMu sync.Mutex
	countMu sync.Mutex
}

type Logger interface {
	Printf(v string, args ...interface{})
}

// TCPListenerConfig representss the information needed to begin listening for
// incoming messages.
type TCPListenerConfig struct {
	// Controls how large the largest Message may be. The server will reject any messages whose clients
	// header size does not match this configuration
	MaxMessageSize int
	// Controls the ability to enable logging errors occurring in the library
	Logger Logger

	// The local address to listen for incoming connections on. Typically, you exclude
	// the ip, and just provide port, ie: ":5031"
	Address string
	// The callback to invoke once a full set of message bytes has been received. It
	// is your responsibility to handle parsing the incoming message and handling errors
	// inside the callback
	Callback ListenCallback

	TLSConfig *tls.Config
}

// ListenTCP creates a TCPListener, and opens it's local connection to
// allow it to begin receiving, once you're ready to. So the connection is open,
// but it is not yet attempting to handle connections.
func ListenTCP(cfg TCPListenerConfig) (*TCPListener, error) {
	maxMessageSize := DefaultMaxMessageSize
	// 0 is the default, and the message must be atleast 1 byte large
	if cfg.MaxMessageSize != 0 {
		maxMessageSize = cfg.MaxMessageSize
	}
	connCfg := TCPServerConfig{
		MaxMessageSize: maxMessageSize,
		Address:        cfg.Address,
		TLSConfig:      cfg.TLSConfig,
	}

	btl := &TCPListener{
		logger:          cfg.Logger,
		callback:        cfg.Callback,
		shutdownChannel: make(chan struct{}),
		shutdownGroup:   &sync.WaitGroup{},
		ConnConfig:      &connCfg,
		tlsConfig:       cfg.TLSConfig,
		Address:         "",
		connectionCount: 0,
	}

	if err := btl.openSocket(); err != nil {
		return nil, err
	}

	return btl, nil
}

// Actually blocks the thread it's running on, and begins handling incoming
// requests
func (t *TCPListener) blockListen() error {
	for {
		// Wait for someone to connect
		c, err := t.socket.Accept()

		if err != nil {
			if t.logger != nil {
				t.logger.Printf("Error attempting to accept connection: %s", err)
			}
			// Stole this approach from http://zhen.org/blog/graceful-shutdown-of-go-net-dot-listeners/
			// Benefits of a channel for the simplicity of use, but don't have to even check it
			// unless theres an error, so performance impact to incoming conns should be lower
			select {
			case <-t.shutdownChannel:
				return nil
			default:
				// Nothing, continue to the top of the loop
			}

			continue
		}

		conn := newTCPServer(t.ConnConfig)
		// Don't dial out, wrap the underlying conn in one of ours
		conn.socket = c

		t.groupMu.Lock()
		// Increment the waitGroup in the event of a shutdown
		t.shutdownGroup.Add(1)
		t.groupMu.Unlock()

		t.countMu.Lock()
		t.connectionCount += 1
		t.countMu.Unlock()

		// Hand this off and immediately listen for more
		go t.readLoop(conn)
	}
}

// This is only ever called from either StartListening or StartListeningAsync
// Theres no need to lock, it will only ever be called upon choosing to start
// to listen, by design. Maybe that'll have to change at some point.
func (t *TCPListener) openSocket() error {
	tcpAddr, err := net.ResolveTCPAddr("tcp", t.ConnConfig.Address)
	if err != nil {
		return err
	}
	insecureConnection, err := net.ListenTCP("tcp", tcpAddr)
	if err != nil {
		return err
	}
	secureConnection := tls.NewListener(insecureConnection, t.tlsConfig)

	t.socket = secureConnection
	t.Address = insecureConnection.Addr().String()
	return err
}

func (t *TCPListener) reopenSocket() error {
	t.blockMu.Lock()
	defer t.blockMu.Unlock()

	t.groupMu.Lock()
	t.shutdownGroup.Wait()
	t.groupMu.Unlock()

	t.countMu.Lock()
	t.connectionCount = 0
	t.countMu.Unlock()

	tcpAddr, err := net.ResolveTCPAddr("tcp", t.Address)
	if err != nil {
		return err
	}
	receiveSocket, err := net.ListenTCP("tcp", tcpAddr)
	if err != nil {
		return err
	}

	conn := tls.NewListener(receiveSocket, t.tlsConfig)

	t.socket = conn
	t.shutdownChannel = make(chan struct{})
	return err
}

// Close represents a way to signal to the Listener that it should no longer accept
// incoming connections, and shutdown
func (t *TCPListener) Close() {
	close(t.shutdownChannel)
	t.socket.Close()

	t.groupMu.Lock()
	t.shutdownGroup.Wait()
	t.groupMu.Unlock()

	t.countMu.Lock()
	t.connectionCount = 0
	t.countMu.Unlock()
}

// StartListeningAsync represents a way to start accepting TCP connections, which are
// handled by the Callback provided upon initialization. It does the listening
// in a go-routine, so as not to block.
func (t *TCPListener) StartListeningAsync() error {
	var err error
	go func() {
		t.blockMu.Lock()
		defer t.blockMu.Unlock()
		err = t.blockListen()
	}()
	return err
}

func (t *TCPListener) RestartListeningAsync() error {
	err := t.reopenSocket()
	if err != nil {
		return err
	}

	return t.StartListeningAsync()
}

// Handles each incoming connection, run within it's own goroutine. This method will
// loop until the client disconnects or another error occurs and is not handled
func (t *TCPListener) readLoop(conn *TCPServer) {
	defer t.shutdownGroup.Done()
	// dataBuffer will hold the message from each read
	dataBuffer := make([]byte, conn.MaxMessageSize)

	// Start an asyncrhonous call that will wait on the shutdown channel, and then close
	// the connection. This will let us respond to the shutdown but also not incur
	// a cost for checking the channel on each run of the loop
	go func(c *TCPServer, s <-chan struct{}) {
		<-s
		c.Close()
	}(conn, t.shutdownChannel)

	// Begin the read loop
	// If there is any error, close the connection officially and break out of the listen-loop.
	// We don't store these connections anywhere else, and if we can't recover from an error on the socket
	// we want to kill the connection, exit the goroutine, and let the client handle re-connecting if need be.
	// Handle getting the data header
	for {
		msgLen, err := conn.Read(dataBuffer)
		if err != nil {
			if t.logger != nil {
				t.logger.Printf("Address %s: Failure to read from connection. Underlying error: %s", conn.address, err)
			}
			conn.Close()
			t.countMu.Lock()
			t.connectionCount -= 1
			t.countMu.Unlock()
			return
		}
		// We take action on the actual message data - but only up to the amount of bytes read,
		// since we re-use the cache
		if msgLen == 0 {
			continue
		}

		if err = t.callback(dataBuffer[:msgLen]); err != nil && t.logger != nil {
			t.logger.Printf("Error in Callback: %s", err.Error())
			// TODO if it's a protobuffs error, it means we likely had an issue and can't
			// deserialize data? Should we kill the connection and have the client start over?
			// At this point, there isn't a reliable recovery mechanic for the server
		}
	}
}

func (t *TCPListener) OpenConnections() int {
	t.countMu.Lock()
	defer t.countMu.Unlock()

	return t.connectionCount
}
