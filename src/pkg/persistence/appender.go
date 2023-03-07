package persistence

import (
	// You will need to make sure this import exists for side effects:
	// _ "github.com/influxdata/influxdb/tsdb/engine"
	// the go linter in some instances removes it

	"errors"
	"github.com/prometheus/prometheus/model/exemplar"
	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/metadata"
	"github.com/prometheus/prometheus/storage"
	"sync"
	"time"

	_ "github.com/influxdata/influxdb/tsdb/engine"
	"github.com/prometheus/prometheus/model/labels"

	"github.com/cloudfoundry/metric-store-release/src/internal/metrics"
	"github.com/cloudfoundry/metric-store-release/src/pkg/logger"
	"github.com/cloudfoundry/metric-store-release/src/pkg/persistence/transform"
	"github.com/cloudfoundry/metric-store-release/src/pkg/rpc"
)

type Adapter interface {
	WritePoints(points []*rpc.Point) error
}

type Appender struct {
	mu                    sync.Mutex
	points                []*rpc.Point
	adapter               Adapter
	labelTruncationLength uint

	log     *logger.Logger
	metrics metrics.Registrar
}

func NewAppender(adapter Adapter, metrics metrics.Registrar, opts ...AppenderOption) *Appender {
	appender := &Appender{
		adapter:               adapter,
		metrics:               metrics,
		log:                   logger.NewNop(),
		labelTruncationLength: 256,
		points:                []*rpc.Point{},
	}

	for _, opt := range opts {
		opt(appender)
	}

	return appender
}

type AppenderOption func(*Appender)

func WithLabelTruncationLength(length uint) AppenderOption {
	return func(a *Appender) {
		a.labelTruncationLength = length
	}
}

func WithAppenderLogger(log *logger.Logger) AppenderOption {
	return func(a *Appender) {
		a.log = log
	}
}

func (a *Appender) Append(ref storage.SeriesRef, l labels.Labels, time int64, value float64) (storage.SeriesRef, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !transform.IsValidFloat(value) {
		return 0, errors.New("NaN float cannot be added")
	}

	point := &rpc.Point{
		Name:      l.Get(labels.MetricName),
		Timestamp: time,
		Value:     value,
		Labels:    a.cleanLabels(l),
	}
	a.points = append(a.points, point)

	return 0, nil
}

func (a *Appender) AppendExemplar(ref storage.SeriesRef, l labels.Labels, e exemplar.Exemplar) (storage.SeriesRef, error) {
	// no longer useful in our implementation, use Append instead
	return 0, nil
}

func (a *Appender) AppendHistogram(ref storage.SeriesRef, l labels.Labels, t int64, h *histogram.Histogram, fh *histogram.FloatHistogram) (storage.SeriesRef, error) {
	// not useful in our implementation
	return 0, nil
}

func (a *Appender) UpdateMetadata(ref storage.SeriesRef, l labels.Labels, m metadata.Metadata) (storage.SeriesRef, error) {
	// not useful in our implementation
	return 0, nil
}

func (a *Appender) Commit() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	start := time.Now()

	err := a.adapter.WritePoints(a.points)

	if err == nil {
		duration := transform.DurationToSeconds(time.Since(start))
		a.metrics.Histogram(metrics.MetricStoreWriteDurationSeconds).Observe(duration)
		a.metrics.Add(metrics.MetricStoreWrittenPointsTotal, float64(len(a.points)))
	}

	a.points = a.points[:0]

	return err
}

func (a *Appender) Rollback() error {
	panic("not implemented")
}

func (a *Appender) cleanLabels(l labels.Labels) map[string]string {
	newLabels := l.Map()

	for name, value := range newLabels {
		if uint(len(value)) > a.labelTruncationLength {
			newLabels[name] = value[:a.labelTruncationLength]
		}
	}

	delete(newLabels, labels.MetricName)

	return newLabels
}
