package persistence

import (
	"context"
	"io/ioutil"
	"log"
	"math"
	"time"

	// You will need to make sure this import exists for side effects:
	// _ "github.com/influxdata/influxdb/tsdb/engine"
	// the go linter in some instances removes it

	"github.com/influxdata/influxdb/tsdb"
	_ "github.com/influxdata/influxdb/tsdb/engine"
	"github.com/prometheus/prometheus/storage"
)

const (
	NULL_DISK_FREE_PERCENT_TARGET = -1
	NULL_RETENTION_PERIOD         = -1
	NULL_EXPIRY_FREQUENCY         = -1
	NULL_METRICS_EMIT_DURATION    = -1
)

type MetricsInitializer interface {
	NewCounter(name string) func(delta uint64)
	NewGauge(name, unit string) func(value float64)
}

type RetentionConfig struct {
	RetentionPeriod       time.Duration
	ExpiryFrequency       time.Duration
	DiskFreePercentTarget float64
}

type diskFreeReporter func() float64

type Store struct {
	adapter *InfluxAdapter
	metrics Metrics
	tsStore *tsdb.Store
	log     *log.Logger

	labelTruncationLength uint
	metricsEmitDuration   time.Duration
	expiryConfig          RetentionConfig
	diskFreeReporter      diskFreeReporter
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
		metricsEmitDuration:   10 * time.Minute,
		expiryConfig: RetentionConfig{
			ExpiryFrequency:       time.Duration(math.MaxInt64),
			RetentionPeriod:       time.Duration(math.MaxInt64),
			DiskFreePercentTarget: 0,
		},
		diskFreeReporter: func() float64 { return 0 },
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

	go store.start()

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

// WithMetricsEmitDuration sets the duration to which periodic metrics are
// emitted
func WithMetricsEmitDuration(metricsEmitDuration time.Duration) StoreOption {
	return func(s *Store) {
		s.metricsEmitDuration = metricsEmitDuration
	}
}

// WithRetentionConfig sets the frequency of automated cleanup that expires old
// data.
func WithRetentionConfig(config RetentionConfig) StoreOption {
	return func(s *Store) {
		s.expiryConfig = config
	}
}

func WithDiskFreeReporter(diskFreeReporter func() float64) StoreOption {
	return func(s *Store) {
		s.diskFreeReporter = diskFreeReporter
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

func (s *Store) Close() error {
	return s.adapter.Close()
}

func (s *Store) StartTime() (int64, error) {
	panic("not implemented")
}

func (store *Store) start() {
	go store.emitStorageMetrics()
	go store.periodicExpiry()
	go store.periodicMetrics()
}

func (store *Store) periodicExpiry() {
	store.deleteExpiredData()

	if store.expiryConfig.ExpiryFrequency > NULL_EXPIRY_FREQUENCY {
		for range time.Tick(store.expiryConfig.ExpiryFrequency) {
			store.deleteExpiredData()
		}
	}
}

func (store *Store) periodicMetrics() {
	store.emitStorageDurationMetric()

	if store.metricsEmitDuration > NULL_METRICS_EMIT_DURATION {
		for range time.Tick(store.metricsEmitDuration) {
			store.emitStorageDurationMetric()
		}
	}
}

func (store *Store) deleteExpiredData() {
	if store.expiryConfig.RetentionPeriod > NULL_RETENTION_PERIOD {
		cutoff := time.Now().Add(-store.expiryConfig.RetentionPeriod)
		store.log.Printf("expiring data older than %s", cutoff.Format(time.RFC3339))
		store.deleteOlderThan(cutoff)
	}

	if store.expiryConfig.DiskFreePercentTarget > NULL_DISK_FREE_PERCENT_TARGET {
		diskFree := store.diskFreeReporter()
		if diskFree < store.expiryConfig.DiskFreePercentTarget {
			store.log.Printf("expiring data due to disk free space of %.0f%%", diskFree)
			store.deleteOldest()
		}
	}
}

func (store *Store) deleteOlderThan(cutoff time.Time) {
	numShardsExpired, err := store.adapter.DeleteOlderThan(cutoff.UnixNano())
	if err != nil {
		store.log.Println(err)
	}
	store.metrics.incNumShardsExpired(numShardsExpired)
}

func (store *Store) deleteOldest() {
	err := store.adapter.DeleteOldest()
	if err != nil {
		store.log.Println(err)
	}
	store.metrics.incNumShardsPruned(1)
}

func (store *Store) emitStorageDurationMetric() {
	oldestShardID, err := store.adapter.OldestShardID()
	if err != nil {
		return
	}

	duration := time.Since(time.Unix(0, int64(oldestShardID)))
	store.metrics.storageDurationGauge(float64(int(duration.Hours()) / 24))
}

func (store *Store) emitStorageMetrics() {
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
