package persistence_test

import (
	"context"
	"errors"
	"regexp"
	"time"

	"github.com/cloudfoundry/metric-store-release/src/internal/metrics"
	"github.com/cloudfoundry/metric-store-release/src/pkg/logger"
	. "github.com/cloudfoundry/metric-store-release/src/pkg/persistence"
	"github.com/cloudfoundry/metric-store-release/src/pkg/rpc"
	"github.com/influxdata/influxdb/models"
	"github.com/influxdata/influxdb/query"
	"github.com/influxdata/influxdb/tsdb"
	"github.com/influxdata/influxql"

	"github.com/cloudfoundry/metric-store-release/src/internal/testing"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type influxAdapterTestContext struct {
	adapter     *InfluxAdapter
	influxStore *mockInfluxStore
	metrics     *testing.SpyMetricRegistrar
}

var _ = Describe("Influx Adapter", func() {
	var setup = func() *influxAdapterTestContext {
		influxStore := newMockInfluxStore([]uint64{
			getShardIdForDay(0),
			getShardIdForDay(1),
			getShardIdForDay(2),
			getShardIdForDay(3),
		})

		metrics := testing.NewSpyMetricRegistrar()

		return &influxAdapterTestContext{
			adapter:     NewInfluxAdapter(influxStore, metrics, logger.NewTestLogger(GinkgoWriter)),
			influxStore: influxStore,
			metrics:     metrics,
		}
	}

	Describe("WritePoints()", func() {
		It("writes data to correct shards", func() {
			tc := setup()
			points := []*rpc.Point{
				{Timestamp: nanosecondsInDays(0)},
				{Timestamp: nanosecondsInDays(1)},
			}
			tc.adapter.WritePoints(points)

			Expect(tc.influxStore.timestampsWrittenToShards).To(Equal(map[uint64][]time.Time{
				getShardIdForDay(0): {
					time.Unix(0, nanosecondsInDays(0)),
				},
				getShardIdForDay(1): {
					time.Unix(0, nanosecondsInDays(1)),
				},
			}))
		})

		It("creates shards as needed", func() {
			tc := setup()

			points := []*rpc.Point{
				{Timestamp: nanosecondsInDays(0)},
				{Timestamp: nanosecondsInDays(4)},
			}
			tc.adapter.WritePoints(points)

			Expect(tc.adapter.ShardIDs()).To(ContainElement(getShardIdForDay(4)))
		})

		It("surfaces any error returned by the store", func() {
			tc := setup()
			tc.influxStore.writeToShardError = errors.New("write-to-shard-error")

			points := []*rpc.Point{
				{Timestamp: nanosecondsInDays(0)},
			}
			err := tc.adapter.WritePoints(points)
			Expect(err).To(MatchError("write-to-shard-error"))
		})
	})

	Describe("GetPoints()", func() {
		// Happy path covered layer up in store
		It("returns an error when CreateIterators returns an error", func() {
			tc := setup()

			// The empty Shard will error when CreateIterators is called,
			// it is not an interface so this is the best we can do.
			tc.influxStore.shards = []*tsdb.Shard{
				{},
			}

			_, err := tc.adapter.GetPoints(context.Background(), "measurement-name", 0, 2, nil)
			Expect(err).To(MatchError("engine is closed"))
		})
	})

	Describe("AllMeasurementNames()", func() {
		// Happy path covered layer up in store
		It("measures the time for retrieving all measurement names", func() {
			tc := setup()
			tc.influxStore.setDelay(1 * time.Millisecond)

			tc.adapter.AllMeasurementNames()

			Expect(tc.metrics.FetchHistogram(metrics.MetricStoreMeasurementNamesQueryDurationSeconds)()).To(HaveLen(1))
		})
	})

	Describe("AllTagValues()", func() {
		// Happy path covered layer up in store
		It("measures the time for retrieving tag values", func() {
			tc := setup()
			tc.influxStore.setDelay(1 * time.Millisecond)

			tc.adapter.AllTagValues("source_id")

			Expect(tc.metrics.FetchHistogram(metrics.MetricStoreTagValuesQueryDurationSeconds)()).To(HaveLen(1))
		})
	})

	Describe("ShardIDs()", func() {
		It("returns all shard ids", func() {
			var expectedIds []uint64
			tc := setup()

			expectedIds = append(expectedIds, getShardIdForDay(0))
			expectedIds = append(expectedIds, getShardIdForDay(1))
			expectedIds = append(expectedIds, getShardIdForDay(2))
			expectedIds = append(expectedIds, getShardIdForDay(3))

			ids := tc.adapter.ShardIDs()

			Expect(ids).To(ConsistOf(expectedIds))
		})
	})

	Describe("Delete()", func() {
		It("can delete shards", func() {
			tc := setup()

			shardId := getShardIdForDay(0)
			Expect(tc.adapter.ShardIDs()).To(ContainElement(shardId))

			tc.adapter.Delete(shardId)

			Expect(tc.adapter.ShardIDs()).NotTo(ContainElement(shardId))
		})
	})

	Describe("DeleteOlderThan()", func() {
		It("deletes old shards when requested", func() {
			tc := setup()

			numDeleted, err := tc.adapter.DeleteOlderThan(nanosecondsInDays(0.5))
			Expect(err).ToNot(HaveOccurred())
			Expect(numDeleted).To(BeEquivalentTo(1))

			Expect(tc.adapter.ShardIDs()).To(ConsistOf(
				getShardIdForDay(1),
				getShardIdForDay(2),
				getShardIdForDay(3),
			))

			tc.adapter.DeleteOlderThan(nanosecondsInDays(0.5))
			Expect(tc.adapter.ShardIDs()).To(HaveLen(3))
		})

		It("allows a slight margin of error near day boundaries", func() {
			tc := setup()

			numDeleted, err := tc.adapter.DeleteOlderThan(nanosecondsInDays(1))
			Expect(err).ToNot(HaveOccurred())
			Expect(numDeleted).To(BeEquivalentTo(1))
			Expect(tc.adapter.ShardIDs()).To(HaveLen(3))

			numDeleted, err = tc.adapter.DeleteOlderThan(nanosecondsInDays(1) + int64(30*time.Minute))
			Expect(err).ToNot(HaveOccurred())
			Expect(numDeleted).To(BeEquivalentTo(1))
			Expect(tc.adapter.ShardIDs()).To(ConsistOf(
				getShardIdForDay(2),
				getShardIdForDay(3),
			))
		})
	})

	Describe("DeleteOldest()", func() {
		It("deletes the oldest shard", func() {
			tc := setup()

			tc.adapter.DeleteOldest()

			Expect(tc.adapter.ShardIDs()).To(ConsistOf(
				getShardIdForDay(3),
				getShardIdForDay(2),
				getShardIdForDay(1),
			))
		})
	})

	Describe("OldestShardID()", func() {
		It("returns the id the oldest shard", func() {
			tc := setup()

			oldestShardID, err := tc.adapter.OldestShardID()
			Expect(err).ToNot(HaveOccurred())
			Expect(oldestShardID).To(Equal(getShardIdForDay(0)))

			tc.adapter.DeleteOldest()

			oldestShardID, err = tc.adapter.OldestShardID()
			Expect(err).ToNot(HaveOccurred())
			Expect(oldestShardID).To(Equal(getShardIdForDay(1)))
		})

		It("returns an error when there are no shards", func() {
			tc := setup()

			tc.adapter.DeleteOldest()
			tc.adapter.DeleteOldest()
			tc.adapter.DeleteOldest()
			tc.adapter.DeleteOldest()

			_, err := tc.adapter.OldestShardID()
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("OldestContiguousShardID()", func() {
		It("returns the id the oldest contiguous shard", func() {
			influxStore := newMockInfluxStore([]uint64{
				getShardIdForDay(0),
				getShardIdForDay(2),
				getShardIdForDay(3),
				getShardIdForDay(4),
				getShardIdForDay(9),
			})

			metrics := testing.NewSpyMetricRegistrar()

			tc := &influxAdapterTestContext{
				adapter:     NewInfluxAdapter(influxStore, metrics, logger.NewTestLogger(GinkgoWriter)),
				influxStore: influxStore,
				metrics:     metrics,
			}

			oldestShardID, err := tc.adapter.OldestContiguousShardID()
			Expect(err).ToNot(HaveOccurred())
			Expect(oldestShardID).To(Equal(getShardIdForDay(9)))
		})

		It("returns the id the oldest contiguous shard", func() {
			influxStore := newMockInfluxStore([]uint64{
				getShardIdForDay(9),
			})

			metrics := testing.NewSpyMetricRegistrar()

			tc := &influxAdapterTestContext{
				adapter:     NewInfluxAdapter(influxStore, metrics, logger.NewTestLogger(GinkgoWriter)),
				influxStore: influxStore,
				metrics:     metrics,
			}

			oldestShardID, err := tc.adapter.OldestContiguousShardID()
			Expect(err).ToNot(HaveOccurred())
			Expect(oldestShardID).To(Equal(getShardIdForDay(9)))
		})

		It("returns an error when there are no shards", func() {
			influxStore := newMockInfluxStore([]uint64{})

			metrics := testing.NewSpyMetricRegistrar()

			tc := &influxAdapterTestContext{
				adapter:     NewInfluxAdapter(influxStore, metrics, logger.NewTestLogger(GinkgoWriter)),
				influxStore: influxStore,
				metrics:     metrics,
			}

			_, err := tc.adapter.OldestContiguousShardID()
			Expect(err).To(HaveOccurred())
		})

		It("ignores shards in the future", func() {
			influxStore := newMockInfluxStore([]uint64{
				getShardIdForDay(9),
				getShardIdInFuture(900),
			})

			metrics := testing.NewSpyMetricRegistrar()

			tc := &influxAdapterTestContext{
				adapter:     NewInfluxAdapter(influxStore, metrics, logger.NewTestLogger(GinkgoWriter)),
				influxStore: influxStore,
				metrics:     metrics,
			}

			oldestShardID, err := tc.adapter.OldestContiguousShardID()
			Expect(err).ToNot(HaveOccurred())
			Expect(oldestShardID).To(Equal(getShardIdForDay(9)))

		})

		It("errors if all shards are in future", func() {
			influxStore := newMockInfluxStore([]uint64{
				getShardIdInFuture(90),
				getShardIdInFuture(900),
			})

			metrics := testing.NewSpyMetricRegistrar()

			tc := &influxAdapterTestContext{
				adapter:     NewInfluxAdapter(influxStore, metrics, logger.NewTestLogger(GinkgoWriter)),
				influxStore: influxStore,
				metrics:     metrics,
			}

			_, err := tc.adapter.OldestContiguousShardID()
			Expect(err).To(HaveOccurred())
		})
	})
})

func getShardIdForDay(days uint64) uint64 {
	return uint64(nanosecondsInDays(float64(days)))
}

func getShardIdInFuture(days int64) uint64 {
	return uint64(time.Now().Add(time.Duration(days*24) * time.Hour).Truncate(time.Hour * 24).UnixNano())
}

// Returns the number of nanoseconds elapsed since days after the epoch.
func nanosecondsInDays(days float64) int64 {
	return int64(24*days) * int64(time.Hour)
}

type mockInfluxStore struct {
	shardIds                  []uint64
	timestampsWrittenToShards map[uint64][]time.Time
	writeToShardError         error
	shardGroup                tsdb.ShardGroup
	shards                    []*tsdb.Shard
	delay                     time.Duration
}

func newMockInfluxStore(shardIds []uint64) *mockInfluxStore {
	return &mockInfluxStore{
		shardIds:                  shardIds,
		timestampsWrittenToShards: make(map[uint64][]time.Time),
		writeToShardError:         nil,
		shardGroup:                newMockShardGroup(),
		shards:                    []*tsdb.Shard{},
		delay:                     0 * time.Millisecond,
	}
}

func (m *mockInfluxStore) setDelay(delay time.Duration) {
	m.delay = delay
	m.shardGroup = &mockShardGroup{
		delay: delay,
	}
}

func (m *mockInfluxStore) ShardIDs() []uint64 {
	return m.shardIds
}

func (m *mockInfluxStore) Shards(ids []uint64) []*tsdb.Shard {
	return m.shards
}

func (m *mockInfluxStore) WriteToShard(shardId uint64, points []models.Point) error {
	for _, point := range points {
		m.timestampsWrittenToShards[shardId] = append(
			m.timestampsWrittenToShards[shardId],
			point.Time(),
		)
	}

	return m.writeToShardError
}

func (m *mockInfluxStore) CreateShard(database string, retentionPolicy string, shardId uint64, enabled bool) error {
	Expect(enabled).To(BeTrue())

	return nil
}

func (m *mockInfluxStore) ShardGroup(shardIds []uint64) tsdb.ShardGroup {
	return m.shardGroup
}

func (m *mockInfluxStore) DeleteShard(shardId uint64) error {
	return nil
}

func (m *mockInfluxStore) TagKeys(auth query.Authorizer, shardIDs []uint64, cond influxql.Expr) ([]tsdb.TagKeys, error) {
	panic("not implemented")

}

func (m *mockInfluxStore) TagValues(auth query.Authorizer, shardIDs []uint64, cond influxql.Expr) ([]tsdb.TagValues, error) {
	time.Sleep(m.delay)
	return []tsdb.TagValues{}, nil

}

func (m *mockInfluxStore) Close() error {
	panic("not implemented")
}

func (m *mockInfluxStore) Statistics(tags map[string]string) []models.Statistic {
	return nil
}

func newMockShardGroup() *mockShardGroup {
	return &mockShardGroup{
		delay: 0 * time.Second,
	}
}

type mockShardGroup struct {
	measurementsNames    []string
	createIteratorError  error
	createIteratorResult query.Iterator
	delay                time.Duration
}

func (msg *mockShardGroup) MeasurementsByRegex(re *regexp.Regexp) []string {
	time.Sleep(msg.delay)
	return msg.measurementsNames
}

func (msg *mockShardGroup) FieldKeysByMeasurement(name []byte) []string {
	panic("not implemented")
}

func (msg *mockShardGroup) FieldDimensions(measurements []string) (fields map[string]influxql.DataType, dimensions map[string]struct{}, err error) {
	return map[string]influxql.DataType{}, map[string]struct{}{}, nil
}

func (msg *mockShardGroup) MapType(measurement string, field string) influxql.DataType {
	panic("not implemented")
}

func (msg *mockShardGroup) CreateIterator(ctx context.Context, measurement *influxql.Measurement, opt query.IteratorOptions) (query.Iterator, error) {
	return msg.createIteratorResult, msg.createIteratorError
}

func (msg *mockShardGroup) IteratorCost(measurement string, opt query.IteratorOptions) (query.IteratorCost, error) {
	panic("not implemented")
}

func (msg *mockShardGroup) ExpandSources(sources influxql.Sources) (influxql.Sources, error) {
	panic("not implemented")
}

type mockStringIterator struct {
	query.Iterator
	nextError error
}

func (msi *mockStringIterator) Next() (*query.StringPoint, error) {
	return nil, msi.nextError
}

func (msi *mockStringIterator) Close() error {
	return nil
}

type mockFloatIterator struct {
	query.Iterator
	nextError error
}

func (msi *mockFloatIterator) Next() (*query.FloatPoint, error) {
	return nil, msi.nextError
}

func (msi *mockFloatIterator) Close() error {
	return nil
}
