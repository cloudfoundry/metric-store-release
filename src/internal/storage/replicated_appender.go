package storage

import (
	// You will need to make sure this import exists for side effects:
	// _ "github.com/influxdata/influxdb/tsdb/engine"
	// the go linter in some instances removes it

	"github.com/cloudfoundry/metric-store-release/src/internal/debug"
	"github.com/cloudfoundry/metric-store-release/src/internal/routing"
	"github.com/cloudfoundry/metric-store-release/src/pkg/logger"
	_ "github.com/influxdata/influxdb/tsdb/engine"
	"github.com/prometheus/prometheus/pkg/labels"
	"github.com/prometheus/prometheus/storage"
	prom_storage "github.com/prometheus/prometheus/storage"
)

type ReplicatedAppender struct {
	log     *logger.Logger
	metrics debug.MetricRegistrar

	appenders []prom_storage.Appender
	lookup    routing.Lookup
}

func NewReplicatedAppender(appenders []storage.Appender, lookup routing.Lookup, opts ...ReplicatedAppenderOption) prom_storage.Appender {
	appender := &ReplicatedAppender{
		appenders: appenders,
		lookup:    lookup,
		log:       logger.NewNop(),
		metrics:   &debug.NullRegistrar{},
	}

	for _, opt := range opts {
		opt(appender)
	}

	return appender
}

type ReplicatedAppenderOption func(*ReplicatedAppender)

func WithReplicatedAppenderLogger(log *logger.Logger) ReplicatedAppenderOption {
	return func(a *ReplicatedAppender) {
		a.log = log
	}
}

func WithReplicatedAppenderMetrics(metrics debug.MetricRegistrar) ReplicatedAppenderOption {
	return func(a *ReplicatedAppender) {
		a.metrics = metrics
	}
}

func (a *ReplicatedAppender) Add(l labels.Labels, time int64, value float64) (uint64, error) {

	for _, nodeIndex := range a.lookup(l.Get(labels.MetricName)) {
		a.appenders[nodeIndex].Add(l, time, value)
	}

	return 0, nil
}

func (a *ReplicatedAppender) AddFast(l labels.Labels, ref uint64, time int64, value float64) error {

	for _, nodeIndex := range a.lookup(l.Get(labels.MetricName)) {
		a.appenders[nodeIndex].AddFast(l, ref, time, value)
	}

	return nil
}

func (a *ReplicatedAppender) Commit() error {
	for _, appender := range a.appenders {
		appender.Commit()
	}

	return nil
}

func (s *ReplicatedAppender) Rollback() error {
	panic("not implemented")
}
