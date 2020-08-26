package handoff

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/cloudfoundry/metric-store-release/src/internal/metrics"
	"github.com/cloudfoundry/metric-store-release/src/internal/ticker"
	"github.com/cloudfoundry/metric-store-release/src/pkg/logger"
	"github.com/cloudfoundry/metric-store-release/src/pkg/rpc"
)

const (
	GiB = 1 << 30

	// DefaultMaxAge is the default maximum amount of time that a hinted handoff write
	// can stay in the queue.  After this time, the write will be purged.
	DefaultMaxAge = 7 * 24 * time.Hour

	// DefaultRetryRateLimit is the default rate that hinted handoffs will be retried.
	// The rate is in bytes per second.   A value of 0 disables the rate limit.
	DefaultRetryRateLimit = 0

	// DefaultRetryInterval is the default amount of time the system waits before
	// attempting to flush hinted handoff queues. With each failure of a hinted
	// handoff write, this retry interval increases exponentially until it reaches
	// the maximum
	DefaultRetryInterval = 10 * time.Millisecond

	// DefaultRetryMaxInterval is the maximum the hinted handoff retry interval
	// will ever be.
	DefaultRetryMaxInterval = 10 * time.Second

	// DefaultPurgeInterval is the amount of time the system waits before attempting
	// to purge hinted handoff data due to age or inactive nodes.
	DefaultPurgeInterval = time.Hour
)

// WriteReplayer encapsulates a queue of hinted-handoff data for a node, and the
// transmission of the data to the node.
type WriteReplayer struct {
	PurgeInterval    time.Duration // Interval between periodic purge checks
	RetryInterval    time.Duration // Interval between periodic write-to-node attempts.
	RetryMaxInterval time.Duration // Max interval between periodic write-to-node attempts.
	MaxAge           time.Duration // Maximum age queue data can get before purging.
	RetryRateLimit   int64         // Limits the rate data is sent to node.
	dir              string
	currentQueueSize uint64 // Stores the number of currently queued points (not including any points reloaded from disk)
	targetNodeIndex  string

	mu   sync.RWMutex
	done chan struct{}

	queue  Queue
	client tcpClient

	log     *logger.Logger
	metrics metrics.Registrar
}

type tcpClient interface {
	Write(data []byte) (int, error)
}

type Queue interface {
	Advance() error
	Append([]byte) error
	Close() error
	Current() ([]byte, error)
	DiskUsage() int64
	Open() error
	PurgeOlderThan(time.Time) error
	SetMaxSegmentSize(size int64) error
}

// NewWriteReplayer returns a new WriteReplayer for the given node, using dir for
// the hinted-handoff data.
func NewWriteReplayer(queue Queue, c tcpClient, metrics metrics.Registrar, targetNodeIndex string, opts ...WriteReplayerOption) *WriteReplayer {
	w := &WriteReplayer{
		PurgeInterval:    DefaultPurgeInterval,
		RetryInterval:    DefaultRetryInterval,
		RetryMaxInterval: DefaultRetryMaxInterval,
		MaxAge:           DefaultMaxAge,
		queue:            queue,
		client:           c,
		targetNodeIndex:  targetNodeIndex,

		log: logger.NewNop(),

		metrics: metrics,
	}

	for _, opt := range opts {
		opt(w)
	}

	return w
}

type WriteReplayerOption func(*WriteReplayer)

func WithWriteReplayerLogger(log *logger.Logger) WriteReplayerOption {
	return func(w *WriteReplayer) {
		w.log = log
	}
}

// Open opens the WriteReplayer. It will read and write data present in dir, and
// start transmitting data to the node. A WriteReplayer must be opened before it
// can accept hinted data.
func (w *WriteReplayer) Open(done chan struct{}) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.done != nil {
		// Already open.
		return nil
	}
	w.done = done

	if err := w.queue.Open(); err != nil {
		return err
	}

	go w.run()

	return nil
}

// close closes the WriteReplayer, terminating all data tranmission to the node.
// When closed it will not accept hinted-handoff data.
func (w *WriteReplayer) close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	return w.queue.Close()
}

func (w *WriteReplayer) Write(points []*rpc.Point) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.done == nil {
		return fmt.Errorf("write replayer is closed")
	}

	var payload bytes.Buffer
	enc := gob.NewEncoder(&payload)
	err := enc.Encode(rpc.Batch{Points: points})
	if err != nil {
		w.log.Error("gob encode error", err)
		return err
	}

	err = w.queue.Append(payload.Bytes())

	if err != nil {
		w.metrics.Inc(metrics.MetricStoreReplayerQueueErrorsTotal, w.targetNodeIndex)
	} else {
		w.metrics.Set(metrics.MetricStoreReplayerDiskUsageBytes, float64(w.queue.DiskUsage()), w.targetNodeIndex)
		w.metrics.Add(metrics.MetricStoreReplayerQueuedBytesTotal, float64(len(payload.Bytes())), w.targetNodeIndex)
	}

	return err
}

// run attempts to send any existing hinted handoff data to the target node. It also purges
// any hinted handoff data older than the configured time.
func (w *WriteReplayer) run() {
	defer w.close()

	purgeTicker := time.NewTicker(w.PurgeInterval)
	defer purgeTicker.Stop()

	tickerConfig := &ticker.Config{
		BaseDelay:  w.RetryInterval,
		Multiplier: 2,
		MaxDelay:   w.RetryMaxInterval,
	}
	delay := ticker.NewExponentialDelay(tickerConfig)
	writeTicker := ticker.New(delay)
	defer writeTicker.Stop()

	for {
		select {
		case <-w.done:
			return

		case <-purgeTicker.C:
			if err := w.queue.PurgeOlderThan(time.Now().Add(-w.MaxAge)); err != nil {
				w.log.Error("failed to purge", err)
			}

		case <-writeTicker.C:
			_, err := w.SendWrite()
			if err == nil {
				writeTicker.Reset()
			}
		}
	}
}

// SendWrite attempts to sent the current block of hinted data to the target
// node. If successful, it returns the number of bytes it sent and advances to
// the next block. Otherwise returns EOF when there is no more data or the
// node is inactive.
func (w *WriteReplayer) SendWrite() (int, error) {
	w.mu.RLock()
	defer w.mu.RUnlock()

	// Get the current block from the queue
	payload, err := w.queue.Current()
	if err != nil {
		if err != io.EOF {
			w.log.Error("error getting current queue", err)

			// TODO - this metric doesn't seem to mean much, it looks like it's
			//   the number of times we've bounced off the bottom of an empty queue
			w.metrics.Inc(metrics.MetricStoreReplayerReadErrorsTotal, w.targetNodeIndex)
		}
		return 0, err
	}

	bytesWritten, err := w.client.Write(payload)

	if err != nil {
		w.log.Error("error replaying", err)

		w.metrics.Inc(metrics.MetricStoreReplayerReplayErrorsTotal, w.targetNodeIndex)
		return 0, err
	}

	w.log.Debug("replayed bytes", logger.Count(bytesWritten))
	w.metrics.Add(metrics.MetricStoreReplayerReplayedBytesTotal, float64(bytesWritten), w.targetNodeIndex)

	if err := w.queue.Advance(); err != nil {
		w.log.Error("failed to advance queue", err)
	}

	return len(payload), nil
}
