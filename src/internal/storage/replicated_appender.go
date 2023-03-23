package storage

import (
	// You will need to make sure this import exists for side effects:
	// _ "github.com/influxdata/influxdb/tsdb/engine"
	// the go linter in some instances removes it

	"github.com/cloudfoundry/metric-store-release/src/internal/metrics"
	"github.com/cloudfoundry/metric-store-release/src/internal/routing"
	"github.com/cloudfoundry/metric-store-release/src/pkg/logger"
	_ "github.com/influxdata/influxdb/tsdb/engine"
	"github.com/prometheus/prometheus/model/exemplar"
	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/metadata"
	"github.com/prometheus/prometheus/storage"
	prom_storage "github.com/prometheus/prometheus/storage"
)

type ReplicatedAppender struct {
	log     *logger.Logger
	metrics metrics.Registrar

	appenders []prom_storage.Appender
	lookup    routing.Lookup
}

func NewReplicatedAppender(appenders []storage.Appender, lookup routing.Lookup, opts ...ReplicatedAppenderOption) prom_storage.Appender {
	appender := &ReplicatedAppender{
		appenders: appenders,
		lookup:    lookup,
		log:       logger.NewNop(),
		metrics:   &metrics.NullRegistrar{},
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

func WithReplicatedAppenderMetrics(metrics metrics.Registrar) ReplicatedAppenderOption {
	return func(a *ReplicatedAppender) {
		a.metrics = metrics
	}
}

func (a *ReplicatedAppender) AppendExemplar(ref prom_storage.SeriesRef, l labels.Labels, e exemplar.Exemplar) (prom_storage.SeriesRef, error) {
	//TODO implement me
	panic("implement me")
}

func (a *ReplicatedAppender) AppendHistogram(ref prom_storage.SeriesRef, l labels.Labels, t int64, h *histogram.Histogram, fh *histogram.FloatHistogram) (prom_storage.SeriesRef, error) {
	//TODO implement me
	panic("implement me")
}

func (a *ReplicatedAppender) UpdateMetadata(ref prom_storage.SeriesRef, l labels.Labels, m metadata.Metadata) (prom_storage.SeriesRef, error) {
	//TODO implement me
	panic("implement me")
}

func (a *ReplicatedAppender) Append(ref prom_storage.SeriesRef, l labels.Labels, t int64, v float64) (prom_storage.SeriesRef, error) {
	for _, nodeIndex := range a.lookup(l.Get(labels.MetricName)) {
		if _, err := a.appenders[nodeIndex].Append(ref, l, t, v); err != nil {
			continue
		}
	}

	return 0, nil
}

func (a *ReplicatedAppender) AddFast(_ uint64, _ int64, _ float64) error {
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
