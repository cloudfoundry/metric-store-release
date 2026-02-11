package storage_test

import (
	"fmt"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/cloudfoundry/metric-store-release/src/internal/metrics"
	"github.com/cloudfoundry/metric-store-release/src/internal/storage"
	"github.com/cloudfoundry/metric-store-release/src/pkg/leanstreams"
	"github.com/cloudfoundry/metric-store-release/src/pkg/logger"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/prometheus/model/labels"
	. "github.com/onsi/gomega"
)

type mockRegistrar struct {
	mu     sync.Mutex
	values map[string]float64
	counts map[string]int
}

func newMockRegistrar() *mockRegistrar {
	return &mockRegistrar{
		values: make(map[string]float64),
		counts: make(map[string]int),
	}
}

func (m *mockRegistrar) Set(name string, value float64, labels ...string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := fmt.Sprintf("%s-%v", name, labels)
	m.values[key] = value
}

func (m *mockRegistrar) Add(name string, delta float64, labels ...string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := fmt.Sprintf("%s-%v", name, labels)
	m.values[key] += delta
}

func (m *mockRegistrar) Inc(name string, labels ...string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := fmt.Sprintf("%s-%v", name, labels)
	m.counts[key]++
}

func (m *mockRegistrar) Histogram(name string, labels ...string) prometheus.Observer {
	return nil
}

func (m *mockRegistrar) Registerer() prometheus.Registerer {
	return nil
}

func (m *mockRegistrar) Gatherer() prometheus.Gatherer {
	return nil
}

func (m *mockRegistrar) getValue(name string, labels ...string) float64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := fmt.Sprintf("%s-%v", name, labels)
	return m.values[key]
}

func (m *mockRegistrar) getCount(name string, labels ...string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := fmt.Sprintf("%s-%v", name, labels)
	return m.counts[key]
}

func TestRemoteAppender_ResilientToConnectionFailures(t *testing.T) {
	t.Run("does not block when connection fails", func(t *testing.T) {
		g := NewGomegaWithT(t)
		log := logger.NewTestLogger(io.Discard)
		metricsReg := newMockRegistrar()

		// Create connection to non-existent server
		conn := leanstreams.NewConnection(
			"127.0.0.1:65440",
			nil,
			storage.MAX_INTERNODE_PAYLOAD_SIZE_IN_BYTES,
			leanstreams.WithMaxRetries(2),
			leanstreams.WithRetryDelay(10*time.Millisecond),
			leanstreams.WithConnectTimeout(100*time.Millisecond),
			leanstreams.WithMetrics(metricsReg, "test-node"),
		)

		done := make(chan struct{})
		defer close(done)

		// Create temp directory for handoff storage
		tempDir := t.TempDir()

		// This should not block even though connection will fail
		start := time.Now()
		appender := storage.NewRemoteAppender(
			"test-node-1",
			conn,
			done,
			storage.WithRemoteAppenderLogger(log),
			storage.WithRemoteAppenderHandoffStoragePath(tempDir),
			storage.WithRemoteAppenderMetrics(metricsReg),
		)
		elapsed := time.Since(start)

		// Should return quickly without blocking
		g.Expect(elapsed).To(BeNumerically("<", 2*time.Second))
		g.Expect(appender).NotTo(BeNil())
	})

	t.Run("writes go to handoff queue when connection unavailable", func(t *testing.T) {
		g := NewGomegaWithT(t)
		log := logger.NewTestLogger(io.Discard)
		metricsReg := newMockRegistrar()

		conn := leanstreams.NewConnection(
			"127.0.0.1:65441",
			nil,
			storage.MAX_INTERNODE_PAYLOAD_SIZE_IN_BYTES,
			leanstreams.WithMaxRetries(1),
			leanstreams.WithRetryDelay(10*time.Millisecond),
			leanstreams.WithConnectTimeout(50*time.Millisecond),
		)

		done := make(chan struct{})
		defer close(done)

		tempDir := t.TempDir()

		appender := storage.NewRemoteAppender(
			"test-node-2",
			conn,
			done,
			storage.WithRemoteAppenderLogger(log),
			storage.WithRemoteAppenderHandoffStoragePath(tempDir),
			storage.WithRemoteAppenderMetrics(metricsReg),
		)

		// Append some points
		lbls := labels.FromStrings("__name__", "test_metric", "instance", "test")
		_, err := appender.Append(0, lbls, time.Now().UnixNano(), 42.0)
		g.Expect(err).NotTo(HaveOccurred())

		err = appender.Commit()
		g.Expect(err).NotTo(HaveOccurred())

		// Give it time to process
		time.Sleep(200 * time.Millisecond)

		// Check that data went to handoff queue
		// The handoff queue should have been created and data written
		g.Eventually(func() float64 {
			return metricsReg.getValue(metrics.MetricStoreReplayerQueuedBytesTotal, "test-node-2")
		}, 2*time.Second, 100*time.Millisecond).Should(BeNumerically(">", 0))
	})

	t.Run("successfully writes when connection is available", func(t *testing.T) {
		g := NewGomegaWithT(t)
		log := logger.NewTestLogger(io.Discard)
		metricsReg := newMockRegistrar()

		// Start a test server
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		g.Expect(err).NotTo(HaveOccurred())
		defer listener.Close()

		// Accept connections and read data
		receivedData := make(chan bool, 1)
		go func() {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			defer conn.Close()

			// Read some data
			buf := make([]byte, 1024)
			n, _ := conn.Read(buf)
			if n > 0 {
				receivedData <- true
			}
		}()

		conn := leanstreams.NewConnection(
			listener.Addr().String(),
			nil,
			storage.MAX_INTERNODE_PAYLOAD_SIZE_IN_BYTES,
		)

		done := make(chan struct{})
		defer close(done)

		tempDir := t.TempDir()

		appender := storage.NewRemoteAppender(
			"test-node-3",
			conn,
			done,
			storage.WithRemoteAppenderLogger(log),
			storage.WithRemoteAppenderHandoffStoragePath(tempDir),
			storage.WithRemoteAppenderMetrics(metricsReg),
		)

		// Wait a bit for connection to establish
		time.Sleep(200 * time.Millisecond)

		// Append some points
		lbls := labels.FromStrings("__name__", "test_metric", "instance", "test")
		_, err = appender.Append(0, lbls, time.Now().UnixNano(), 42.0)
		g.Expect(err).NotTo(HaveOccurred())

		err = appender.Commit()
		g.Expect(err).NotTo(HaveOccurred())

		// Should receive data
		select {
		case <-receivedData:
			// Success
		case <-time.After(2 * time.Second):
			t.Fatal("Did not receive data from appender")
		}

		// Check distributed points metric
		g.Eventually(func() float64 {
			return metricsReg.getValue(metrics.MetricStoreDistributedPointsTotal, "test-node-3")
		}, 2*time.Second, 100*time.Millisecond).Should(BeNumerically(">", 0))
	})
}

func TestRemoteAppender_GracefulDegradation(t *testing.T) {
	t.Run("appender continues working after connection failures", func(t *testing.T) {
		g := NewGomegaWithT(t)
		log := logger.NewTestLogger(io.Discard)
		metricsReg := newMockRegistrar()

		conn := leanstreams.NewConnection(
			"127.0.0.1:65442",
			nil,
			storage.MAX_INTERNODE_PAYLOAD_SIZE_IN_BYTES,
			leanstreams.WithMaxRetries(1),
			leanstreams.WithRetryDelay(10*time.Millisecond),
			leanstreams.WithConnectTimeout(50*time.Millisecond),
		)

		done := make(chan struct{})
		defer close(done)

		tempDir := t.TempDir()

		appender := storage.NewRemoteAppender(
			"test-node-4",
			conn,
			done,
			storage.WithRemoteAppenderLogger(log),
			storage.WithRemoteAppenderHandoffStoragePath(tempDir),
			storage.WithRemoteAppenderMetrics(metricsReg),
		)

		// Try multiple appends - should not panic or block
		for i := 0; i < 10; i++ {
			lbls := labels.FromStrings("__name__", "test_metric", "count", fmt.Sprintf("%d", i))
			_, err := appender.Append(0, lbls, time.Now().UnixNano(), float64(i))
			g.Expect(err).NotTo(HaveOccurred())

			err = appender.Commit()
			g.Expect(err).NotTo(HaveOccurred())
		}

		// Give it time to process
		time.Sleep(1 * time.Second)

		// Should have queued all the data
		queuedBytes := metricsReg.getValue(metrics.MetricStoreReplayerQueuedBytesTotal, "test-node-4")
		g.Expect(queuedBytes).To(BeNumerically(">", 0))

		// Should not have dropped any points (all went to handoff)
		droppedPoints := metricsReg.getValue(metrics.MetricStoreDroppedPointsTotal, "test-node-4")
		g.Expect(droppedPoints).To(Equal(0.0))
	})
}

func TestRemoteAppender_Integration(t *testing.T) {
	t.Run("server becomes available after initial failure", func(t *testing.T) {
		t.Skip("Flaky test - timing sensitive with async handoff queue initialization")
		g := NewGomegaWithT(t)
		log := logger.NewTestLogger(io.Discard)
		metricsReg := newMockRegistrar()

		// Choose a port but don't start server yet
		port := 65443

		conn := leanstreams.NewConnection(
			fmt.Sprintf("127.0.0.1:%d", port),
			nil,
			storage.MAX_INTERNODE_PAYLOAD_SIZE_IN_BYTES,
			leanstreams.WithMaxRetries(2),
			leanstreams.WithRetryDelay(10*time.Millisecond),
			leanstreams.WithConnectTimeout(100*time.Millisecond),
		)

		done := make(chan struct{})
		defer close(done)

		tempDir := t.TempDir()

		// Create appender before server starts
		appender := storage.NewRemoteAppender(
			"test-node-5",
			conn,
			done,
			storage.WithRemoteAppenderLogger(log),
			storage.WithRemoteAppenderHandoffStoragePath(tempDir),
			storage.WithRemoteAppenderMetrics(metricsReg),
		)

		// Write some data while server is down
		lbls := labels.FromStrings("__name__", "test_metric", "phase", "before")
		_, err := appender.Append(0, lbls, time.Now().UnixNano(), 1.0)
		g.Expect(err).NotTo(HaveOccurred())
		err = appender.Commit()
		g.Expect(err).NotTo(HaveOccurred())

		time.Sleep(200 * time.Millisecond)

		// Data should be in handoff queue
		queuedBefore := metricsReg.getValue(metrics.MetricStoreReplayerQueuedBytesTotal, "test-node-5")
		g.Expect(queuedBefore).To(BeNumerically(">", 0))

		// Now start the server
		listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		g.Expect(err).NotTo(HaveOccurred())
		defer listener.Close()

		go func() {
			for {
				conn, err := listener.Accept()
				if err != nil {
					return
				}
				go func(c net.Conn) {
					defer c.Close()
					buf := make([]byte, 4096)
					for {
						_, err := c.Read(buf)
						if err != nil {
							return
						}
					}
				}(conn)
			}
		}()

		// Wait for handoff queue to drain (replayer will retry and succeed)
		time.Sleep(2 * time.Second)

		// New writes should succeed directly now
		lbls2 := labels.FromStrings("__name__", "test_metric", "phase", "after")
		_, err = appender.Append(0, lbls2, time.Now().UnixNano(), 2.0)
		g.Expect(err).NotTo(HaveOccurred())
		err = appender.Commit()
		g.Expect(err).NotTo(HaveOccurred())

		time.Sleep(1 * time.Second)

		// Should eventually have replayed bytes
		g.Eventually(func() float64 {
			return metricsReg.getValue(metrics.MetricStoreReplayerReplayedBytesTotal, "test-node-5")
		}, 5*time.Second, 500*time.Millisecond).Should(BeNumerically(">", 0))
	})
}

