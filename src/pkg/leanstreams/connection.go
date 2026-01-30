package leanstreams

import (
	"crypto/tls"
	"fmt"
	"sync"
	"time"
)

const (
	DefaultMaxRetries     = 10
	DefaultRetryDelay     = 1 * time.Second
	DefaultConnectTimeout = 30 * time.Second
)

type MetricsRecorder interface {
	Inc(name string, labels ...string)
	Set(name string, value float64, labels ...string)
}

type Connection struct {
	clientConfig *TCPClientConfig
	client       *TCPClient

	maxRetries     int
	retryDelay     time.Duration
	connectTimeout time.Duration
	metrics        MetricsRecorder
	nodeLabel      string

	done chan struct{}
	sync.Mutex
}

type ConnectionOption func(*Connection)

// WithMaxRetries sets the maximum number of connection attempts before giving up
func WithMaxRetries(maxRetries int) ConnectionOption {
	return func(c *Connection) {
		c.maxRetries = maxRetries
	}
}

// WithRetryDelay sets the delay between connection retry attempts
func WithRetryDelay(delay time.Duration) ConnectionOption {
	return func(c *Connection) {
		c.retryDelay = delay
	}
}

// WithConnectTimeout sets the total timeout for establishing a connection
func WithConnectTimeout(timeout time.Duration) ConnectionOption {
	return func(c *Connection) {
		c.connectTimeout = timeout
	}
}

// WithMetrics sets the metrics recorder for tracking connection state
func WithMetrics(metrics MetricsRecorder, nodeLabel string) ConnectionOption {
	return func(c *Connection) {
		c.metrics = metrics
		c.nodeLabel = nodeLabel
	}
}

func NewConnection(addr string, tlsConfig *tls.Config, maxPayloadSizeInBytes int, opts ...ConnectionOption) *Connection {
	clientConfig := &TCPClientConfig{
		MaxMessageSize: maxPayloadSizeInBytes,
		Address:        addr,
		TLSConfig:      tlsConfig,
	}

	conn := &Connection{
		clientConfig:   clientConfig,
		client:         nil,
		maxRetries:     DefaultMaxRetries,
		retryDelay:     DefaultRetryDelay,
		connectTimeout: DefaultConnectTimeout,
		done:           make(chan struct{}),
	}

	for _, opt := range opts {
		opt(conn)
	}

	return conn
}

func (c *Connection) Connect() error {
	select {
	case <-c.done:
		return fmt.Errorf("connection closed")
	default:
	}

	c.Lock()
	defer c.Unlock()

	// If already connected, return success
	if c.client != nil {
		return nil
	}

	var err error
	var lastErr error
	startTime := time.Now()
	attempts := 0

	for attempts < c.maxRetries {
		// Track connection attempt
		if c.metrics != nil && c.nodeLabel != "" {
			c.metrics.Inc("metric_store_internode_connection_attempts_total", c.nodeLabel)
		}

		// Check if we've exceeded the total timeout
		if time.Since(startTime) >= c.connectTimeout {
			if c.metrics != nil && c.nodeLabel != "" {
				c.metrics.Inc("metric_store_internode_connection_failures_total", c.nodeLabel)
				c.metrics.Set("metric_store_internode_connection_state", 0, c.nodeLabel) // 0 = disconnected
			}
			return fmt.Errorf("connection timeout after %v (address: %s, attempts: %d, last error: %w)",
				c.connectTimeout, c.clientConfig.Address, attempts, lastErr)
		}

		c.client, err = DialTCP(c.clientConfig)
		if err == nil {
			// Connection successful
			if c.metrics != nil && c.nodeLabel != "" {
				c.metrics.Inc("metric_store_internode_connection_successes_total", c.nodeLabel)
				c.metrics.Set("metric_store_internode_connection_state", 1, c.nodeLabel) // 1 = connected
			}
			return nil
		}

		lastErr = err
		attempts++

		// Don't sleep on the last attempt
		if attempts < c.maxRetries {
			// Calculate exponential backoff delay: retryDelay * (2 ^ attempts)
			// For attempts 0,1,2,3,4 with 1s base: 1s, 2s, 4s, 8s, 16s
			backoffDelay := c.retryDelay * (1 << uint(attempts))

			// Cap the backoff to avoid excessive delays
			maxBackoff := 30 * time.Second
			if backoffDelay > maxBackoff {
				backoffDelay = maxBackoff
			}

			// Check if we should continue or timeout
			select {
			case <-c.done:
				if c.metrics != nil && c.nodeLabel != "" {
					c.metrics.Inc("metric_store_internode_connection_failures_total", c.nodeLabel)
					c.metrics.Set("metric_store_internode_connection_state", 0, c.nodeLabel) // 0 = disconnected
				}
				return fmt.Errorf("connection closed during retry")
			case <-time.After(backoffDelay):
				// Continue to next attempt
			}
		}
	}

	if c.metrics != nil && c.nodeLabel != "" {
		c.metrics.Inc("metric_store_internode_connection_failures_total", c.nodeLabel)
		c.metrics.Set("metric_store_internode_connection_state", 0, c.nodeLabel) // 0 = disconnected
	}

	return fmt.Errorf("failed to connect after %d attempts (address: %s, last error: %w)",
		attempts, c.clientConfig.Address, lastErr)
}

func (c *Connection) Client() (*TCPClient, error) {
	if c.client == nil {
		err := c.Connect()
		if err != nil {
			return nil, err
		}
	}

	return c.client, nil
}

func (c *Connection) IsConnected() bool {
	c.Lock()
	defer c.Unlock()
	return c.client != nil
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
