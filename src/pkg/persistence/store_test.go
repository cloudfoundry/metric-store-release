package persistence_test

import (
	"context"
	"io/ioutil"
	"os"
	"strings"
	"time"

	. "github.com/cloudfoundry/metric-store-release/src/pkg/persistence" // TEMP
	"github.com/prometheus/prometheus/pkg/labels"
	"github.com/prometheus/prometheus/storage"

	"github.com/influxdata/influxql"
	. "github.com/onsi/ginkgo"

	"github.com/cloudfoundry/metric-store-release/src/pkg/testing"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

type storeTestContext struct {
	store                 *Store
	querier               storage.Querier
	storagePath           string
	metrics               *testing.SpyMetrics
	minTimeInMilliseconds int64
	maxTimeInMilliseconds int64
}

const NO_LIMIT int = 0

var _ = Describe("Persistent Store", func() {
	var setup = func() storeTestContext {
		storagePath, err := ioutil.TempDir("", "metric-store")
		if err != nil {
			panic(err)
		}

		metrics := testing.NewSpyMetrics()

		influx, err := OpenTsStore(storagePath)
		Expect(err).ToNot(HaveOccurred())

		adapter := NewInfluxAdapter(influx, metrics)
		store := NewStore(
			adapter,
			metrics,
			WithAppenderLabelTruncationLength(64),
		)
		querier, _ := store.Querier(context.TODO(), 0, 0)

		return storeTestContext{
			metrics:               metrics,
			store:                 store,
			storagePath:           storagePath,
			minTimeInMilliseconds: influxql.MinTime / int64(time.Millisecond),
			maxTimeInMilliseconds: influxql.MaxTime / int64(time.Millisecond),
			querier:               querier,
		}
	}

	var teardown = func(tc storeTestContext) {
		tc.store.Close()
		os.RemoveAll(tc.storagePath)
	}

	Describe("Select()", func() {
		Context("when the metric has no extra fields", func() {
			It("fetches the point and its metadata", func() {
				tc := setup()
				defer teardown(tc)

				tc.storePointWithLabels(10, "counter", 1.0, map[string]string{"source_id": "source_id"})

				seriesSet, _, err := tc.querier.Select(
					&storage.SelectParams{Start: tc.minTimeInMilliseconds, End: tc.maxTimeInMilliseconds},
					&labels.Matcher{Name: "__name__", Value: "counter", Type: labels.MatchEqual},
				)
				Expect(err).ToNot(HaveOccurred())

				series := testing.ExplodeSeriesSet(seriesSet)
				Expect(series).To(ConsistOf(
					testing.Series{
						Labels: map[string]string{"__name__": "counter", "source_id": "source_id"},
						Points: []testing.Point{
							{Time: 10, Value: 1.0},
						},
					},
				))
			})
		})

		Context("when the metric has extra fields", func() {
			It("fetches the point and its metadata", func() {
				tc := setup()
				defer teardown(tc)

				tc.storePointWithLabels(10, "gauge", 1.5, map[string]string{
					"unit":       "units/interval",
					"deployment": "foobar",
					"source_id":  "source_id",
				})

				seriesSet, _, err := tc.querier.Select(
					&storage.SelectParams{Start: tc.minTimeInMilliseconds, End: tc.maxTimeInMilliseconds},
					&labels.Matcher{Name: "__name__", Value: "gauge", Type: labels.MatchEqual},
				)
				Expect(err).ToNot(HaveOccurred())

				series := testing.ExplodeSeriesSet(seriesSet)
				Expect(series).To(ConsistOf(
					testing.Series{
						Labels: map[string]string{
							"__name__":   "gauge",
							"source_id":  "source_id",
							"deployment": "foobar",
							"unit":       "units/interval",
						},
						Points: []testing.Point{
							{Time: 10, Value: 1.5},
						},
					},
				))
			})

			It("truncates all labels", func() {
				tc := setup()
				defer teardown(tc)

				tc.storePointWithLabels(10, "gauge", 1.5, map[string]string{
					"unit":       strings.Repeat("u", 1024),
					"deployment": strings.Repeat("d", 1024),
					"source_id":  "source_id",
				})

				seriesSet, _, err := tc.querier.Select(
					&storage.SelectParams{Start: tc.minTimeInMilliseconds, End: tc.maxTimeInMilliseconds},
					&labels.Matcher{Name: "__name__", Value: "gauge", Type: labels.MatchEqual},
				)
				Expect(err).ToNot(HaveOccurred())

				series := testing.ExplodeSeriesSet(seriesSet)
				Expect(series).To(ConsistOf(
					testing.Series{
						Labels: map[string]string{
							"__name__":   "gauge",
							"source_id":  "source_id",
							"deployment": strings.Repeat("d", 64),
							"unit":       strings.Repeat("u", 64),
						},
						Points: []testing.Point{
							{Time: 10, Value: 1.5},
						},
					},
				))
			})
		})

		DescribeTable("applies filtering operators correctly",
			func(operator labels.MatchType, expression string) {
				tc := setup()
				defer teardown(tc)

				tc.storeDefaultFilteringPoints()

				seriesSet, _, err := tc.querier.Select(
					&storage.SelectParams{Start: tc.minTimeInMilliseconds, End: tc.maxTimeInMilliseconds},
					&labels.Matcher{Name: "__name__", Value: "gauge", Type: labels.MatchEqual},
					&labels.Matcher{Name: "deployment", Value: expression, Type: operator},
				)
				Expect(err).ToNot(HaveOccurred())

				series := testing.ExplodeSeriesSet(seriesSet)[0]
				Expect(series.Points).To(ConsistOf(testing.Point{Time: 10, Value: 1.5}))
				Expect(series.Labels).To(HaveKeyWithValue("__name__", "gauge"))
			},
			Entry("NEQ", labels.MatchNotEqual, "der-hofbrau"),
			Entry("REGEX", labels.MatchRegexp, "^.*schnitzel$"),
			Entry("NREGEX", labels.MatchNotRegexp, "^.*hofbrau$"),
		)

		It("applies multiple filters correctly", func() {
			tc := setup()
			defer teardown(tc)

			tc.storeDefaultFilteringPoints()

			seriesSet, _, err := tc.querier.Select(
				&storage.SelectParams{Start: tc.minTimeInMilliseconds, End: tc.maxTimeInMilliseconds},
				&labels.Matcher{Name: "__name__", Value: "gauge", Type: labels.MatchEqual},
				&labels.Matcher{Name: "deployment", Value: "der-schnitzel", Type: labels.MatchEqual},
				&labels.Matcher{Name: "unit", Value: "microns", Type: labels.MatchEqual},
				&labels.Matcher{Name: "fake", Value: "true", Type: labels.MatchEqual},
			)
			Expect(err).ToNot(HaveOccurred())

			series := testing.ExplodeSeriesSet(seriesSet)
			Expect(series).To(ConsistOf(
				testing.Series{
					Labels: map[string]string{
						"__name__":   "gauge",
						"source_id":  "source_id_1",
						"deployment": "der-schnitzel",
						"unit":       "microns",
						"fake":       "true",
					},
					Points: []testing.Point{
						{Time: 10, Value: 1.5},
					},
				},
			))
		})

		It("fetches data with respect to time filtering", func() {
			tc := setup()
			defer teardown(tc)

			tc.storePoint(10, "counter", 1)
			tc.storePoint(20, "counter", 2)
			tc.storePoint(30, "counter", 3)
			tc.storePoint(40, "counter", 4)

			seriesSet, _, err := tc.querier.Select(
				&storage.SelectParams{Start: 10, End: 30},
				&labels.Matcher{Name: "__name__", Value: "counter", Type: labels.MatchEqual},
			)
			Expect(err).ToNot(HaveOccurred())

			series := testing.ExplodeSeriesSet(seriesSet)
			Expect(series).To(ConsistOf(
				testing.Series{
					Labels: map[string]string{"__name__": "counter"},
					Points: []testing.Point{
						{Time: 10, Value: 1},
						{Time: 20, Value: 2},
					},
				},
			))
		})

		It("fetches data with respect to metric name filtering", func() {
			tc := setup()
			defer teardown(tc)

			tc.storePoint(10, "cpu", 1)
			tc.storePoint(20, "memory", 2)

			seriesSet, _, err := tc.querier.Select(
				&storage.SelectParams{Start: tc.minTimeInMilliseconds, End: tc.maxTimeInMilliseconds},
				&labels.Matcher{Name: "__name__", Value: "cpu", Type: labels.MatchEqual},
			)
			Expect(err).ToNot(HaveOccurred())

			series := testing.ExplodeSeriesSet(seriesSet)
			Expect(series).To(ConsistOf(
				testing.Series{
					Labels: map[string]string{"__name__": "cpu"},
					Points: []testing.Point{
						{Time: 10, Value: 1},
					},
				},
			))
		})

		It("defaults Start to 0, End to now for nil params", func() {
			tc := setup()
			defer teardown(tc)

			now := time.Now().UnixNano() / int64(time.Millisecond)
			tc.storePoint(1, "point-to-test-nil-default", 1)
			tc.storePoint(now, "point-to-test-nil-default", 2)

			seriesSet, _, err := tc.querier.Select(
				nil,
				&labels.Matcher{Name: "__name__", Value: "point-to-test-nil-default", Type: labels.MatchEqual},
			)
			Expect(err).ToNot(HaveOccurred())

			series := testing.ExplodeSeriesSet(seriesSet)
			Expect(series).To(ConsistOf(
				testing.Series{
					Labels: map[string]string{"__name__": "point-to-test-nil-default"},
					Points: []testing.Point{
						{Time: 1, Value: 1},
						{Time: now, Value: 2},
					},
				},
			))
		})

		It("defaults Start to 0, End to now for empty params", func() {
			tc := setup()
			defer teardown(tc)

			now := time.Now().UnixNano() / int64(time.Millisecond)
			tc.storePoint(1, "point-to-test-empty-default", 1)
			tc.storePoint(now, "point-to-test-empty-default", 2)

			seriesSet, _, err := tc.querier.Select(
				&storage.SelectParams{},
				&labels.Matcher{Name: "__name__", Value: "point-to-test-empty-default", Type: labels.MatchEqual},
			)
			Expect(err).ToNot(HaveOccurred())

			series := testing.ExplodeSeriesSet(seriesSet)
			Expect(series).To(ConsistOf(
				testing.Series{
					Labels: map[string]string{"__name__": "point-to-test-empty-default"},
					Points: []testing.Point{
						{Time: 1, Value: 1},
						{Time: now, Value: 2},
					},
				},
			))
		})

		It("accepts query with a start time but without an end time", func() {
			tc := setup()
			defer teardown(tc)

			now := time.Now().UnixNano() / int64(time.Millisecond)
			tc.storePoint(1, "point-to-test-end-default", 1)
			tc.storePoint(now, "point-to-test-end-default", 2)

			seriesSet, _, err := tc.querier.Select(
				&storage.SelectParams{Start: 0},
				&labels.Matcher{Name: "__name__", Value: "point-to-test-end-default", Type: labels.MatchEqual},
			)
			Expect(err).ToNot(HaveOccurred())

			series := testing.ExplodeSeriesSet(seriesSet)
			Expect(series).To(ConsistOf(
				testing.Series{
					Labels: map[string]string{"__name__": "point-to-test-end-default"},
					Points: []testing.Point{
						{Time: 1, Value: 1},
						{Time: now, Value: 2},
					},
				},
			))
		})

		It("returns an empty set when an invalid query is provided", func() {
			tc := setup()
			defer teardown(tc)

			seriesSet, _, err := tc.querier.Select(
				&storage.SelectParams{Start: tc.minTimeInMilliseconds, End: tc.maxTimeInMilliseconds},
				&labels.Matcher{Name: "__name__", Value: "i-definitely-do-not-exist", Type: labels.MatchEqual},
			)
			Expect(err).ToNot(HaveOccurred())

			series := testing.ExplodeSeriesSet(seriesSet)
			Expect(series).To(HaveLen(0))
		})
	})

	Describe("LabelNames()", func() {
		It("returns labels that are stored as tags", func() {
			tc := setup()
			defer teardown(tc)

			tc.storePointWithLabels(1, "metric-one", 1, map[string]string{
				"source_id": "1", "ip": "1",
			})
			tc.storePointWithLabels(1, "metric-two", 1, map[string]string{
				"source_id": "1", "job": "1",
			})

			res, err := tc.querier.LabelNames()
			Expect(err).ToNot(HaveOccurred())
			Expect(res).To(ConsistOf("source_id", "ip", "job"))
		})
	})

	Describe("LabelValues()", func() {
		It("returns values for a given (tag) label", func() {
			tc := setup()
			defer teardown(tc)

			tc.storePointWithLabels(1, "metric-one", 1, map[string]string{
				"source_id": "1",
			})
			tc.storePointWithLabels(1, "metric-two", 1, map[string]string{
				"source_id": "10",
			})

			res, err := tc.querier.LabelValues("source_id")
			Expect(err).ToNot(HaveOccurred())
			Expect(res).To(ConsistOf("1", "10"))
		})

		It("returns all measurement names for label __name__", func() {
			tc := setup()
			defer teardown(tc)

			tc.storePointWithLabels(1, "metric-one", 1, map[string]string{
				"user_agent": "1",
				"source_id":  "1",
			})
			tc.storePointWithLabels(2, "metric-one", 1, map[string]string{
				"ip": "0.0.0.0",
			})
			tc.storePointWithLabels(3, "metric-two", 1, map[string]string{
				"user_agent": "10",
			})

			res, err := tc.querier.LabelValues("__name__")
			Expect(err).ToNot(HaveOccurred())
			Expect(res).To(ConsistOf("metric-one", "metric-two"))
		})
	})

	Describe("automatic expiry", func() {
		It("truncates points that are older than a pre-defined expiration time", func() {
			tc := setup()
			defer teardown(tc)

			today := time.Now().Truncate(24 * time.Hour)

			todayInMilliseconds := today.UnixNano() / int64(time.Millisecond)
			oneHourBeforeTodayInMilliseconds := today.Add(-time.Hour).UnixNano() / int64(time.Millisecond)
			oneDayAgo := today.Add(-24 * time.Hour)

			tc.storePoint(1, "counter", 1)
			tc.storePoint(todayInMilliseconds, "counter", 3)
			tc.storePoint(oneHourBeforeTodayInMilliseconds, "counter", 2)
			tc.store.DeleteOlderThan(oneDayAgo)
			Expect(tc.metrics.Getter("metric_store_num_shards_expired")()).To(BeEquivalentTo(1))

			seriesSet, _, err := tc.querier.Select(
				&storage.SelectParams{Start: tc.minTimeInMilliseconds, End: tc.maxTimeInMilliseconds},
				&labels.Matcher{Name: "__name__", Value: "counter", Type: labels.MatchEqual},
			)
			Expect(err).ToNot(HaveOccurred())

			series := testing.ExplodeSeriesSet(seriesSet)
			Expect(series).To(ConsistOf(
				testing.Series{
					Labels: map[string]string{"__name__": "counter"},
					Points: []testing.Point{
						{Time: oneHourBeforeTodayInMilliseconds, Value: 2},
						{Time: todayInMilliseconds, Value: 3},
					},
				},
			))
		})

		It("truncates oldest points", func() {
			tc := setup()
			defer teardown(tc)

			now := time.Now()
			nowInMilliseconds := now.UnixNano() / int64(time.Millisecond)

			tc.storePoint(1, "counter", 1)
			tc.storePoint(nowInMilliseconds, "counter", 2)
			tc.store.DeleteOldest()
			Expect(tc.metrics.Getter("metric_store_num_shards_pruned")()).To(BeEquivalentTo(1))

			seriesSet, _, err := tc.querier.Select(
				&storage.SelectParams{Start: tc.minTimeInMilliseconds, End: tc.maxTimeInMilliseconds},
				&labels.Matcher{Name: "__name__", Value: "counter", Type: labels.MatchEqual},
			)
			Expect(err).ToNot(HaveOccurred())

			series := testing.ExplodeSeriesSet(seriesSet)
			Expect(series).To(ConsistOf(
				testing.Series{
					Labels: map[string]string{"__name__": "counter"},
					Points: []testing.Point{
						{Time: nowInMilliseconds, Value: 2},
					},
				},
			))
		})
	})

	Describe("instrumentation", func() {
		It("updates ingress metrics", func() {
			tc := setup()
			defer teardown(tc)

			tc.storePoint(1, "any_counter", 1)

			Expect(tc.metrics.Get("metric_store_ingress")).To(BeEquivalentTo(1))
		})

		It("updates storage duration metrics", func() {
			tc := setup()
			defer teardown(tc)

			today := time.Now().Truncate(24 * time.Hour)
			todayInMilliseconds := today.UnixNano() / int64(time.Millisecond)
			oneDayAgo := today.Add(-1 * 24 * time.Hour)
			oneDayAgoInMilliseconds := oneDayAgo.UnixNano() / int64(time.Millisecond)
			threeDaysAgo := today.Add(-3 * 24 * time.Hour)
			threeDaysAgoInMilliseconds := threeDaysAgo.UnixNano() / int64(time.Millisecond)

			tc.storePoint(todayInMilliseconds, "counter", 1)

			tc.store.EmitStorageDurationMetric()
			Expect(tc.metrics.Getter("metric_store_storage_duration")()).To(BeEquivalentTo(0))

			tc.storePoint(oneDayAgoInMilliseconds, "counter", 1)

			tc.store.EmitStorageDurationMetric()
			Expect(tc.metrics.Getter("metric_store_storage_duration")()).To(BeEquivalentTo(1))

			tc.storePoint(threeDaysAgoInMilliseconds, "counter", 1)

			tc.store.EmitStorageDurationMetric()
			Expect(tc.metrics.Getter("metric_store_storage_duration")()).To(BeEquivalentTo(3))
		})
	})
})

func (tc *storeTestContext) storePoint(ts int64, name string, value float64) {
	tc.storePointWithLabels(ts, name, value, nil)
}

func (tc *storeTestContext) storePointWithLabels(ts int64, name string, value float64, addLabels map[string]string) {
	// point := &rpc.Point{
	// 	Name:      name,
	// 	Timestamp: ts * int64(time.Millisecond),
	// 	Value:     value,
	// 	Labels:    labels,
	// }

	// tc.store.Put([]*rpc.Point{point})

	appender, _ := tc.store.Appender()
	pointLabels := labels.FromMap(addLabels)
	pointLabels = append(pointLabels, labels.Label{Name: "__name__", Value: name})
	appender.Add(pointLabels, ts*int64(time.Millisecond), value)
	appender.Commit()
}

func (tc *storeTestContext) storeDefaultFilteringPoints() {
	tc.storePointWithLabels(10, "gauge", 1.5, map[string]string{
		"unit":       "microns",
		"deployment": "der-schnitzel",
		"fake":       "true",
		"source_id":  "source_id_1",
	})
	tc.storePointWithLabels(20, "gauge", 3.0, map[string]string{
		"unit":       "microns",
		"deployment": "der-hofbrau",
		"fake":       "true",
		"source_id":  "source_id_2",
	})
	tc.storePointWithLabels(30, "gauge", 4.5, map[string]string{
		"unit":       "nanoseconds",
		"deployment": "der-hofbrau",
		"fake":       "true",
		"source_id":  "source_id_2",
	})
	tc.storePointWithLabels(30, "gauge", 6.0, map[string]string{
		"unit":       "nanoseconds",
		"deployment": "der-hofbrau",
		"fake":       "nope",
		"source_id":  "source_id_2",
	})
}
