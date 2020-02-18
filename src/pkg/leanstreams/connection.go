package leanstreams

import (
	"crypto/tls"
	"sync"
	"time"
)

type Connection struct {
	clientConfig *TCPClientConfig
	client       *TCPClient

	done chan struct{}
	sync.Mutex
}

func NewConnection(addr string, tlsConfig *tls.Config, maxPayloadSizeInBytes int) *Connection {
	clientConfig := &TCPClientConfig{
		MaxMessageSize: maxPayloadSizeInBytes,
		Address:        addr,
		TLSConfig:      tlsConfig,
	}

	return &Connection{
		clientConfig: clientConfig,
		client:       nil,
		done:         make(chan struct{}),
	}
}

func (c *Connection) Connect() {
	select {
	case <-c.done:
		return
	default:
	}

	c.Lock()
	defer c.Unlock()

	var err error

	for {
		c.client, err = DialTCP(c.clientConfig)
		if err != nil {
			// waiting for remote node to start listening

			// TODO: do this better
			time.Sleep(100 * time.Millisecond)

			continue
		}

		break
	}
}

func (c *Connection) Client() *TCPClient {
	if c.client == nil {
		c.Connect()
	}

	return c.client
}

func (c *Connection) Close() error {
	close(c.done)
	c.Lock()
	defer c.Unlock()

	if c.client != nil {
		return c.client.Close()
	}

	return nil
}
