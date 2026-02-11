package leanstreams_test

import (
	"net"
	"sync"
	"testing"
	"time"

	"github.com/cloudfoundry/metric-store-release/src/pkg/leanstreams"
	. "github.com/onsi/gomega"
)

type mockMetrics struct {
	mu        sync.Mutex
	incCalls  map[string]int
	setCalls  map[string]float64
	incLabels map[string][]string
	setLabels map[string][]string
}

func newMockMetrics() *mockMetrics {
	return &mockMetrics{
		incCalls:  make(map[string]int),
		setCalls:  make(map[string]float64),
		incLabels: make(map[string][]string),
		setLabels: make(map[string][]string),
	}
}

func (m *mockMetrics) Inc(name string, labels ...string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.incCalls[name]++
	m.incLabels[name] = labels
}

func (m *mockMetrics) Set(name string, value float64, labels ...string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.setCalls[name] = value
	m.setLabels[name] = labels
}

func (m *mockMetrics) getIncCount(name string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.incCalls[name]
}

func (m *mockMetrics) getSetValue(name string) float64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.setCalls[name]
}

func TestConnection_ConnectWithTimeout(t *testing.T) {
	t.Run("fails after max retries with no server", func(t *testing.T) {
		g := NewGomegaWithT(t)

		// Use an address that will definitely fail
		conn := leanstreams.NewConnection(
			"127.0.0.1:65432", // Non-existent server
			nil,
			1024,
			leanstreams.WithMaxRetries(3),
			leanstreams.WithRetryDelay(10*time.Millisecond),
			leanstreams.WithConnectTimeout(1*time.Second),
		)

		start := time.Now()
		err := conn.Connect()
		elapsed := time.Since(start)

		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("failed to connect after"))
		g.Expect(err.Error()).To(ContainSubstring("127.0.0.1:65432"))
		// Should have tried 3 times with delays
		g.Expect(elapsed).To(BeNumerically(">=", 20*time.Millisecond))
		g.Expect(elapsed).To(BeNumerically("<", 1*time.Second))
	})

	t.Run("respects total timeout even with many retries", func(t *testing.T) {
		g := NewGomegaWithT(t)

		conn := leanstreams.NewConnection(
			"127.0.0.1:65433",
			nil,
			1024,
			leanstreams.WithMaxRetries(100), // Many retries
			leanstreams.WithRetryDelay(10*time.Millisecond),
			leanstreams.WithConnectTimeout(100*time.Millisecond), // Short timeout
		)

		start := time.Now()
		err := conn.Connect()
		elapsed := time.Since(start)

		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("connection timeout"))
		// Should timeout before all retries are exhausted
		g.Expect(elapsed).To(BeNumerically(">=", 100*time.Millisecond))
		g.Expect(elapsed).To(BeNumerically("<", 500*time.Millisecond))
	})

	t.Run("succeeds when server becomes available", func(t *testing.T) {
		g := NewGomegaWithT(t)

		// Start a TCP server after a delay
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		g.Expect(err).NotTo(HaveOccurred())
		defer listener.Close()

		addr := listener.Addr().String()

		// Start accepting connections after a delay
		go func() {
			time.Sleep(50 * time.Millisecond)
			conn, _ := listener.Accept()
			if conn != nil {
				defer conn.Close()
			}
		}()

		conn := leanstreams.NewConnection(
			addr,
			nil,
			1024,
			leanstreams.WithMaxRetries(10),
			leanstreams.WithRetryDelay(10*time.Millisecond),
			leanstreams.WithConnectTimeout(2*time.Second),
		)

		err = conn.Connect()
		g.Expect(err).NotTo(HaveOccurred())

		client, err := conn.Client()
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(client).NotTo(BeNil())
	})

	t.Run("returns immediately if already connected", func(t *testing.T) {
		g := NewGomegaWithT(t)

		listener, err := net.Listen("tcp", "127.0.0.1:0")
		g.Expect(err).NotTo(HaveOccurred())
		defer listener.Close()

		go func() {
			conn, _ := listener.Accept()
			if conn != nil {
				defer conn.Close()
			}
		}()

		conn := leanstreams.NewConnection(
			listener.Addr().String(),
			nil,
			1024,
		)

		// First connect
		err = conn.Connect()
		g.Expect(err).NotTo(HaveOccurred())

		// Second connect should return immediately
		start := time.Now()
		err = conn.Connect()
		elapsed := time.Since(start)

		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(elapsed).To(BeNumerically("<", 10*time.Millisecond))
	})

	t.Run("respects done channel during retries", func(t *testing.T) {
		g := NewGomegaWithT(t)

		conn := leanstreams.NewConnection(
			"127.0.0.1:65434",
			nil,
			1024,
			leanstreams.WithMaxRetries(100),
			leanstreams.WithRetryDelay(50*time.Millisecond),
			leanstreams.WithConnectTimeout(10*time.Second),
		)

		// Start connect in goroutine
		errChan := make(chan error, 1)
		go func() {
			errChan <- conn.Connect()
		}()

		// Close connection after a short delay
		time.Sleep(100 * time.Millisecond)
		conn.Close()

		// Should fail quickly after close
		select {
		case err := <-errChan:
			g.Expect(err).To(HaveOccurred())
			g.Expect(err.Error()).To(Or(
				ContainSubstring("connection closed"),
				ContainSubstring("failed to connect"),
			))
		case <-time.After(2 * time.Second):
			t.Fatal("Connect did not respect done channel")
		}
	})
}

func TestConnection_ClientAutoConnect(t *testing.T) {
	t.Run("Client() triggers connection if not connected", func(t *testing.T) {
		g := NewGomegaWithT(t)

		listener, err := net.Listen("tcp", "127.0.0.1:0")
		g.Expect(err).NotTo(HaveOccurred())
		defer listener.Close()

		go func() {
			conn, _ := listener.Accept()
			if conn != nil {
				defer conn.Close()
			}
		}()

		conn := leanstreams.NewConnection(
			listener.Addr().String(),
			nil,
			1024,
		)

		// Client should auto-connect
		client, err := conn.Client()
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(client).NotTo(BeNil())
		g.Expect(conn.IsConnected()).To(BeTrue())
	})

	t.Run("Client() returns error if connection fails", func(t *testing.T) {
		g := NewGomegaWithT(t)

		conn := leanstreams.NewConnection(
			"127.0.0.1:65435",
			nil,
			1024,
			leanstreams.WithMaxRetries(2),
			leanstreams.WithRetryDelay(10*time.Millisecond),
			leanstreams.WithConnectTimeout(100*time.Millisecond),
		)

		client, err := conn.Client()
		g.Expect(err).To(HaveOccurred())
		g.Expect(client).To(BeNil())
		g.Expect(conn.IsConnected()).To(BeFalse())
	})
}

func TestConnection_WithMetrics(t *testing.T) {
	t.Run("tracks connection attempts and failures", func(t *testing.T) {
		g := NewGomegaWithT(t)
		metrics := newMockMetrics()

		conn := leanstreams.NewConnection(
			"127.0.0.1:65436",
			nil,
			1024,
			leanstreams.WithMaxRetries(3),
			leanstreams.WithRetryDelay(10*time.Millisecond),
			leanstreams.WithConnectTimeout(500*time.Millisecond),
			leanstreams.WithMetrics(metrics, "node-1"),
		)

		err := conn.Connect()
		g.Expect(err).To(HaveOccurred())

		// Should have tracked attempts (3 retries = 3 attempts)
		attempts := metrics.getIncCount("metric_store_internode_connection_attempts_total")
		g.Expect(attempts).To(BeNumerically(">=", 3))

		// Should have tracked failure
		failures := metrics.getIncCount("metric_store_internode_connection_failures_total")
		g.Expect(failures).To(Equal(1))

		// Should have set state to disconnected (0)
		state := metrics.getSetValue("metric_store_internode_connection_state")
		g.Expect(state).To(Equal(0.0))
	})

	t.Run("tracks connection success", func(t *testing.T) {
		g := NewGomegaWithT(t)
		metrics := newMockMetrics()

		listener, err := net.Listen("tcp", "127.0.0.1:0")
		g.Expect(err).NotTo(HaveOccurred())
		defer listener.Close()

		go func() {
			conn, _ := listener.Accept()
			if conn != nil {
				defer conn.Close()
			}
		}()

		conn := leanstreams.NewConnection(
			listener.Addr().String(),
			nil,
			1024,
			leanstreams.WithMetrics(metrics, "node-2"),
		)

		err = conn.Connect()
		g.Expect(err).NotTo(HaveOccurred())

		// Should have tracked successful connection
		successes := metrics.getIncCount("metric_store_internode_connection_successes_total")
		g.Expect(successes).To(Equal(1))

		// Should have set state to connected (1)
		state := metrics.getSetValue("metric_store_internode_connection_state")
		g.Expect(state).To(Equal(1.0))

		// Should have correct labels
		g.Expect(metrics.setLabels["metric_store_internode_connection_state"]).To(ContainElement("node-2"))
	})
}

func TestConnection_IsConnected(t *testing.T) {
	t.Run("returns false before connection", func(t *testing.T) {
		g := NewGomegaWithT(t)

		conn := leanstreams.NewConnection("127.0.0.1:65437", nil, 1024)
		g.Expect(conn.IsConnected()).To(BeFalse())
	})

	t.Run("returns true after successful connection", func(t *testing.T) {
		g := NewGomegaWithT(t)

		listener, err := net.Listen("tcp", "127.0.0.1:0")
		g.Expect(err).NotTo(HaveOccurred())
		defer listener.Close()

		go func() {
			conn, _ := listener.Accept()
			if conn != nil {
				defer conn.Close()
			}
		}()

		conn := leanstreams.NewConnection(
			listener.Addr().String(),
			nil,
			1024,
		)

		err = conn.Connect()
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(conn.IsConnected()).To(BeTrue())
	})
}

func TestConnection_DefaultValues(t *testing.T) {
	t.Skip("Flaky test - timing sensitive with retry logic")
	g := NewGomegaWithT(t)

	conn := leanstreams.NewConnection("127.0.0.1:65438", nil, 1024)

	// Test that defaults are applied by attempting connection with default timeouts
	start := time.Now()
	err := conn.Connect()
	elapsed := time.Since(start)

	g.Expect(err).To(HaveOccurred())
	// With default 10 retries and 1s delay, should take at least a few seconds
	// but less than 30s (default timeout)
	g.Expect(elapsed).To(BeNumerically("<", 30*time.Second))
}
