package persistence

import (
	// You will need to make sure this import exists for side effects:
	// _ "github.com/influxdata/influxdb/tsdb/engine"
	// the go linter in some instances removes it

	"errors"
	"sync"

	_ "github.com/influxdata/influxdb/tsdb/engine"
	"github.com/prometheus/prometheus/pkg/labels"

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

	log *logger.Logger
}

func NewAppender(adapter Adapter, opts ...AppenderOption) *Appender {
	appender := &Appender{
		adapter:               adapter,
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

func (a *Appender) Add(l labels.Labels, time int64, value float64) (uint64, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !transform.IsValidFloat(value) {
		return 0, errors.New("NaN float cannot be added")
	}

	point := &rpc.Point{
		Name:      l.Get("__name__"),
		Timestamp: time,
		Value:     value,
		Labels:    a.cleanLabels(l),
	}
	a.points = append(a.points, point)

	return 0, nil
}

func (a *Appender) AddFast(l labels.Labels, _ uint64, time int64, value float64) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !transform.IsValidFloat(value) {
		return errors.New("NaN float cannot be added")
	}

	point := &rpc.Point{
		Name:      l.Get("__name__"),
		Timestamp: time,
		Value:     value,
		Labels:    a.cleanLabels(l),
	}
	a.adapter.WritePoints([]*rpc.Point{point})
	return nil
}

func (a *Appender) Commit() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	err := a.adapter.WritePoints(a.points)
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

	delete(newLabels, "__name__")

	return newLabels
}
