package query_test

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/cloudfoundry/metric-store/pkg/query"
	rpc "github.com/cloudfoundry/metric-store/pkg/rpc/metricstore_v1"

	"github.com/cloudfoundry/metric-store/pkg/testing"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Engine", func() {
	Describe("InstantQuery()", func() {
		It("returns a scalar", func() {
			tc := engineSetup()
			req := &rpc.PromQL_InstantQueryRequest{Query: `7*9`, Time: `2.001`}

			r, err := tc.engine.InstantQuery(context.Background(), req, nil)
			Expect(err).ToNot(HaveOccurred())

			Expect(r.GetScalar().GetValue()).To(Equal(63.0))
			Expect(r.GetScalar().GetTime()).To(Equal(int64(2001)))
		})

		It("returns a vector", func() {
			tc := engineSetup()
			tc.spyDataReader.ReadErrs = []error{nil}
			tc.spyDataReader.ReadResults = []*rpc.PromQL_Matrix{{
				Series: []*rpc.PromQL_Series{{
					Metric: map[string]string{
						"__name__": "metric",
						"a":        "tag-a",
					},
					Points: []*rpc.PromQL_Point{
						{Time: 1001, Value: 99.0},
					},
				}},
			}}

			r, err := tc.engine.InstantQuery(
				context.Background(),
				&rpc.PromQL_InstantQueryRequest{Query: `metric`, Time: `2.001`},
				tc.spyDataReader,
			)
			Expect(err).ToNot(HaveOccurred())

			Expect(r.GetVector().GetSamples()).To(HaveLen(1))
			Expect(r.GetVector().GetSamples()).To(Equal([]*rpc.PromQL_Sample{
				{
					Metric: map[string]string{
						"__name__": "metric",
						"a":        "tag-a",
					},
					Point: &rpc.PromQL_Point{
						Time:  2001,
						Value: 99,
					},
				},
			}))

			Expect(tc.spyDataReader.ReadEnds[0]).To(Equal(int64(2001)))
			Expect(tc.spyDataReader.ReadStarts[0]).To(Equal(int64(2001 - (time.Minute * 5 / time.Millisecond))))
		})

		It("returns a vector for regex matching", func() {
			tc := engineSetup()
			tc.spyDataReader.ReadErrs = []error{nil}
			tc.spyDataReader.ReadResults = []*rpc.PromQL_Matrix{{
				Series: []*rpc.PromQL_Series{
					{
						Metric: map[string]string{
							"__name__":  "metric",
							"source_id": "some-id-1",
						},
						Points: []*rpc.PromQL_Point{
							{Time: 1001, Value: 99.0},
						},
					},
					{
						Metric: map[string]string{
							"__name__":  "metric",
							"source_id": "some-id-2",
						},
						Points: []*rpc.PromQL_Point{
							{Time: 1001, Value: 101},
						},
					},
				},
			}}

			result, err := tc.engine.InstantQuery(
				context.Background(),
				&rpc.PromQL_InstantQueryRequest{Query: `metric{source_id=~"some-id-1|some-id-2"}`, Time: "2"},
				tc.spyDataReader,
			)
			Expect(err).ToNot(HaveOccurred())

			Expect(func() []map[string]string {
				var metrics []map[string]string

				for _, sample := range result.GetVector().GetSamples() {
					metrics = append(metrics, sample.GetMetric())
				}

				return metrics
			}()).To(ConsistOf(
				HaveKeyWithValue("source_id", "some-id-1"),
				HaveKeyWithValue("source_id", "some-id-2"),
			))
		})

		It("returns a vector for a binary operation", func() {
			tc := engineSetup()
			tc.spyDataReader.ReadErrs = []error{nil, nil}
			tc.spyDataReader.ReadResults = []*rpc.PromQL_Matrix{
				{
					Series: []*rpc.PromQL_Series{
						{
							Metric: map[string]string{
								"__name__":  "metric",
								"a":         "tag-a",
								"b":         "tag-b",
								"source_id": "some-id-1",
							},
							Points: []*rpc.PromQL_Point{
								{Time: 1001, Value: 99.0},
							},
						},
					},
				},
				{
					Series: []*rpc.PromQL_Series{
						{
							Metric: map[string]string{
								"__name__":  "metric",
								"a":         "tag-a",
								"b":         "tag-b",
								"source_id": "some-id-2",
							},
							Points: []*rpc.PromQL_Point{
								{Time: 1002, Value: 101},
							},
						},
					},
				},
			}

			r, err := tc.engine.InstantQuery(
				context.Background(),
				&rpc.PromQL_InstantQueryRequest{
					Query: `metric{source_id="some-id-1"} + ignoring(source_id) metric{source_id="some-id-2"}`,
					Time:  `2.001`,
				},
				tc.spyDataReader,
			)
			Expect(err).ToNot(HaveOccurred())

			Expect(r.GetVector().GetSamples()).To(HaveLen(1))
			Expect(r.GetVector().GetSamples()).To(Equal([]*rpc.PromQL_Sample{
				{
					Metric: map[string]string{
						"a": "tag-a",
						"b": "tag-b",
					},
					Point: &rpc.PromQL_Point{
						Time:  2001,
						Value: 200,
					},
				},
			}))

			Expect(tc.spyDataReader.ReadEnds[0]).To(Equal(int64(2001)))
			Expect(tc.spyDataReader.ReadStarts[0]).To(Equal(int64(2001 - (time.Minute * 5 / time.Millisecond))))
		})

		It("returns a matrix", func() {
			tc := engineSetup()
			tc.spyDataReader.ReadErrs = []error{nil}
			tc.spyDataReader.ReadResults = []*rpc.PromQL_Matrix{{
				Series: []*rpc.PromQL_Series{{
					Metric: map[string]string{
						"__name__":  "metric",
						"a":         "tag-a",
						"b":         "tag-b",
						"source_id": "some-id-1",
					},
					Points: []*rpc.PromQL_Point{
						{Time: 1001, Value: 99.0},
					},
				}},
			}}

			r, err := tc.engine.InstantQuery(
				context.Background(),
				&rpc.PromQL_InstantQueryRequest{
					Query: `metric{source_id="some-id-1"}[5m]`,
					Time:  `2.001`,
				},
				tc.spyDataReader,
			)
			Expect(err).ToNot(HaveOccurred())

			Expect(r.GetMatrix().GetSeries()).To(Equal([]*rpc.PromQL_Series{
				{
					Metric: map[string]string{
						"__name__":  "metric",
						"a":         "tag-a",
						"b":         "tag-b",
						"source_id": "some-id-1",
					},
					Points: []*rpc.PromQL_Point{{
						Time:  1001,
						Value: 99,
					}},
				},
			}))

			Expect(tc.spyDataReader.ReadEnds[0]).To(Equal(int64(2001)))
			Expect(tc.spyDataReader.ReadStarts[0]).To(Equal(int64(2001 - (time.Minute * 5 / time.Millisecond))))
		})

		It("sets a request time to now when no time given", func() {
			tc := engineSetup()
			req := &rpc.PromQL_InstantQueryRequest{Query: `7*9`, Time: ""}

			r, err := tc.engine.InstantQuery(context.Background(), req, nil)
			Expect(err).ToNot(HaveOccurred())

			Expect(r.GetScalar().GetTime()).To(BeNumerically("~", time.Now().UnixNano()/int64(time.Millisecond), 10))
		})

		It("parses unix time as request time", func() {
			tc := engineSetup()
			now := time.Now().Unix()
			req := &rpc.PromQL_InstantQueryRequest{Query: `7*9`, Time: fmt.Sprintf("%d", now)}

			r, err := tc.engine.InstantQuery(context.Background(), req, nil)
			Expect(err).ToNot(HaveOccurred())

			Expect(r.GetScalar().GetTime()).To(BeNumerically("~", now*int64(time.Second/time.Millisecond), 10))
		})

		It("parses RFC3339 time as request time", func() {
			tc := engineSetup()
			ts := `2017-01-30T11:41:00-07:00`
			req := &rpc.PromQL_InstantQueryRequest{Query: `7*9`, Time: ts}

			r, err := tc.engine.InstantQuery(context.Background(), req, nil)
			Expect(err).ToNot(HaveOccurred())

			t, _ := time.Parse(time.RFC3339, ts)
			Expect(r.GetScalar().GetTime()).To(BeNumerically("~", t.UnixNano()/int64(time.Millisecond), 10))
		})

		It("captures the query time as a metric", func() {
			tc := engineSetup()
			tc.spyDataReader.ReadErrs = []error{nil}
			tc.spyDataReader.ReadResults = []*rpc.PromQL_Matrix{{}}

			_, err := tc.engine.InstantQuery(
				context.Background(),
				&rpc.PromQL_InstantQueryRequest{Query: `metric_name{}`},
				tc.spyDataReader,
			)
			Expect(err).ToNot(HaveOccurred())

			Expect(tc.spyMetrics.Get("metric_store_promql_instant_query_time")).ToNot(Equal(testing.UNDEFINED_METRIC))
			Expect(tc.spyMetrics.Get("metric_store_promql_instant_query_time")).ToNot(BeZero())
			Expect(tc.spyMetrics.GetUnit("metric_store_promql_instant_query_time")).To(Equal("milliseconds"))
		})

		It("surfaces error when request time parsing fails", func() {
			tc := engineSetup()
			ts := `2017-01-30 11:41:00-07:00`
			req := &rpc.PromQL_InstantQueryRequest{Query: `7*9`, Time: ts}

			_, err := tc.engine.InstantQuery(context.Background(), req, nil)
			Expect(err).To(HaveOccurred())
		})

		It("returns an error if the data reader fails", func() {
			tc := engineSetup()
			tc.spyDataReader.ReadResults = []*rpc.PromQL_Matrix{{}}
			tc.spyDataReader.ReadErrs = []error{errors.New("some-error")}

			_, err := tc.engine.InstantQuery(
				context.Background(),
				&rpc.PromQL_InstantQueryRequest{Query: `metric{source_id="some-id-1"}[5m]`},
				tc.spyDataReader,
			)
			Expect(err).To(HaveOccurred())

			Expect(tc.spyMetrics.Get("metric_store_promql_timeout")).To(BeEquivalentTo(1))
		})

		It("returns an error for an invalid query", func() {
			tc := engineSetup()
			_, err := tc.engine.InstantQuery(
				context.Background(),
				&rpc.PromQL_InstantQueryRequest{Query: `invalid.query`},
				nil,
			)
			Expect(err).To(HaveOccurred())
		})

		It("returns an error for a cancelled context", func() {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			tc := engineSetup()
			_, err := tc.engine.InstantQuery(
				ctx,
				&rpc.PromQL_InstantQueryRequest{Query: `metric{source_id="some-id-1"}[5m]`},
				nil,
			)

			Expect(err).To(HaveOccurred())
		})
	})

	Describe("RangeQuery()", func() {
		It("returns a matrix", func() {
			tc := engineSetup()
			tc.spyDataReader.ReadErrs = []error{nil}
			tc.spyDataReader.ReadResults = []*rpc.PromQL_Matrix{{
				Series: []*rpc.PromQL_Series{{
					Metric: map[string]string{
						"__name__":  "metric",
						"a":         "tag-a",
						"b":         "tag-b",
						"source_id": "some-id-1",
					},
					Points: []*rpc.PromQL_Point{
						{Time: 1001, Value: 99.0},
					},
				}},
			}}

			r, err := tc.engine.RangeQuery(
				context.Background(),
				&rpc.PromQL_RangeQueryRequest{
					Query: `metric{source_id="some-id-1"}`,
					Start: `0.001`,
					End:   `2.001`,
					Step:  `1s`,
				},
				tc.spyDataReader,
			)
			Expect(err).ToNot(HaveOccurred())

			Expect(r.GetMatrix().GetSeries()).To(Equal([]*rpc.PromQL_Series{
				{
					Metric: map[string]string{
						"__name__":  "metric",
						"a":         "tag-a",
						"b":         "tag-b",
						"source_id": "some-id-1",
					},
					Points: []*rpc.PromQL_Point{
						{
							Time:  1001,
							Value: 99,
						},
						{
							Time:  2001,
							Value: 99,
						},
					},
				},
			}))

			Expect(tc.spyDataReader.ReadEnds[0]).To(Equal(int64(2001)))
			Expect(tc.spyDataReader.ReadStarts[0]).To(Equal(int64(1 - (time.Minute * 5 / time.Millisecond))))
		})

		It("returns a matrix of aggregated values", func() {
			tc := engineSetup()
			lastHour := time.Now().Truncate(time.Hour).Add(-time.Hour)

			tc.spyDataReader.ReadErrs = []error{nil}
			tc.spyDataReader.ReadResults = []*rpc.PromQL_Matrix{{
				Series: []*rpc.PromQL_Series{{
					Metric: map[string]string{"__name__": "metric", "a": "tag-a", "b": "tag-b"},
					Points: []*rpc.PromQL_Point{
						{
							Time:  lastHour.Add(-time.Minute).UnixNano() / int64(time.Millisecond),
							Value: 97,
						},
						{
							Time:  lastHour.UnixNano() / int64(time.Millisecond),
							Value: 99,
						},
						{
							Time:  lastHour.Add(30*time.Second).UnixNano() / int64(time.Millisecond),
							Value: 101,
						},
						{
							Time:  lastHour.Add(45*time.Second).UnixNano() / int64(time.Millisecond),
							Value: 103,
						},
						{
							Time:  lastHour.Add(1*time.Minute).UnixNano() / int64(time.Millisecond),
							Value: 105,
						},
						{
							Time:  lastHour.Add(2*time.Minute).UnixNano() / int64(time.Millisecond),
							Value: 107,
						},
						{
							Time:  lastHour.Add(3*time.Minute).UnixNano() / int64(time.Millisecond),
							Value: 109,
						},
						{
							Time:  lastHour.Add(4*time.Minute).UnixNano() / int64(time.Millisecond),
							Value: 111,
						},
						{
							Time:  lastHour.Add(5*time.Minute).UnixNano() / int64(time.Millisecond),
							Value: 113,
						},
					},
				}},
			}}

			r, err := tc.engine.RangeQuery(
				context.Background(),
				&rpc.PromQL_RangeQueryRequest{
					Query: `avg_over_time(metric[1m])`,
					Start: testing.FormatTimeWithDecimalMillis(lastHour),
					End:   testing.FormatTimeWithDecimalMillis(lastHour.Add(5 * time.Minute)),
					Step:  "1m",
				},
				tc.spyDataReader,
			)
			Expect(err).ToNot(HaveOccurred())

			Expect(r.GetMatrix().GetSeries()).To(Equal([]*rpc.PromQL_Series{
				{
					Metric: map[string]string{
						"a": "tag-a",
						"b": "tag-b",
					},
					Points: []*rpc.PromQL_Point{
						{Time: lastHour.UnixNano() / int64(time.Millisecond), Value: 98},
						{Time: lastHour.Add(time.Minute).UnixNano() / int64(time.Millisecond), Value: 102},
						{Time: lastHour.Add(2*time.Minute).UnixNano() / int64(time.Millisecond), Value: 106},
						{Time: lastHour.Add(3*time.Minute).UnixNano() / int64(time.Millisecond), Value: 108},
						{Time: lastHour.Add(4*time.Minute).UnixNano() / int64(time.Millisecond), Value: 110},
						{Time: lastHour.Add(5*time.Minute).UnixNano() / int64(time.Millisecond), Value: 112},
					},
				},
			}))
		})

		It("returns a matrix of unaggregated values, choosing the latest value in the time window", func() {
			tc := engineSetup()
			lastHour := time.Now().Truncate(time.Hour).Add(-time.Hour)

			tc.spyDataReader.ReadErrs = []error{nil}
			tc.spyDataReader.ReadResults = []*rpc.PromQL_Matrix{
				{
					Series: []*rpc.PromQL_Series{{
						Metric: map[string]string{"__name__": "metric", "a": "tag-a", "b": "tag-b"},
						Points: []*rpc.PromQL_Point{
							{
								Value: 97,
								Time:  lastHour.Add(-time.Minute).UnixNano() / int64(time.Millisecond),
							},
							{
								Value: 101,
								Time:  lastHour.Add(30*time.Second).UnixNano() / int64(time.Millisecond),
							},
							{
								Value: 103,
								Time:  lastHour.Add(45*time.Second).UnixNano() / int64(time.Millisecond),
							},
							{
								Value: 105,
								Time:  lastHour.Add(3*time.Minute).UnixNano() / int64(time.Millisecond),
							},
							{
								Value: 111,
								Time:  lastHour.Add(5*time.Minute).UnixNano() / int64(time.Millisecond),
							},
							{
								Value: 113,
								Time:  lastHour.Add(6*time.Minute).UnixNano() / int64(time.Millisecond),
							},
						},
					}},
				},
			}

			r, err := tc.engine.RangeQuery(
				context.Background(),
				&rpc.PromQL_RangeQueryRequest{
					Query: `metric`,
					Start: testing.FormatTimeWithDecimalMillis(lastHour),
					End:   testing.FormatTimeWithDecimalMillis(lastHour.Add(5 * time.Minute)),
					Step:  "1m",
				},
				tc.spyDataReader,
			)
			Expect(err).ToNot(HaveOccurred())

			Expect(r.GetMatrix().GetSeries()).To(Equal([]*rpc.PromQL_Series{
				{
					Metric: map[string]string{
						"__name__": "metric",
						"a":        "tag-a",
						"b":        "tag-b",
					},
					Points: []*rpc.PromQL_Point{
						{Time: lastHour.UnixNano() / int64(time.Millisecond), Value: 97},
						{Time: lastHour.Add(time.Minute).UnixNano() / int64(time.Millisecond), Value: 103},
						{Time: lastHour.Add(2*time.Minute).UnixNano() / int64(time.Millisecond), Value: 103},
						{Time: lastHour.Add(3*time.Minute).UnixNano() / int64(time.Millisecond), Value: 105},
						{Time: lastHour.Add(4*time.Minute).UnixNano() / int64(time.Millisecond), Value: 105},
						{Time: lastHour.Add(5*time.Minute).UnixNano() / int64(time.Millisecond), Value: 111},
					},
				},
			}))
		})

		It("returns a matrix of unaggregated values, ordered by tag", func() {
			tc := engineSetup()
			lastHour := time.Now().Truncate(time.Hour).Add(-time.Hour)

			tc.spyDataReader.ReadErrs = []error{nil}
			tc.spyDataReader.ReadResults = []*rpc.PromQL_Matrix{
				{
					Series: []*rpc.PromQL_Series{
						{
							Metric: map[string]string{"__name__": "metric", "a": "tag-a", "b": "tag-b"},
							Points: []*rpc.PromQL_Point{
								{
									Value: 97,
									Time:  lastHour.UnixNano() / int64(time.Millisecond),
								},
								{
									Value: 113,
									Time:  lastHour.Add(2*time.Minute).UnixNano() / int64(time.Millisecond),
								},
							},
						},
						{
							Metric: map[string]string{"__name__": "metric", "a": "tag-a", "c": "tag-c"},
							Points: []*rpc.PromQL_Point{
								{
									Value: 101,
									Time:  lastHour.Add(1*time.Minute).UnixNano() / int64(time.Millisecond),
								},
							},
						},
					},
				},
			}

			r, err := tc.engine.RangeQuery(
				context.Background(),
				&rpc.PromQL_RangeQueryRequest{
					Query: `metric`,
					Start: testing.FormatTimeWithDecimalMillis(lastHour),
					End:   testing.FormatTimeWithDecimalMillis(lastHour.Add(5 * time.Minute)),
					Step:  "1m",
				},
				tc.spyDataReader,
			)
			Expect(err).ToNot(HaveOccurred())

			Expect(r.GetMatrix().GetSeries()).To(Equal([]*rpc.PromQL_Series{
				{
					Metric: map[string]string{"__name__": "metric", "a": "tag-a", "b": "tag-b"},
					Points: []*rpc.PromQL_Point{
						{Time: lastHour.UnixNano() / int64(time.Millisecond), Value: 97},
						{Time: lastHour.Add(time.Minute).UnixNano() / int64(time.Millisecond), Value: 97},
						{Time: lastHour.Add(2*time.Minute).UnixNano() / int64(time.Millisecond), Value: 113},
						{Time: lastHour.Add(3*time.Minute).UnixNano() / int64(time.Millisecond), Value: 113},
						{Time: lastHour.Add(4*time.Minute).UnixNano() / int64(time.Millisecond), Value: 113},
						{Time: lastHour.Add(5*time.Minute).UnixNano() / int64(time.Millisecond), Value: 113},
					},
				},
				{
					Metric: map[string]string{"__name__": "metric", "a": "tag-a", "c": "tag-c"},
					Points: []*rpc.PromQL_Point{
						{Time: lastHour.Add(time.Minute).UnixNano() / int64(time.Millisecond), Value: 101},
						{Time: lastHour.Add(2*time.Minute).UnixNano() / int64(time.Millisecond), Value: 101},
						{Time: lastHour.Add(3*time.Minute).UnixNano() / int64(time.Millisecond), Value: 101},
						{Time: lastHour.Add(4*time.Minute).UnixNano() / int64(time.Millisecond), Value: 101},
						{Time: lastHour.Add(5*time.Minute).UnixNano() / int64(time.Millisecond), Value: 101},
					},
				},
			}))
		})

		It("returns an error for an invalid query", func() {
			tc := engineSetup()
			_, err := tc.engine.RangeQuery(
				context.Background(),
				&rpc.PromQL_RangeQueryRequest{Query: `invalid.query`, Start: "1", End: "2", Step: "1m"},
				nil,
			)
			Expect(err).To(HaveOccurred())
		})

		It("returns an error for an invalid start time", func() {
			tc := engineSetup()
			_, err := tc.engine.RangeQuery(
				context.Background(),
				&rpc.PromQL_RangeQueryRequest{Query: `metric{source_id="some-id-1"}`, Start: "potato", End: "2", Step: "1m"},
				nil,
			)
			Expect(err).To(HaveOccurred())

			_, err = tc.engine.RangeQuery(
				context.Background(),
				&rpc.PromQL_RangeQueryRequest{Query: `metric{source_id="some-id-1"}`, Start: "", End: "2", Step: "1m"},
				nil,
			)
			Expect(err).To(HaveOccurred())
		})

		It("returns an error for an invalid end time", func() {
			tc := engineSetup()
			_, err := tc.engine.RangeQuery(
				context.Background(),
				&rpc.PromQL_RangeQueryRequest{Query: `metric{source_id="some-id-1"}`, Start: "1", End: "lemons", Step: "1m"},
				nil,
			)
			Expect(err).To(HaveOccurred())

			_, err = tc.engine.RangeQuery(
				context.Background(),
				&rpc.PromQL_RangeQueryRequest{Query: `metric{source_id="some-id-1"}`, Start: "1", End: "", Step: "1m"},
				nil,
			)
			Expect(err).To(HaveOccurred())
		})

		It("returns an error for an invalid step", func() {
			tc := engineSetup()
			_, err := tc.engine.RangeQuery(
				context.Background(),
				&rpc.PromQL_RangeQueryRequest{Query: `metric{source_id="some-id-1"}`, Start: "1", End: "3", Step: "1mim"},
				nil,
			)
			Expect(err).To(HaveOccurred())

			_, err = tc.engine.RangeQuery(
				context.Background(),
				&rpc.PromQL_RangeQueryRequest{Query: `metric{source_id="some-id-1"}`, Start: "1", End: "3", Step: ""},
				nil,
			)
			Expect(err).To(HaveOccurred())
		})

		It("accepts RFC3339 start and end times", func() {
			tc := engineSetup()
			tc.spyDataReader.ReadErrs = []error{nil}
			tc.spyDataReader.ReadResults = []*rpc.PromQL_Matrix{{}}

			_, err := tc.engine.RangeQuery(
				context.Background(),
				&rpc.PromQL_RangeQueryRequest{
					Query: `metric{source_id="some-id-1"}`,
					Start: "2099-01-01T01:23:45.678Z",
					End:   "2099-01-01T01:24:45.678Z",
					Step:  "1m",
				},
				tc.spyDataReader,
			)

			Expect(err).ToNot(HaveOccurred())
		})

		It("captures the query time as a metric", func() {
			tc := engineSetup()
			tc.spyDataReader.ReadErrs = []error{nil}
			tc.spyDataReader.ReadResults = []*rpc.PromQL_Matrix{{}}

			_, err := tc.engine.RangeQuery(
				context.Background(),
				&rpc.PromQL_RangeQueryRequest{Query: `metric{source_id="some-id-1"}`, Start: "1", End: "1", Step: "1m"},
				tc.spyDataReader,
			)

			Expect(err).ToNot(HaveOccurred())

			Expect(tc.spyMetrics.Get("metric_store_promql_range_query_time")).ToNot(Equal(testing.UNDEFINED_METRIC))
			Expect(tc.spyMetrics.Get("metric_store_promql_range_query_time")).ToNot(BeZero())
			Expect(tc.spyMetrics.GetUnit("metric_store_promql_range_query_time")).To(Equal("milliseconds"))
		})

		It("returns an error if the data reader fails", func() {
			tc := engineSetup()
			tc.spyDataReader.ReadResults = []*rpc.PromQL_Matrix{{}}
			tc.spyDataReader.ReadErrs = []error{errors.New("some-error")}

			_, err := tc.engine.RangeQuery(
				context.Background(),
				&rpc.PromQL_RangeQueryRequest{Query: `metric{source_id="some-id-1"}`, Start: "1", End: "2", Step: "1m"},
				tc.spyDataReader,
			)
			Expect(err).To(HaveOccurred())

			Expect(tc.spyMetrics.Get("metric_store_promql_timeout")).To(BeEquivalentTo(1))
		})

		It("returns an error for a cancelled context", func() {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			tc := engineSetup()
			tc.spyDataReader.ReadResults = []*rpc.PromQL_Matrix{{}}
			tc.spyDataReader.ReadErrs = []error{errors.New("some-error")}

			_, err := tc.engine.RangeQuery(
				ctx,
				&rpc.PromQL_RangeQueryRequest{Query: `metric{source_id="some-id-1"}`, Start: "1", End: "2", Step: "1m"},
				tc.spyDataReader,
			)

			Expect(err).To(HaveOccurred())
		})
	})

	Describe("SeriesQuery()", func() {
		It("returns a series", func() {
			tc := engineSetup()
			tc.spyDataReader.ReadErrs = []error{nil}
			tc.spyDataReader.ReadResults = []*rpc.PromQL_Matrix{{
				Series: []*rpc.PromQL_Series{
					{
						Metric: map[string]string{
							"__name__":  "metric_name",
							"a":         "tag-a",
							"source_id": "some-id-1",
						},
						Points: []*rpc.PromQL_Point{
							{Time: 1001, Value: 99.0},
						},
					},
					{
						Metric: map[string]string{
							"__name__":  "metric_name",
							"b":         "tag-b",
							"source_id": "some-id-1",
						},
						Points: []*rpc.PromQL_Point{
							{Time: 1500, Value: 99.0},
						},
					},
				},
			}}

			r, err := tc.engine.SeriesQuery(
				context.Background(),
				&rpc.PromQL_SeriesQueryRequest{
					Match: []string{`metric_name{source_id="some-id-1"}`},
					Start: "1",
					End:   "2",
				},
				tc.spyDataReader,
			)
			Expect(err).ToNot(HaveOccurred())

			Expect(r.GetSeries()).To(ConsistOf([]*rpc.PromQL_SeriesInfo{
				{
					Info: map[string]string{
						"__name__":  "metric_name",
						"a":         "tag-a",
						"source_id": "some-id-1",
					},
				},
				{
					Info: map[string]string{
						"__name__":  "metric_name",
						"b":         "tag-b",
						"source_id": "some-id-1",
					},
				},
			}))

			Expect(tc.spyDataReader.ReadEnds[0]).To(Equal(int64(2000)))
			Expect(tc.spyDataReader.ReadStarts[0]).To(Equal(int64(1000)))
		})

		It("returns an error for when no matchers given", func() {
			tc := engineSetup()
			_, err := tc.engine.SeriesQuery(
				context.Background(),
				&rpc.PromQL_SeriesQueryRequest{Match: []string{}, Start: "1", End: "2"},
				nil,
			)
			Expect(err).To(HaveOccurred())
		})

		It("returns an error for an invalid match", func() {
			tc := engineSetup()
			_, err := tc.engine.SeriesQuery(
				context.Background(),
				&rpc.PromQL_SeriesQueryRequest{Match: []string{`invalid.query`}, Start: "1", End: "2"},
				nil,
			)
			Expect(err).To(HaveOccurred())
		})

		It("returns an error for an invalid start time", func() {
			tc := engineSetup()
			_, err := tc.engine.SeriesQuery(
				context.Background(),
				&rpc.PromQL_SeriesQueryRequest{Match: []string{`metric{source_id="some-id-1"}`}, Start: "potato", End: "2"},
				nil,
			)
			Expect(err).To(HaveOccurred())

			_, err = tc.engine.SeriesQuery(
				context.Background(),
				&rpc.PromQL_SeriesQueryRequest{Match: []string{`metric{source_id="some-id-1"}`}, Start: "", End: "2"},
				nil,
			)
			Expect(err).To(HaveOccurred())
		})

		It("returns an error for an invalid end time", func() {
			tc := engineSetup()
			_, err := tc.engine.SeriesQuery(
				context.Background(),
				&rpc.PromQL_SeriesQueryRequest{Match: []string{`metric{source_id="some-id-1"}`}, Start: "1", End: "lemons"},
				nil,
			)
			Expect(err).To(HaveOccurred())

			_, err = tc.engine.SeriesQuery(
				context.Background(),
				&rpc.PromQL_SeriesQueryRequest{Match: []string{`metric{source_id="some-id-1"}`}, Start: "1", End: ""},
				nil,
			)
			Expect(err).To(HaveOccurred())
		})

		It("accepts RFC3339 start and end times", func() {
			tc := engineSetup()
			tc.spyDataReader.ReadErrs = []error{nil}
			tc.spyDataReader.ReadResults = []*rpc.PromQL_Matrix{{}}

			_, err := tc.engine.SeriesQuery(
				context.Background(),
				&rpc.PromQL_SeriesQueryRequest{
					Match: []string{`metric{source_id="some-id-1"}`},
					Start: "2099-01-01T01:23:45.678Z",
					End:   "2099-01-01T01:24:45.678Z",
				},
				tc.spyDataReader,
			)

			Expect(err).ToNot(HaveOccurred())
		})

		It("returns an error if the data reader fails", func() {
			tc := engineSetup()
			tc.spyDataReader.ReadResults = []*rpc.PromQL_Matrix{{}}
			tc.spyDataReader.ReadErrs = []error{errors.New("some-error")}

			_, err := tc.engine.SeriesQuery(
				context.Background(),
				&rpc.PromQL_SeriesQueryRequest{Match: []string{`metric{source_id="some-id-1"}`}, Start: "1", End: "2"},
				tc.spyDataReader,
			)
			Expect(err).To(HaveOccurred())
		})

		It("returns an error for a cancelled context", func() {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			tc := engineSetup()
			tc.spyDataReader.ReadResults = []*rpc.PromQL_Matrix{{}}
			tc.spyDataReader.ReadErrs = []error{errors.New("some-error")}

			_, err := tc.engine.SeriesQuery(
				ctx,
				&rpc.PromQL_SeriesQueryRequest{Match: []string{`metric{source_id="some-id-1"}`}, Start: "1", End: "2"},
				tc.spyDataReader,
			)

			Expect(err).To(HaveOccurred())
		})
	})
})

type engineTestContext struct {
	engine        *query.Engine
	spyDataReader *testing.SpyDataReader
	spyMetrics    *testing.SpyMetrics
}

func engineSetup() *engineTestContext {
	spyMetrics := testing.NewSpyMetrics()
	spyDataReader := testing.NewSpyDataReader()

	engine := query.NewEngine(
		spyMetrics,
		query.WithQueryTimeout(5*time.Second),
	)

	return &engineTestContext{
		engine:        engine,
		spyDataReader: spyDataReader,
		spyMetrics:    spyMetrics,
	}
}
