package storage

// TODO
// - Close()
//   - passing in connections, not config

import (
	// You will need to make sure this import exists for side effects:
	// _ "github.com/influxdata/influxdb/tsdb/engine"
	// the go linter in some instances removes it

	"bytes"
	"crypto/tls"
	"encoding/gob"
	"fmt"
	"os"
	"sync"
	"time"

	diodes "code.cloudfoundry.org/go-diodes"

	"github.com/cloudfoundry/metric-store-release/src/internal/batch"
	"github.com/cloudfoundry/metric-store-release/src/internal/debug"
	"github.com/cloudfoundry/metric-store-release/src/internal/handoff"
	"github.com/cloudfoundry/metric-store-release/src/internal/metrics"
	"github.com/cloudfoundry/metric-store-release/src/pkg/leanstreams"
	"github.com/cloudfoundry/metric-store-release/src/pkg/logger"
	"github.com/cloudfoundry/metric-store-release/src/pkg/persistence/transform"
	"github.com/cloudfoundry/metric-store-release/src/pkg/rpc"
	_ "github.com/influxdata/influxdb/tsdb/engine"
	"github.com/prometheus/prometheus/pkg/labels"
	prom_storage "github.com/prometheus/prometheus/storage"
)

const (
	BATCH_FLUSH_INTERVAL = 500 * time.Millisecond

	MAX_BATCH_SIZE_IN_BYTES             = 32 * 1024
	MAX_INTERNODE_PAYLOAD_SIZE_IN_BYTES = 2 * MAX_BATCH_SIZE_IN_BYTES
)

type RemoteAppender struct {
	log     *logger.Logger
	metrics debug.MetricRegistrar

	mu     sync.Mutex
	points []*rpc.Point

	targetNodeIndex string
	remoteAddr      string
	nodeBuffer      *diodes.OneToOne

	// internodeConn chan *leanstreams.TCPClient
	TLSConfig          *tls.Config
	handoffStoragePath string
}

// TODO: remove addr and tlsconfig, replace with connection
// then, these connections can be managed by MS
func NewRemoteAppender(targetNodeIndex string, remoteAddr string, tlsConfig *tls.Config, opts ...RemoteAppenderOption) prom_storage.Appender {
	appender := &RemoteAppender{
		log:                logger.NewNop(),
		metrics:            &debug.NullRegistrar{},
		points:             []*rpc.Point{},
		targetNodeIndex:    targetNodeIndex,
		remoteAddr:         remoteAddr,
		TLSConfig:          tlsConfig,
		handoffStoragePath: "/tmp/metric-store/handoff",
	}

	for _, opt := range opts {
		opt(appender)
	}

	appender.nodeBuffer = diodes.NewOneToOne(16384, diodes.AlertFunc(func(missed int) {
		appender.metrics.Add(metrics.MetricStoreDroppedPointsTotal, float64(missed), appender.targetNodeIndex)
		appender.log.Info("dropped points for remote node", logger.Count(missed), logger.String("remote_node", appender.targetNodeIndex))
	}))

	// Starting this in a goroutine otherwise connecting to each other node
	// will block forever
	go appender.createWriter()

	return appender
}

type RemoteAppenderOption func(*RemoteAppender)

func WithRemoteAppenderLogger(log *logger.Logger) RemoteAppenderOption {
	return func(a *RemoteAppender) {
		a.log = log
	}
}

func WithRemoteAppenderHandoffStoragePath(handoffStoragePath string) RemoteAppenderOption {
	return func(a *RemoteAppender) {
		a.handoffStoragePath = handoffStoragePath
	}
}

func WithRemoteAppenderMetrics(metrics debug.MetricRegistrar) RemoteAppenderOption {
	return func(a *RemoteAppender) {
		a.metrics = metrics
	}
}

func (a *RemoteAppender) createWriter() {
	// defer store.connectionGroup.Done()

	cfg := &leanstreams.TCPClientConfig{
		MaxMessageSize: MAX_INTERNODE_PAYLOAD_SIZE_IN_BYTES,
		Address:        a.remoteAddr,
		TLSConfig:      a.TLSConfig,
	}

	var tcpClient *leanstreams.TCPClient
	var err error

	for {
		tcpClient, err = leanstreams.DialTCP(cfg)
		if err != nil {
			// waiting for remote node to start listening

			// TODO: do this better
			time.Sleep(100 * time.Millisecond)

			continue
		}

		break
	}
	// a.internodeConn <- tcpClient

	nodeHandoffStoragePath := fmt.Sprintf("%s/%s", a.handoffStoragePath, a.targetNodeIndex)
	err = os.MkdirAll(nodeHandoffStoragePath, os.ModePerm)
	if err != nil {
		// a.log.Fatal("failed to create handoff storage directory", err, logger.String("path", nodeHandoffStoragePath))
		a.log.Panic("failed to create handoff storage directory", logger.Error(err), logger.String("path", nodeHandoffStoragePath))
	}

	writeReplayer := handoff.NewWriteReplayer(
		nodeHandoffStoragePath,
		tcpClient,
		a.metrics,
		a.targetNodeIndex,
		handoff.WithWriteReplayerLogger(a.log),
	)
	err = writeReplayer.Open()
	if err != nil {
		// a.log.Fatal("failed to open handoff storage", err, logger.String("path", nodeHandoffStoragePath))
		a.log.Panic("failed to open handoff storage", logger.Error(err), logger.String("path", nodeHandoffStoragePath))
	}

	writer := func(points []*rpc.Point) {
		// TODO: consider adding back in a timeout (i.e. 3 seconds)
		start := time.Now()

		var payload bytes.Buffer
		enc := gob.NewEncoder(&payload)
		err := enc.Encode(rpc.Batch{Points: points})
		if err != nil {
			a.log.Error("gob encode error", err)
			return
		}

		bytesWritten, err := tcpClient.Write(payload.Bytes())
		if err != nil {
			err = writeReplayer.Write(points)
			if err != nil {
				a.log.Error("failed to write to write replayer", err, logger.String("node", a.targetNodeIndex))
			}
			return
		}
		a.log.Debug("wrote bytes", logger.Count(bytesWritten))

		duration := transform.DurationToSeconds(time.Since(start))
		a.metrics.Set(metrics.MetricStoreDistributedRequestDurationSeconds, duration, a.targetNodeIndex)
		a.metrics.Add(metrics.MetricStoreDistributedPointsTotal, float64(len(points)), a.targetNodeIndex)
	}

	batcher := batch.NewBatcher(
		BATCH_FLUSH_INTERVAL,
		MAX_BATCH_SIZE_IN_BYTES,
		a.nodeBuffer,
		writer,
		func(count int) {
			a.metrics.Add(metrics.MetricStoreDroppedPointsTotal, float64(count), a.targetNodeIndex)
		},
	)
	batcher.Start()
}

func (a *RemoteAppender) Add(l labels.Labels, timestamp int64, value float64) (uint64, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	point := &rpc.Point{
		Name:      l.Get(labels.MetricName),
		Value:     value,
		Timestamp: timestamp,
		Labels:    l.Map(),
	}
	a.points = append(a.points, point)

	return 0, nil
}

func (a *RemoteAppender) AddFast(l labels.Labels, ref uint64, t int64, v float64) error {
	panic("not implemented")
}

func (a *RemoteAppender) Commit() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	for _, point := range a.points {
		a.nodeBuffer.Set(diodes.GenericDataType(point))
	}

	a.points = a.points[:0]

	return nil
}

func (a *RemoteAppender) Rollback() error {
	panic("not implemented")
}
