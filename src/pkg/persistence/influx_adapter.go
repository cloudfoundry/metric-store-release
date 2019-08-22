package persistence

import (
	"context"
	"fmt"
	"regexp"
	"sync"
	"time"

	"github.com/cloudfoundry/metric-store-release/src/pkg/persistence/transform"
	"github.com/cloudfoundry/metric-store-release/src/pkg/rpc"
	"github.com/influxdata/influxdb/models"
	"github.com/influxdata/influxdb/query"
	"github.com/influxdata/influxdb/tsdb"
	"github.com/influxdata/influxql"
	"github.com/prometheus/prometheus/pkg/labels"
)

type InfluxStore interface {
	Close() error
	CreateShard(database string, retentionPolicy string, shardId uint64, enabled bool) error
	DeleteShard(shardId uint64) error
	ShardGroup(shardIds []uint64) tsdb.ShardGroup
	ShardIDs() []uint64
	Statistics(database map[string]string) []models.Statistic
	TagKeys(auth query.Authorizer, shardIDs []uint64, cond influxql.Expr) ([]tsdb.TagKeys, error)
	TagValues(auth query.Authorizer, shardIDs []uint64, cond influxql.Expr) ([]tsdb.TagValues, error)
	WriteToShard(shardId uint64, points []models.Point) error
}

type InfluxAdapter struct {
	influx  InfluxStore
	shards  sync.Map
	metrics Metrics
	sync.RWMutex
}

func NewInfluxAdapter(influx InfluxStore, m MetricsInitializer) *InfluxAdapter {
	t := &InfluxAdapter{
		influx: influx,
		metrics: Metrics{
			labelFieldsQueryTime:           m.NewGauge("metric_store_label_fields_query_time", "milliseconds"),
			labelTagsQueryTime:             m.NewGauge("metric_store_label_tags_query_time", "milliseconds"),
			labelMeasurementNamesQueryTime: m.NewGauge("metric_store_measurement_names_query_time", "milliseconds"),
		},
	}

	for _, shardId := range influx.ShardIDs() {
		checkShardId(shardId)
		t.shards.Store(shardId, struct{}{})
	}

	return t
}

func (t *InfluxAdapter) WritePoints(points []*rpc.Point) error {
	pointBuckets := make(map[int64][]models.Point)
	influxPoints := transform.ToInfluxPoints(points)

	for _, point := range influxPoints {
		shardStart := getShardStartForTimestamp(point.Time().UnixNano())
		pointBuckets[shardStart] = append(pointBuckets[shardStart], point)
	}

	for bucketIndex, points := range pointBuckets {
		t.Lock()
		shardId := t.findOrCreateShardForTimestamp(bucketIndex)
		err := t.influx.WriteToShard(shardId, points)
		t.Unlock()

		if err != nil {
			return err
		}
	}

	return nil
}

func (t *InfluxAdapter) GetPoints(measurementName string, start, end int64, matchers []*labels.Matcher) (*transform.SeriesSetBuilder, error) {
	shardIds := t.forTimestampRange(start, end)
	shards := t.influx.ShardGroup(shardIds)

	fieldSet, dimensionSet, err := shards.FieldDimensions([]string{measurementName})
	if err != nil {
		panic(err)
	}

	var fields, dimensions []string
	for field := range fieldSet {
		fields = append(fields, field)
	}

	for dimension := range dimensionSet {
		dimensions = append(dimensions, dimension)
	}

	filterCondition, err := transform.ToInfluxFilters(matchers)
	if err != nil {
		return nil, err
	}

	var auxFields []influxql.VarRef
	for _, field := range fields {
		auxFields = append(auxFields, influxql.VarRef{Val: field})
	}

	queryOpts := query.IteratorOptions{
		Expr:       influxql.MustParseExpr("value"),
		Aux:        auxFields,
		Dimensions: dimensions,
		StartTime:  start,
		EndTime:    end,
		Condition:  filterCondition,
		Ascending:  true,
		Ordered:    true,
		Limit:      0,
	}

	iterator, err := shards.CreateIterator(
		context.Background(),
		&influxql.Measurement{Name: measurementName},
		queryOpts,
	)
	if err != nil {
		return nil, err
	}

	builder := transform.NewSeriesBuilder()

	// if our query was invalid, we'll have a nil iterator. let's return our
	// empty builder so that the query returns no results.
	if iterator == nil {
		return builder, nil
	}

	defer iterator.Close()

	switch typedIterator := iterator.(type) {
	case query.FloatIterator:
		for {
			floatPoint, err := typedIterator.Next()
			if err != nil {
				return builder, err
			}
			if floatPoint == nil {
				break
			}
			builder.AddInfluxPoint(floatPoint, fields)
		}
	default:
		// fall through
	}
	return builder, nil
}

func (t *InfluxAdapter) AllTagKeys() []string {
	shardIds := t.ShardIDs()
	tagKeys, _ := t.influx.TagKeys(nil, shardIds, nil)

	var values []string
	for _, tagKey := range tagKeys {
		values = append(values, tagKey.Keys...)
	}

	return values
}

func (t *InfluxAdapter) AllTagValues(tagKey string) []string {
	start := time.Now()
	shardIds := t.ShardIDs()
	selectValuesByTagKey := &influxql.BinaryExpr{
		LHS: &influxql.VarRef{Val: "_tagKey"},
		RHS: &influxql.StringLiteral{Val: tagKey},
		Op:  influxql.EQ,
	}

	tagValues, _ := t.influx.TagValues(nil, shardIds, selectValuesByTagKey)

	var values []string
	for _, tagValue := range tagValues {
		for _, value := range tagValue.Values {
			values = append(values, value.Value)
		}
	}

	t.metrics.labelTagsQueryTime(float64(time.Since(start) / time.Millisecond))
	return values
}

func (t *InfluxAdapter) DeleteOlderThan(cutoff int64) (numDeleted uint64, err error) {
	t.Lock()
	defer t.Unlock()

	adjustedCutoff := time.Unix(0, cutoff).Add(-time.Minute).Truncate(24 * time.Hour)
	for _, shardId := range t.forTimestampRange(
		influxql.MinTime,
		adjustedCutoff.UnixNano(),
	) {
		t.Delete(shardId)
		err = t.influx.DeleteShard(shardId)
		if err != nil {
			return
		}

		numDeleted++
	}

	return
}

func (t *InfluxAdapter) DeleteOldest() error {
	t.Lock()
	defer t.Unlock()

	shardId := t.getOldest()
	t.Delete(shardId)
	return t.influx.DeleteShard(shardId)
}

func (t *InfluxAdapter) Delete(shardId uint64) {
	t.shards.Delete(shardId)
}

func (t *InfluxAdapter) ShardIDs() []uint64 {
	t.RLock()
	defer t.RUnlock()

	var shards []uint64

	t.shards.Range(func(shardIdKey interface{}, _ interface{}) bool {
		shardId := shardIdKey.(uint64)
		shards = append(shards, shardId)
		return true
	})

	return shards
}

func (t *InfluxAdapter) OldestShardID() (uint64, error) {
	shardIDs := t.ShardIDs()
	if len(shardIDs) == 0 {
		return 0, fmt.Errorf("Cannot determine oldest shardID when there are no shardIDs")
	}

	oldestShardID := shardIDs[0]
	for _, shardID := range shardIDs {
		if shardID < oldestShardID {
			oldestShardID = shardID
		}
	}

	return oldestShardID, nil
}

func (t *InfluxAdapter) Close() error {
	return t.influx.Close()
}

func (t *InfluxAdapter) AllMeasurementNames() []string {
	start := time.Now()
	measurementNames := t.allShards().MeasurementsByRegex(regexp.MustCompile(".*"))
	t.metrics.labelMeasurementNamesQueryTime(float64(time.Since(start) / time.Millisecond))
	return measurementNames
}

func (t *InfluxAdapter) findOrCreateShardForTimestamp(ts int64) uint64 {
	shardId := uint64(getShardStartForTimestamp(ts))
	_, existed := t.shards.LoadOrStore(shardId, struct{}{})

	if !existed {
		err := t.influx.CreateShard("db", "rp", shardId, true)
		if err != nil {
			panic(err)
		}
	}

	return shardId
}

func (t *InfluxAdapter) allShards() tsdb.ShardGroup {
	shardIds := t.ShardIDs()
	return t.influx.ShardGroup(shardIds)
}

func checkShardId(shardId uint64) {
	shardStart := int64(shardId)

	if shardStart != getShardStartForTimestamp(shardStart) {
		panic(fmt.Sprintf("misaligned shard ID: derived start time %d, expected %d", shardStart, getShardStartForTimestamp(shardStart)))
	}
}

func getShardStartForTimestamp(ts int64) int64 {
	return time.Unix(0, ts).Truncate(24 * time.Hour).UnixNano()
}

func getShardEndForTimestamp(ts int64) int64 {
	return time.Unix(0, ts).Truncate(24 * time.Hour).Add(24 * time.Hour).UnixNano()
}

func (t *InfluxAdapter) forTimestampRange(start, end int64) []uint64 {
	var shards []uint64

	t.shards.Range(func(shardIdKey interface{}, _ interface{}) bool {
		shardId := shardIdKey.(uint64)

		shardStart := int64(shardId)
		if shardStart < end && getShardEndForTimestamp(shardStart) >= start {
			shards = append(shards, shardId)
		}

		return true
	})

	return shards
}

func (t *InfluxAdapter) getOldest() uint64 {
	shards := t.forTimestampRange(0, time.Now().UnixNano())
	if len(shards) == 0 {
		// Attempting to delete a shard that doesn't exist is fine.
		return 0
	}

	min := shards[0]

	for _, shard := range shards {
		if shard < min {
			min = shard
		}
	}

	return min
}
