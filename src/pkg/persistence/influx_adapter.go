package persistence

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"sync"
	"time"

	"github.com/cloudfoundry/metric-store-release/src/internal/metrics"
	"github.com/cloudfoundry/metric-store-release/src/pkg/logger"
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
	Shards(ids []uint64) []*tsdb.Shard
	Statistics(database map[string]string) []models.Statistic
	TagKeys(auth query.Authorizer, shardIDs []uint64, cond influxql.Expr) ([]tsdb.TagKeys, error)
	TagValues(auth query.Authorizer, shardIDs []uint64, cond influxql.Expr) ([]tsdb.TagValues, error)
	WriteToShard(shardId uint64, points []models.Point) error
}

type InfluxAdapter struct {
	log     *logger.Logger
	metrics metrics.Registrar

	influx InfluxStore
	shards sync.Map
}

func NewInfluxAdapter(influx InfluxStore, metrics metrics.Registrar, log *logger.Logger) *InfluxAdapter {
	t := &InfluxAdapter{
		influx:  influx,
		metrics: metrics,
		log:     log,
	}

	for _, shardId := range influx.ShardIDs() {
		t.checkShardId(shardId)
		t.shards.Store(shardId, struct{}{})
	}

	return t
}

func (t *InfluxAdapter) Close() error {
	return t.influx.Close()
}

func (t *InfluxAdapter) WritePoints(points []*rpc.Point) error {
	pointBuckets := make(map[int64][]models.Point)
	influxPoints := transform.ToInfluxPoints(points)
	for _, point := range influxPoints {
		shardStart := getShardStartForTimestamp(point.Time().UnixNano())
		pointBuckets[shardStart] = append(pointBuckets[shardStart], point)
	}

	for bucketIndex, points := range pointBuckets {
		shardId := t.findOrCreateShardForTimestamp(bucketIndex)
		err := t.influx.WriteToShard(shardId, points)

		if err != nil {
			return err
		}
	}

	return nil
}

func (t *InfluxAdapter) GetPoints(ctx context.Context, measurementName string, start, end int64, matchers []*labels.Matcher) (*transform.SeriesSetBuilder, error) {
	shardIDs := t.forTimestampRange(start, end)
	shards := t.influx.ShardGroup(shardIDs)
	fieldSet, dimensionSet, err := shards.FieldDimensions([]string{measurementName})
	if err != nil {
		return nil, err
	}

	var fields, dimensions []string
	var auxFields []influxql.VarRef

	for field := range fieldSet {
		fields = append(fields, field)
		auxFields = append(auxFields, influxql.VarRef{Val: field})
	}
	for dimension := range dimensionSet {
		dimensions = append(dimensions, dimension)
	}

	filterCondition, err := transform.ToInfluxFilters(matchers)
	if err != nil {
		return nil, err
	}

	iteratorOptions := query.IteratorOptions{
		Expr:       influxql.MustParseExpr("value"),
		Aux:        auxFields,
		Dimensions: dimensions,
		Condition:  filterCondition,
		StartTime:  start,
		EndTime:    end,
		Ascending:  true,
		Ordered:    true,
		Limit:      0,
	}

	builder := transform.NewSeriesBuilder()

	wg := &sync.WaitGroup{}
	errChannel := make(chan error, len(shardIDs))
	for _, shard := range t.influx.Shards(shardIDs) {
		wg.Add(1)
		go func(builder *transform.SeriesSetBuilder, shard *tsdb.Shard) {
			labeledIterators, err := shard.CreateIterators(
				ctx,
				&influxql.Measurement{Name: measurementName},
				iteratorOptions,
			)
			if err != nil {
				errChannel <- err
				wg.Done()
				return
			}

			for _, labeledIterator := range labeledIterators {
				defer labeledIterator.Iterator.Close()

				typedIterator := labeledIterator.Iterator.(query.FloatIterator)
				defer typedIterator.Close()

				points := []*query.FloatPoint{}
				for {
					floatPoint, err := typedIterator.Next()
					if err != nil {
						errChannel <- err
						wg.Done()
						return
					}
					if floatPoint == nil {
						break
					}

					points = append(points, floatPoint.Clone())
				}
				builder.AddSeriesPoints(labeledIterator.Labels, points)
			}
			wg.Done()
		}(builder, shard)
	}

	wg.Wait()
	close(errChannel)
	if err := <-errChannel; err != nil {
		return nil, err
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

	t.metrics.Histogram(metrics.MetricStoreTagValuesQueryDurationSeconds).Observe(transform.DurationToSeconds(time.Since(start)))
	return values
}

func (t *InfluxAdapter) DeleteOldest() error {
	shardId, err := t.OldestShardID()
	if err != nil {
		return err
	}
	t.Delete(shardId)
	return t.influx.DeleteShard(shardId)
}

func (t *InfluxAdapter) DeleteOlderThan(cutoff int64) (uint64, error) {
	adjustedCutoff := time.Unix(0, cutoff).Add(-time.Minute).Truncate(24 * time.Hour).UnixNano()

	var deleted uint64
	for _, shardID := range t.ShardIDsOldestSort() {
		if int64(shardID) > adjustedCutoff {
			break
		}
		err := t.Delete(shardID)
		if err != nil {
			return deleted, err
		}

		deleted++
	}
	return deleted, nil
}

func (t *InfluxAdapter) Delete(shardID uint64) error {
	t.shards.Delete(shardID)
	return t.influx.DeleteShard(shardID)
}

type UintSlice []uint64

func (u UintSlice) Len() int           { return len(u) }
func (u UintSlice) Less(i, j int) bool { return u[i] < u[j] }
func (u UintSlice) Swap(i, j int)      { u[i], u[j] = u[j], u[i] }

// ShardIDs returns all shardIDs known to the adapter, sorted newest to oldest
func (t *InfluxAdapter) ShardIDs() []uint64 {
	var shards []uint64
	t.shards.Range(func(shardIdKey interface{}, _ interface{}) bool {
		shardId := shardIdKey.(uint64)
		shards = append(shards, shardId)
		return true
	})

	sort.Sort(sort.Reverse(UintSlice(shards)))
	return shards
}

func (t *InfluxAdapter) ShardIDsOldestSort() []uint64 {
	var shards []uint64
	t.shards.Range(func(shardIdKey interface{}, _ interface{}) bool {
		shardId := shardIdKey.(uint64)
		shards = append(shards, shardId)
		return true
	})

	sort.Sort(UintSlice(shards))
	return shards
}

// OldestShardID returns the absolute oldest shardID known to the adapter,
// regardless of potential time gaps between shards
func (t *InfluxAdapter) OldestShardID() (uint64, error) {
	shardIDs := t.ShardIDs()
	if len(shardIDs) == 0 {
		return 0, fmt.Errorf("Cannot determine oldest shardID when there are no shardIDs")
	}

	return shardIDs[len(shardIDs)-1], nil
}

func removeFutureShardIds(shardIds []uint64) []uint64 {
	nonFutureShards := make([]uint64, 0)

	cutoff := uint64(time.Now().Add(time.Hour * 24).UnixNano())
	for _, id := range shardIds {
		if id <= cutoff {
			nonFutureShards = append(nonFutureShards, id)
		}
	}

	return nonFutureShards
}

// OldestContiguousShardID returns the oldest shardID known to the adapter,
// that is part of the unbroken series closest to the newest shardID
func (t *InfluxAdapter) OldestContiguousShardID() (uint64, error) {
	shardIDs := removeFutureShardIds(t.ShardIDs())
	if len(shardIDs) == 0 {
		return 0, fmt.Errorf("Cannot determine oldest shardID when there are no shardIDs")
	}

	lastContiguousShardId := shardIDs[0]
	for i := 1; i < len(shardIDs); i++ {
		if (lastContiguousShardId - shardIDs[i]) == uint64(time.Hour*24) {
			lastContiguousShardId = shardIDs[i]
			continue
		}
		return lastContiguousShardId, nil
	}

	return lastContiguousShardId, nil
}

func (t *InfluxAdapter) AllMeasurementNames() []string {
	start := time.Now()
	allShards := t.influx.ShardGroup(t.ShardIDs())
	measurementNames := allShards.MeasurementsByRegex(regexp.MustCompile(".*"))
	t.metrics.Histogram(metrics.MetricStoreMeasurementNamesQueryDurationSeconds).Observe(transform.DurationToSeconds(time.Since(start)))
	return measurementNames
}

func (t *InfluxAdapter) findOrCreateShardForTimestamp(ts int64) uint64 {
	shardId := uint64(getShardStartForTimestamp(ts))
	_, existed := t.shards.LoadOrStore(shardId, struct{}{})

	if !existed {
		err := t.influx.CreateShard("db", "rp", shardId, true)
		if err != nil {
			t.log.Panic("error creating shard", logger.Error(err))
		}
	}

	return shardId
}

func (t *InfluxAdapter) checkShardId(shardId uint64) {
	shardStart := int64(shardId)

	if shardStart != getShardStartForTimestamp(shardStart) {
		t.log.Panic(
			"misaligned shard ID",
			logger.Int("derived start time", shardStart),
			logger.Int("expected start time", getShardStartForTimestamp(shardStart)),
		)
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

	sort.Sort(ShardIDs(shards))

	return shards
}
