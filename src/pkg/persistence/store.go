package persistence

import (
	"context"
	"fmt"
	"math"
	"time"

	// You will need to make sure this import exists for side effects:
	// _ "github.com/influxdata/influxdb/tsdb/engine"
	// the go linter in some instances removes it

	"github.com/cloudfoundry/metric-store-release/src/internal/debug"
	"github.com/cloudfoundry/metric-store-release/src/pkg/logger"
	"github.com/influxdata/influxdb/tsdb"
	_ "github.com/influxdata/influxdb/tsdb/engine"
	"github.com/prometheus/prometheus/storage"
)

const (
	UNCONFIGURED_DISK_FREE_PERCENT_TARGET = -1
	UNCONFIGURED_RETENTION_PERIOD         = -1
	UNCONFIGURED_EXPIRY_FREQUENCY         = -1
	UNCONFIGURED_METRICS_EMIT_DURATION    = -1
	UNKNOWN_DISK_FREE_PERCENT             = 100
)

type RetentionConfig struct {
	RetentionPeriod       time.Duration
	ExpiryFrequency       time.Duration
	DiskFreePercentTarget float64
}

type diskFreeReporter func() (float64, error)

type Store struct {
	adapter *InfluxAdapter
	metrics debug.MetricRegistrar
	tsStore *tsdb.Store
	log     *logger.Logger

	labelTruncationLength uint
	metricsEmitDuration   time.Duration
	expiryConfig          RetentionConfig
	diskFreeReporter      diskFreeReporter
}

func NewStore(storagePath string, metrics debug.MetricRegistrar, opts ...StoreOption) *Store {
	store := &Store{
		metrics:               metrics,
		log:                   logger.NewNop(),
		labelTruncationLength: 256,
		metricsEmitDuration:   10 * time.Minute,
		expiryConfig: RetentionConfig{
			ExpiryFrequency:       time.Duration(math.MaxInt64),
			RetentionPeriod:       time.Duration(math.MaxInt64),
			DiskFreePercentTarget: 0,
		},
		diskFreeReporter: func() (float64, error) { return 0, nil },
	}

	for _, opt := range opts {
		opt(store)
	}

	var err error
	store.tsStore, err = OpenTsStore(storagePath)
	if err != nil {
		store.log.Fatal("failed to open store", err)
	}

	store.adapter = NewInfluxAdapter(store.tsStore, metrics, store.log)

	go store.start()

	return store
}

type StoreOption func(*Store)

func WithAppenderLabelTruncationLength(length uint) StoreOption {
	return func(s *Store) {
		s.labelTruncationLength = length
	}
}

func WithLogger(log *logger.Logger) StoreOption {
	return func(s *Store) {
		s.log = log
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

func WithDiskFreeReporter(diskFreeReporter func() (float64, error)) StoreOption {
	return func(s *Store) {
		s.diskFreeReporter = diskFreeReporter
	}
}

func (store *Store) Compact() {
	for _, shardId := range store.tsStore.ShardIDs() {
		shard := store.tsStore.Shard(shardId)
		shard.ScheduleFullCompaction()
	}
}

func (store *Store) Querier(ctx context.Context, mint, maxt int64) (storage.Querier, error) {
	return NewQuerier(
		ctx,
		store.adapter,
		store.metrics,
	), nil
}

func (store *Store) Appender() (storage.Appender, error) {
	return NewAppender(
		store.adapter,
		store.metrics,
		WithLabelTruncationLength(store.labelTruncationLength),
		WithAppenderLogger(store.log),
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

	if store.expiryConfig.ExpiryFrequency > UNCONFIGURED_EXPIRY_FREQUENCY {
		for range time.Tick(store.expiryConfig.ExpiryFrequency) {
			store.deleteExpiredData()
		}
	}
}

func (store *Store) periodicMetrics() {
	store.emitStorageDurationMetric()

	if store.metricsEmitDuration > UNCONFIGURED_METRICS_EMIT_DURATION {
		for range time.Tick(store.metricsEmitDuration) {
			store.emitStorageDurationMetric()
		}
	}
}

func (store *Store) deleteExpiredData() {
	if store.expiryConfig.RetentionPeriod > UNCONFIGURED_RETENTION_PERIOD {
		cutoff := time.Now().Add(-store.expiryConfig.RetentionPeriod)
		store.log.Debug("expiring old data", logger.String("older than", cutoff.Format(time.RFC3339)))
		store.deleteOlderThan(cutoff)
	}

	if store.expiryConfig.DiskFreePercentTarget > UNCONFIGURED_DISK_FREE_PERCENT_TARGET {
		diskFree, err := store.diskFreeReporter()
		if err != nil {
			store.log.Error("error reporting disk free", err)
			return
		}

		if diskFree < store.expiryConfig.DiskFreePercentTarget {
			store.log.Debug("expiring data due to disk free space", logger.String("disk free", fmt.Sprintf("%.0f%%", diskFree)))
			store.deleteOldest()
		}
	}
}

func (store *Store) deleteOlderThan(cutoff time.Time) {
	numShardsExpired, err := store.adapter.DeleteOlderThan(cutoff.UnixNano())
	if err != nil {
		store.log.Error("error deleting old shards", err)
	}

	store.metrics.Add(debug.MetricStoreExpiredShardsTotal, float64(numShardsExpired))
}

func (store *Store) deleteOldest() {
	err := store.adapter.DeleteOldest()
	if err != nil {
		store.log.Error("error deleting oldest shard", err)
	}
	store.metrics.Inc(debug.MetricStorePrunedShardsTotal)
}

func (store *Store) emitStorageDurationMetric() {
	oldestShardID, err := store.adapter.OldestContiguousShardID()
	if err != nil {
		return
	}

	duration := time.Since(time.Unix(0, int64(oldestShardID)))
	store.metrics.Set(debug.MetricStoreStorageDays, math.Floor(duration.Hours()/24))
}

func (store *Store) emitStorageMetrics() {
	for {
		time.Sleep(time.Minute)

		statistics := store.tsStore.Statistics(map[string]string{})

		for _, statistic := range statistics {
			if statistic.Values["numSeries"] != nil {
				store.metrics.Set(debug.MetricStoreSeriesCount, float64(statistic.Values["numSeries"].(int64)))
			}

			if statistic.Values["numMeasurements"] != nil {
				store.metrics.Set(debug.MetricStoreMeasurementsCount, float64(statistic.Values["numMeasurements"].(int64)))
			}
		}
	}
}
