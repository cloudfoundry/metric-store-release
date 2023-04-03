package storage

// TODO
// - Close()
//   - passing in connections, not config

import (
	// You will need to make sure this import exists for side effects:
	// _ "github.com/influxdata/influxdb/tsdb/engine"
	// the go linter in some instances removes it

	"bytes"
	"encoding/gob"
	"fmt"
	"github.com/prometheus/prometheus/model/exemplar"
	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/metadata"
	"os"
	"sync"
	"time"

	"code.cloudfoundry.org/go-diodes"

	_ "github.com/influxdata/influxdb/tsdb/engine"
	"github.com/prometheus/prometheus/model/labels"
	prom_storage "github.com/prometheus/prometheus/storage"

	"github.com/cloudfoundry/metric-store-release/src/internal/batch"
	"github.com/cloudfoundry/metric-store-release/src/internal/handoff"
	"github.com/cloudfoundry/metric-store-release/src/internal/metrics"
	"github.com/cloudfoundry/metric-store-release/src/pkg/leanstreams"
	"github.com/cloudfoundry/metric-store-release/src/pkg/logger"
	"github.com/cloudfoundry/metric-store-release/src/pkg/persistence/transform"
	"github.com/cloudfoundry/metric-store-release/src/pkg/rpc"
)

const (
	BATCH_FLUSH_INTERVAL = 500 * time.Millisecond

	MAX_BATCH_SIZE_IN_BYTES             = 32 * 1024
	MAX_INTERNODE_PAYLOAD_SIZE_IN_BYTES = 2 * MAX_BATCH_SIZE_IN_BYTES
)

type RemoteAppender struct {
	log     *logger.Logger
	metrics metrics.Registrar

	mu     sync.Mutex
	points []*rpc.Point

	targetNodeIndex string
	nodeBuffer      *diodes.OneToOne
	connection      *leanstreams.Connection

	handoffStoragePath string

	done chan struct{}
}

func NewRemoteAppender(targetNodeIndex string, connection *leanstreams.Connection, done chan struct{}, opts ...RemoteAppenderOption) prom_storage.Appender {
	appender := &RemoteAppender{
		log:                logger.NewNop(),
		metrics:            &metrics.NullRegistrar{},
		points:             []*rpc.Point{},
		targetNodeIndex:    targetNodeIndex,
		connection:         connection,
		handoffStoragePath: "/tmp/metric-store/handoff",
		done:               done,
	}

	for _, opt := range opts {
		opt(appender)
	}

	appender.nodeBuffer = diodes.NewOneToOne(16384, diodes.AlertFunc(func(missed int) {
		appender.metrics.Add(metrics.MetricStoreDroppedPointsTotal, float64(missed), appender.targetNodeIndex)
		appender.log.Info("diode: dropped points for remote node", logger.Count(missed), logger.String("remote_node", appender.targetNodeIndex))
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

func WithRemoteAppenderMetrics(metrics metrics.Registrar) RemoteAppenderOption {
	return func(a *RemoteAppender) {
		a.metrics = metrics
	}
}

func (a *RemoteAppender) createWriter() {
	a.connection.Connect()

	nodeHandoffStoragePath := fmt.Sprintf("%s/%s", a.handoffStoragePath, a.targetNodeIndex)
	err := os.MkdirAll(nodeHandoffStoragePath, os.ModePerm)
	if err != nil {
		a.log.Panic("failed to create handoff storage directory", logger.Error(err), logger.String("path", nodeHandoffStoragePath))
	}

	queue := handoff.NewDiskBackedQueue(nodeHandoffStoragePath)

	writeReplayer := handoff.NewWriteReplayer(
		queue,
		a.connection.Client(),
		a.metrics,
		a.targetNodeIndex,
		handoff.WithWriteReplayerLogger(a.log),
	)
	err = writeReplayer.Open(a.done)
	if err != nil {
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

		client := a.connection.Client()
		bytesWritten, err := client.Write(payload.Bytes())
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
		a.done,
		func(count int) {
			a.metrics.Add(metrics.MetricStoreDroppedPointsTotal, float64(count), a.targetNodeIndex)
			a.log.Info("batcher: dropped points for remote node", logger.Count(count), logger.String("remote_node", a.targetNodeIndex))
		},
	)
	batcher.Start()
}

func (a *RemoteAppender) Append(ref prom_storage.SeriesRef, l labels.Labels, timestamp int64, value float64) (prom_storage.SeriesRef, error) {
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

func (a *RemoteAppender) AppendExemplar(ref prom_storage.SeriesRef, l labels.Labels, e exemplar.Exemplar) (prom_storage.SeriesRef, error) {
	panic("not implemented")
}

func (a *RemoteAppender) AppendHistogram(ref prom_storage.SeriesRef, l labels.Labels, t int64, h *histogram.Histogram, fh *histogram.FloatHistogram) (prom_storage.SeriesRef, error) {
	panic("not implemented")
}

func (a *RemoteAppender) UpdateMetadata(ref prom_storage.SeriesRef, l labels.Labels, m metadata.Metadata) (prom_storage.SeriesRef, error) {
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
