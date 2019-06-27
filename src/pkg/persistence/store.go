package persistence

import (
	"fmt"
	"time"

	// You will need to make sure this import exists for side effects:
	// _ "github.com/influxdata/influxdb/tsdb/engine"
	// the go linter in some instances removes it
	_ "github.com/influxdata/influxdb/tsdb/engine"
	"github.com/prometheus/prometheus/pkg/labels"
	"github.com/prometheus/prometheus/storage"

	"github.com/cloudfoundry/metric-store-release/src/pkg/persistence/transform"
)

type MetricsInitializer interface {
	NewCounter(name string) func(delta uint64)
	NewGauge(name, unit string) func(value float64)
}

type Store struct {
	adapter  *InfluxAdapter
	metrics  Metrics
	appender storage.Appender
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

func NewStore(appender storage.Appender, adapter *InfluxAdapter, m MetricsInitializer) *Store {
	store := &Store{
		appender: appender,
		adapter:  adapter,

		metrics: Metrics{
			incNumShardsExpired:  m.NewCounter("metric_store_num_shards_expired"),
			incNumShardsPruned:   m.NewCounter("metric_store_num_shards_pruned"),
			incNumGetErrors:      m.NewCounter("metric_store_num_get_errors"),
			storageDurationGauge: m.NewGauge("metric_store_storage_duration", "days"),
		},
	}

	return store
}

func (store *Store) Appender() (storage.Appender, error) {
	return store.appender, nil
}

func (store *Store) Select(params *storage.SelectParams, labelMatchers ...*labels.Matcher) (storage.SeriesSet, storage.Warnings, error) {
	if params.End != 0 && params.Start > params.End {
		return nil, nil, fmt.Errorf("Start (%d) must be before End (%d)", params.Start, params.End)
	}

	if params.End == 0 {
		params.End = time.Now().UnixNano()
	}

	var name string
	for index, labelMatcher := range labelMatchers {
		if labelMatcher.Name == "__name__" {
			name = labelMatcher.Value
			labelMatchers = append(labelMatchers[:index], labelMatchers[index+1:]...)
			break
		}
	}

	startTimeInNanoseconds := transform.MillisecondsToNanoseconds(params.Start)
	endTimeInNanoseconds := transform.MillisecondsToNanoseconds(params.End) - 1

	builder, err := store.adapter.GetPoints(name, startTimeInNanoseconds, endTimeInNanoseconds, labelMatchers)
	if err != nil {
		store.metrics.incNumGetErrors(1)
		return nil, nil, err
	}

	return builder.SeriesSet(), nil, nil
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

func (store *Store) Close() error {
	return store.adapter.Close()
}

func (store *Store) LabelNames() ([]string, error) {
	distinctKeys := make(map[string]struct{})

	tagKeys := store.adapter.AllTagKeys()
	for _, tagKey := range tagKeys {
		distinctKeys[tagKey] = struct{}{}
	}

	var labels []string
	for k := range distinctKeys {
		labels = append(labels, k)
	}

	return labels, nil
}

func (store *Store) LabelValues(name string) ([]string, error) {
	distinctValues := make(map[string]struct{})

	if name == transform.MEASUREMENT_NAME {
		values := store.adapter.AllMeasurementNames()
		return values, nil
	}

	tagValues := store.adapter.AllTagValues(name)
	for _, tagValue := range tagValues {
		distinctValues[tagValue] = struct{}{}
	}

	var values []string
	for v := range distinctValues {
		values = append(values, v)
	}

	return values, nil
}
