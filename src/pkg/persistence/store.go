package persistence

import (
	"context"
	"io/ioutil"
	"log"
	"time"

	// You will need to make sure this import exists for side effects:
	// _ "github.com/influxdata/influxdb/tsdb/engine"
	// the go linter in some instances removes it

	"github.com/influxdata/influxdb/tsdb"
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
	tsStore *tsdb.Store
	log     *log.Logger

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
	indexSizeGauge                 func(float64)
	numberOfSeriesGauge            func(float64)
	numberOfMeasurementsGauge      func(float64)
}

func NewStore(storagePath string, m MetricsInitializer, opts ...StoreOption) *Store {
	store := &Store{
		metrics: Metrics{
			incNumShardsExpired:       m.NewCounter("metric_store_num_shards_expired"),
			incNumShardsPruned:        m.NewCounter("metric_store_num_shards_pruned"),
			storageDurationGauge:      m.NewGauge("metric_store_storage_duration", "days"),
			indexSizeGauge:            m.NewGauge("metric_store_index_size", "byte"),
			numberOfSeriesGauge:       m.NewGauge("metric_store_num_series", "series"),
			numberOfMeasurementsGauge: m.NewGauge("metric_store_num_measurements", "measurement"),
			incNumGetErrors:           m.NewCounter("metric_store_num_get_errors"),
		},
		log:                   log.New(ioutil.Discard, "", 0),
		labelTruncationLength: 256,
	}

	for _, opt := range opts {
		opt(store)
	}

	var err error
	store.tsStore, err = OpenTsStore(storagePath)
	if err != nil {
		store.log.Fatalf("failed to open store: %v", err)
	}

	store.adapter = NewInfluxAdapter(store.tsStore, m)

	return store
}

type StoreOption func(*Store)

func WithAppenderLabelTruncationLength(length uint) StoreOption {
	return func(s *Store) {
		s.labelTruncationLength = length
	}
}

func WithLogger(l *log.Logger) StoreOption {
	return func(s *Store) {
		s.log = l
	}
}

func (store *Store) Querier(ctx context.Context, mint, maxt int64) (storage.Querier, error) {
	return NewQuerier(
		store.adapter,
		store.metrics,
	), nil
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
		store.log.Println(err)
	}
	store.metrics.incNumShardsExpired(numShardsExpired)
}

func (store *Store) DeleteOldest() {
	err := store.adapter.DeleteOldest()
	if err != nil {
		store.log.Println(err)
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

func (store *Store) EmitStorageMetrics() {
	for {
		time.Sleep(time.Minute)

		statistics := store.tsStore.Statistics(map[string]string{})
		store.metrics.indexSizeGauge(float64(store.tsStore.IndexBytes()))

		for _, statistic := range statistics {
			if statistic.Values["numSeries"] != nil {
				store.metrics.numberOfSeriesGauge(float64(statistic.Values["numSeries"].(int64)))
			}

			if statistic.Values["numMeasurements"] != nil {
				store.metrics.numberOfMeasurementsGauge(float64(statistic.Values["numMeasurements"].(int64)))
			}
		}
	}
}

func (s *Store) Close() error {
	return s.adapter.Close()
}

func (s *Store) StartTime() (int64, error) {
	panic("not implemented")
}
