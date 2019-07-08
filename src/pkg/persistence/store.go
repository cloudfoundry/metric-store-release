package persistence

import (
	"context"
	"fmt"
	"time"

	// You will need to make sure this import exists for side effects:
	// _ "github.com/influxdata/influxdb/tsdb/engine"
	// the go linter in some instances removes it
	_ "github.com/influxdata/influxdb/tsdb/engine"
	"github.com/prometheus/prometheus/storage"
)

type MetricsInitializer interface {
	NewCounter(name string) func(delta uint64)
	NewGauge(name, unit string) func(value float64)
}

type Store struct {
	adapter *InfluxAdapter
	metrics Metrics
	querier storage.Querier

	labelTruncationLength uint
}

type Metrics struct {
	incNumShardsExpired            func(delta uint64)
	incNumShardsPruned             func(delta uint64)
	incNumGetErrors                func(delta uint64)
	storageDurationGauge           func(value float64)
	labelTagsQueryTime             func(time float64)
	labelFieldsQueryTime           func(time float64)
	labelMeasurementNamesQueryTime func(time float64)
}

func NewStore(adapter *InfluxAdapter, m MetricsInitializer, opts ...StoreOption) *Store {
	store := &Store{
		adapter: adapter,

		metrics: Metrics{
			incNumShardsExpired:  m.NewCounter("metric_store_num_shards_expired"),
			incNumShardsPruned:   m.NewCounter("metric_store_num_shards_pruned"),
			storageDurationGauge: m.NewGauge("metric_store_storage_duration", "days"),
		},
		querier:               NewQuerier(adapter, m),
		labelTruncationLength: 256,
	}

	for _, opt := range opts {
		opt(store)
	}

	return store
}

type StoreOption func(*Store)

func WithAppenderLabelTruncationLength(length uint) StoreOption {
	return func(s *Store) {
		s.labelTruncationLength = length
	}
}

func (store *Store) Querier(ctx context.Context, mint, maxt int64) (storage.Querier, error) {
	return store.querier, nil
}

func (store *Store) Appender() (storage.Appender, error) {
	return NewAppender(
		store.adapter,
		WithLabelTruncationLength(store.labelTruncationLength),
	), nil
}

func (store *Store) DeleteOlderThan(cutoff time.Time) {
	numShardsExpired, err := store.adapter.DeleteOlderThan(cutoff.UnixNano())
	if err != nil {
		fmt.Println(err)
	}
	store.metrics.incNumShardsExpired(numShardsExpired)
}

func (store *Store) DeleteOldest() {
	err := store.adapter.DeleteOldest()
	if err != nil {
		fmt.Println(err)
	}
	store.metrics.incNumShardsPruned(1)
}

func (store *Store) EmitStorageDurationMetric() {
	oldestShardID, err := store.adapter.OldestShardID()
	if err != nil {
		return
	}

	duration := time.Since(time.Unix(0, int64(oldestShardID)))
	store.metrics.storageDurationGauge(float64(int(duration.Hours()) / 24))
}

func (s *Store) Close() error {
	return s.adapter.Close()
}

func (s *Store) StartTime() (int64, error) {
	panic("not implemented")
}
