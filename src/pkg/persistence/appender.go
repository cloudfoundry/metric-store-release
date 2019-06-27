package persistence

import (

	// You will need to make sure this import exists for side effects:
	// _ "github.com/influxdata/influxdb/tsdb/engine"
	// the go linter in some instances removes it

	"sync"

	_ "github.com/influxdata/influxdb/tsdb/engine"
	"github.com/prometheus/prometheus/pkg/labels"

	rpc "github.com/cloudfoundry/metric-store-release/src/pkg/rpc/metricstore_v1"
)

type StoreAppender struct {
	mu                    sync.Mutex
	points                []*rpc.Point
	adapter               *InfluxAdapter
	metrics               AppenderMetrics
	labelTruncationLength uint
}

// TODO: refactor?
type AppenderMetrics struct {
	incIngress func(delta uint64)
}

func NewAppender(adapter *InfluxAdapter, m MetricsInitializer, opts ...WithAppenderOption) *StoreAppender {
	appender := &StoreAppender{
		adapter: adapter,

		metrics: AppenderMetrics{
			incIngress: m.NewCounter("metric_store_ingress"),
		},

		labelTruncationLength: 256,
		points:                []*rpc.Point{},
	}

	for _, opt := range opts {
		opt(appender)
	}

	return appender
}

type WithAppenderOption func(*StoreAppender)

func WithLabelTruncationLength(length uint) WithAppenderOption {
	return func(a *StoreAppender) {
		a.labelTruncationLength = length
	}
}

func (a *StoreAppender) Add(l labels.Labels, time int64, value float64) (uint64, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.metrics.incIngress(1)

	point := &rpc.Point{
		Name:      l.Get("__name__"),
		Timestamp: time,
		Value:     value,
		Labels:    a.cleanLabels(l),
	}
	a.points = append(a.points, point)

	return 0, nil
}

func (s *StoreAppender) AddFast(l labels.Labels, ref uint64, t int64, v float64) error {
	panic("not implemented")
}

func (a *StoreAppender) Commit() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	err := a.adapter.WritePoints(a.points)
	a.points = a.points[:0]
	return err
}

func (s *StoreAppender) Rollback() error {
	panic("not implemented")
}

func (a *StoreAppender) cleanLabels(l labels.Labels) map[string]string {
	newLabels := l.Map()

	for name, value := range newLabels {
		if uint(len(value)) > a.labelTruncationLength {
			newLabels[name] = value[:a.labelTruncationLength]
		}
	}

	delete(newLabels, "__name__")

	return newLabels
}
