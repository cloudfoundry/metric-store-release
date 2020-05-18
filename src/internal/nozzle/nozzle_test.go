package nozzle_test

import (
	"sync"
	"time"

	"github.com/cloudfoundry/metric-store-release/src/internal/debug"
	. "github.com/cloudfoundry/metric-store-release/src/internal/nozzle"
	"github.com/cloudfoundry/metric-store-release/src/pkg/logger"
	"github.com/cloudfoundry/metric-store-release/src/pkg/rpc"

	loggregator "code.cloudfoundry.org/go-loggregator"
	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	sharedtls "github.com/cloudfoundry/metric-store-release/src/internal/tls"
	"golang.org/x/net/context"

	. "github.com/cloudfoundry/metric-store-release/src/internal/matchers"
	"github.com/cloudfoundry/metric-store-release/src/internal/testing"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Nozzle", func() {
	var (
		n               *Nozzle
		streamConnector *spyStreamConnector
		metricStore     *testing.SpyMetricStore
		metricRegistrar *testing.SpyMetricRegistrar
	)

	BeforeEach(func() {
		tlsServerConfig, err := sharedtls.NewMutualTLSServerConfig(
			testing.Cert("metric-store-ca.crt"),
			testing.Cert("metric-store.crt"),
			testing.Cert("metric-store.key"),
		)
		Expect(err).ToNot(HaveOccurred())

		tlsClientConfig, err := sharedtls.NewMutualTLSClientConfig(
			testing.Cert("metric-store-ca.crt"),
			testing.Cert("metric-store.crt"),
			testing.Cert("metric-store.key"),
			"metric-store",
		)
		Expect(err).ToNot(HaveOccurred())

		streamConnector = newSpyStreamConnector()
		metricRegistrar = testing.NewSpyMetricRegistrar()
		metricStore = testing.NewSpyMetricStore(tlsServerConfig)
		addrs := metricStore.Start()

		n = NewNozzle(streamConnector, addrs.EgressAddr, addrs.IngressAddr, tlsClientConfig, "metric-store", 0,
			WithNozzleDebugRegistrar(metricRegistrar),
			WithNozzleTimerRollup(
				100*time.Millisecond,
				[]string{"tag1", "tag2", "status_code"},
				[]string{"tag1", "tag2"},
			),
			WithNozzleLogger(logger.NewTestLogger(GinkgoWriter)),
		)
		go n.Start()
	})

	AfterEach(func() {
		metricStore.Stop()
	})

	It("connects and reads from a logs provider server", func() {
		addEnvelope(1, "memory", "some-source-id", streamConnector)
		addEnvelope(2, "memory", "some-source-id", streamConnector)
		addEnvelope(3, "memory", "some-source-id", streamConnector)

		Eventually(streamConnector.requests).Should(HaveLen(1))
		Expect(streamConnector.requests()[0].ShardId).To(Equal("metric-store"))
		Expect(streamConnector.requests()[0].UsePreferredTags).To(BeTrue())
		Expect(streamConnector.requests()[0].Selectors).To(HaveLen(3))

		Expect(streamConnector.requests()[0].Selectors).To(ConsistOf(
			[]*loggregator_v2.Selector{
				{
					Message: &loggregator_v2.Selector_Gauge{
						Gauge: &loggregator_v2.GaugeSelector{},
					},
				},
				{
					Message: &loggregator_v2.Selector_Counter{
						Counter: &loggregator_v2.CounterSelector{},
					},
				},
				{
					Message: &loggregator_v2.Selector_Timer{
						Timer: &loggregator_v2.TimerSelector{},
					},
				},
			},
		))

		Eventually(streamConnector.envelopes).Should(HaveLen(0))
	})

	It("writes each envelope as a point to the metric-store", func() {
		addEnvelope(1, "memory", "some-source-id", streamConnector)
		addEnvelope(2, "memory", "some-source-id", streamConnector)
		addEnvelope(3, "memory", "some-source-id", streamConnector)

		Eventually(metricStore.GetPoints).Should(HaveLen(3))
		Expect(metricStore.GetPoints()).To(ContainPoints([]*rpc.Point{
			{
				Name:      "memory",
				Timestamp: 1,
				Value:     0,
				Labels: map[string]string{
					"source_id": "some-source-id",
				},
			},
			{
				Name:      "memory",
				Timestamp: 2,
				Value:     0,
				Labels: map[string]string{
					"source_id": "some-source-id",
				},
			},
			{
				Name:      "memory",
				Timestamp: 3,
				Value:     0,
				Labels: map[string]string{
					"source_id": "some-source-id",
				},
			},
		}))
	})

	Describe("when the envelope is a Timer", func() {
		It("rolls up configured metrics", func() {
			intervalStart := time.Now().Truncate(100 * time.Millisecond).UnixNano()

			streamConnector.envelopes <- []*loggregator_v2.Envelope{
				{
					Timestamp: intervalStart + 1,
					SourceId:  "source-id",
					Message: &loggregator_v2.Envelope_Timer{
						Timer: &loggregator_v2.Timer{
							Name:  "http",
							Start: 0,
							Stop:  5 * int64(time.Millisecond),
						},
					},
					Tags: map[string]string{
						"tag1":        "t1",
						"tag2":        "t2",
						"status_code": "500",
					},
				},
				{
					Timestamp: intervalStart + 2,
					SourceId:  "source-id",
					Message: &loggregator_v2.Envelope_Timer{
						Timer: &loggregator_v2.Timer{
							Name:  "http",
							Start: 5 * int64(time.Millisecond),
							Stop:  100 * int64(time.Millisecond),
						},
					},
					Tags: map[string]string{
						"tag1":        "t1",
						"tag2":        "t2",
						"status_code": "200",
					},
				},
				{
					Timestamp: intervalStart + 3,
					SourceId:  "source-id-2",
					Message: &loggregator_v2.Envelope_Timer{
						Timer: &loggregator_v2.Timer{
							Name:  "http",
							Start: 100 * int64(time.Millisecond),
							Stop:  106 * int64(time.Millisecond),
						},
					},
					Tags: map[string]string{
						"peer_type":   "Server",
						"status_code": "200",
					},
				},
				{
					Timestamp: intervalStart + 4,
					SourceId:  "source-id-2",
					Message: &loggregator_v2.Envelope_Timer{
						Timer: &loggregator_v2.Timer{
							Name:  "http",
							Start: 96 * int64(time.Millisecond),
							Stop:  100 * int64(time.Millisecond),
						},
					},
					Tags: map[string]string{
						"peer_type":   "Server",
						"status_code": "200",
					},
				},
				{
					Timestamp: intervalStart + 5,
					SourceId:  "source-id-2",
					Message: &loggregator_v2.Envelope_Timer{
						Timer: &loggregator_v2.Timer{
							Name:  "http",
							Start: 500 * int64(time.Millisecond),
							Stop:  1000 * int64(time.Millisecond),
						},
					},
					Tags: map[string]string{
						"peer_type":   "Server",
						"status_code": "400",
					},
				},
			}

			numberOfExpectedSeriesIncludingStatusCode := 4
			numberOfExpectedSeriesExcludingStatusCode := 2
			numberOfExpectedPoints := numberOfExpectedSeriesIncludingStatusCode + (14 * numberOfExpectedSeriesExcludingStatusCode)
			Eventually(metricStore.GetPoints).Should(HaveLen(numberOfExpectedPoints))

			points := metricStore.GetPoints()

			// _count points, per series including status_code
			Expect(points).To(ContainPoints([]*rpc.Point{
				{
					Name:  "http_total",
					Value: 1,
					Labels: map[string]string{
						"node_index":  "0",
						"source_id":   "source-id",
						"tag1":        "t1",
						"tag2":        "t2",
						"status_code": "500",
					},
				},
				{
					Name:  "http_total",
					Value: 1,
					Labels: map[string]string{
						"node_index":  "0",
						"source_id":   "source-id",
						"tag1":        "t1",
						"tag2":        "t2",
						"status_code": "200",
					},
				},
				{
					Name:  "http_total",
					Value: 2,
					Labels: map[string]string{
						"node_index":  "0",
						"source_id":   "source-id-2",
						"status_code": "200",
					},
				},
				{
					Name:  "http_total",
					Value: 1,
					Labels: map[string]string{
						"node_index":  "0",
						"source_id":   "source-id-2",
						"status_code": "400",
					},
				},
			}))

			// _duration_seconds histogram points, per series excluding status_code
			// only testing one series
			Expect(points).To(ContainPoints([]*rpc.Point{
				{
					Name:  "http_duration_seconds_bucket",
					Value: 1,
					Labels: map[string]string{
						"node_index": "0",
						"source_id":  "source-id",
						"tag1":       "t1",
						"tag2":       "t2",
						"le":         "0.005",
					},
				},
				{
					Name:  "http_duration_seconds_bucket",
					Value: 1,
					Labels: map[string]string{
						"node_index": "0",
						"source_id":  "source-id",
						"tag1":       "t1",
						"tag2":       "t2",
						"le":         "0.01",
					},
				},
				{
					Name:  "http_duration_seconds_bucket",
					Value: 1,
					Labels: map[string]string{
						"node_index": "0",
						"source_id":  "source-id",
						"tag1":       "t1",
						"tag2":       "t2",
						"le":         "0.025",
					},
				},
				{
					Name:  "http_duration_seconds_bucket",
					Value: 1,
					Labels: map[string]string{
						"node_index": "0",
						"source_id":  "source-id",
						"tag1":       "t1",
						"tag2":       "t2",
						"le":         "0.05",
					},
				},
				{
					Name:  "http_duration_seconds_bucket",
					Value: 2,
					Labels: map[string]string{
						"node_index": "0",
						"source_id":  "source-id",
						"tag1":       "t1",
						"tag2":       "t2",
						"le":         "0.1",
					},
				},
				{
					Name:  "http_duration_seconds_bucket",
					Value: 2,
					Labels: map[string]string{
						"node_index": "0",
						"source_id":  "source-id",
						"tag1":       "t1",
						"tag2":       "t2",
						"le":         "0.25",
					},
				},
				{
					Name:  "http_duration_seconds_bucket",
					Value: 2,
					Labels: map[string]string{
						"node_index": "0",
						"source_id":  "source-id",
						"tag1":       "t1",
						"tag2":       "t2",
						"le":         "0.5",
					},
				},
				{
					Name:  "http_duration_seconds_bucket",
					Value: 2,
					Labels: map[string]string{
						"node_index": "0",
						"source_id":  "source-id",
						"tag1":       "t1",
						"tag2":       "t2",
						"le":         "1",
					},
				},
				{
					Name:  "http_duration_seconds_bucket",
					Value: 2,
					Labels: map[string]string{
						"node_index": "0",
						"source_id":  "source-id",
						"tag1":       "t1",
						"tag2":       "t2",
						"le":         "2.5",
					},
				},
				{
					Name:  "http_duration_seconds_bucket",
					Value: 2,
					Labels: map[string]string{
						"node_index": "0",
						"source_id":  "source-id",
						"tag1":       "t1",
						"tag2":       "t2",
						"le":         "5",
					},
				},
				{
					Name:  "http_duration_seconds_bucket",
					Value: 2,
					Labels: map[string]string{
						"node_index": "0",
						"source_id":  "source-id",
						"tag1":       "t1",
						"tag2":       "t2",
						"le":         "10",
					},
				},
				{
					Name:  "http_duration_seconds_bucket",
					Value: 2,
					Labels: map[string]string{
						"node_index": "0",
						"source_id":  "source-id",
						"tag1":       "t1",
						"tag2":       "t2",
						"le":         "+Inf",
					},
				},
				{
					Name:  "http_duration_seconds_count",
					Value: 2,
					Labels: map[string]string{
						"node_index": "0",
						"source_id":  "source-id",
						"tag1":       "t1",
						"tag2":       "t2",
					},
				},
				{
					Name:  "http_duration_seconds_sum",
					Value: 0.100,
					Labels: map[string]string{
						"node_index": "0",
						"source_id":  "source-id",
						"tag1":       "t1",
						"tag2":       "t2",
					},
				},
			}))

			firstPointTimestamp := points[0].Timestamp
			firstPointTime := time.Unix(0, firstPointTimestamp)

			Expect(firstPointTime).To(BeTemporally("~", time.Unix(0, intervalStart), time.Second))
			Expect(firstPointTime).To(Equal(firstPointTime.Truncate(100 * time.Millisecond)))

			for _, point := range points {
				Expect(point.Timestamp).To(Equal(firstPointTimestamp))
			}
		})

		It("only rolls up gorouter metrics with a peer_type of Server", func() {
			intervalStart := time.Now().Truncate(100 * time.Millisecond).UnixNano()

			streamConnector.envelopes <- []*loggregator_v2.Envelope{
				{
					Timestamp: intervalStart + 1,
					SourceId:  "gorouter",
					Message: &loggregator_v2.Envelope_Timer{
						Timer: &loggregator_v2.Timer{
							Name:  "http",
							Start: 100 * int64(time.Millisecond),
							Stop:  106 * int64(time.Millisecond),
						},
					},
					Tags: map[string]string{
						"peer_type":   "Server",
						"status_code": "200",
					},
				},
				{
					Timestamp: intervalStart + 1,
					SourceId:  "gorouter",
					Message: &loggregator_v2.Envelope_Timer{
						Timer: &loggregator_v2.Timer{
							Name:  "http",
							Start: 100 * int64(time.Millisecond),
							Stop:  106 * int64(time.Millisecond),
						},
					},
					Tags: map[string]string{
						"peer_type":   "Client",
						"status_code": "200",
					},
				},
				{
					Timestamp: intervalStart + 2,
					SourceId:  "gorouter",
					Message: &loggregator_v2.Envelope_Timer{
						Timer: &loggregator_v2.Timer{
							Name:  "http",
							Start: 96 * int64(time.Millisecond),
							Stop:  100 * int64(time.Millisecond),
						},
					},
					Tags: map[string]string{
						"peer_type":   "Server",
						"status_code": "200",
					},
				},
				{
					Timestamp: intervalStart + 2,
					SourceId:  "gorouter",
					Message: &loggregator_v2.Envelope_Timer{
						Timer: &loggregator_v2.Timer{
							Name:  "http",
							Start: 96 * int64(time.Millisecond),
							Stop:  100 * int64(time.Millisecond),
						},
					},
					Tags: map[string]string{
						"peer_type":   "Client",
						"status_code": "200",
					},
				},
				{
					Timestamp: intervalStart + 3,
					SourceId:  "gorouter",
					Message: &loggregator_v2.Envelope_Timer{
						Timer: &loggregator_v2.Timer{
							Name:  "http",
							Start: 500 * int64(time.Millisecond),
							Stop:  1000 * int64(time.Millisecond),
						},
					},
					Tags: map[string]string{
						"peer_type":   "Server",
						"status_code": "400",
					},
				},
				{
					Timestamp: intervalStart + 3,
					SourceId:  "gorouter",
					Message: &loggregator_v2.Envelope_Timer{
						Timer: &loggregator_v2.Timer{
							Name:  "http",
							Start: 500 * int64(time.Millisecond),
							Stop:  1000 * int64(time.Millisecond),
						},
					},
					Tags: map[string]string{
						"peer_type":   "Client",
						"status_code": "400",
					},
				},
			}

			numberOfExpectedSeriesIncludingStatusCode := 2
			numberOfExpectedSeriesExcludingStatusCode := 1
			numberOfExpectedPoints := numberOfExpectedSeriesIncludingStatusCode + (14 * numberOfExpectedSeriesExcludingStatusCode)
			Eventually(metricStore.GetPoints).Should(HaveLen(numberOfExpectedPoints))

			points := metricStore.GetPoints()

			// _count points, per series including status_code
			Expect(points).To(ContainPoints([]*rpc.Point{
				{
					Name:  "http_total",
					Value: 2,
					Labels: map[string]string{
						"node_index":  "0",
						"source_id":   "gorouter",
						"status_code": "200",
					},
				},
				{
					Name:  "http_total",
					Value: 1,
					Labels: map[string]string{
						"node_index":  "0",
						"source_id":   "gorouter",
						"status_code": "400",
					},
				},
			}))

			// _duration_seconds histogram points, per series excluding status_code
			// only testing one series
			Expect(points).To(ContainPoints([]*rpc.Point{
				{
					Name:  "http_duration_seconds_bucket",
					Value: 1,
					Labels: map[string]string{
						"node_index": "0",
						"source_id":  "gorouter",
						"le":         "0.005",
					},
				},
				{
					Name:  "http_duration_seconds_bucket",
					Value: 2,
					Labels: map[string]string{
						"node_index": "0",
						"source_id":  "gorouter",
						"le":         "0.01",
					},
				},
				{
					Name:  "http_duration_seconds_bucket",
					Value: 2,
					Labels: map[string]string{
						"node_index": "0",
						"source_id":  "gorouter",
						"le":         "0.025",
					},
				},
				{
					Name:  "http_duration_seconds_bucket",
					Value: 2,
					Labels: map[string]string{
						"node_index": "0",
						"source_id":  "gorouter",
						"le":         "0.05",
					},
				},
				{
					Name:  "http_duration_seconds_bucket",
					Value: 2,
					Labels: map[string]string{
						"node_index": "0",
						"source_id":  "gorouter",
						"le":         "0.1",
					},
				},
				{
					Name:  "http_duration_seconds_bucket",
					Value: 2,
					Labels: map[string]string{
						"node_index": "0",
						"source_id":  "gorouter",
						"le":         "0.25",
					},
				},
				{
					Name:  "http_duration_seconds_bucket",
					Value: 3,
					Labels: map[string]string{
						"node_index": "0",
						"source_id":  "gorouter",
						"le":         "0.5",
					},
				},
				{
					Name:  "http_duration_seconds_bucket",
					Value: 3,
					Labels: map[string]string{
						"node_index": "0",
						"source_id":  "gorouter",
						"le":         "1",
					},
				},
				{
					Name:  "http_duration_seconds_bucket",
					Value: 3,
					Labels: map[string]string{
						"node_index": "0",
						"source_id":  "gorouter",
						"le":         "2.5",
					},
				},
				{
					Name:  "http_duration_seconds_bucket",
					Value: 3,
					Labels: map[string]string{
						"node_index": "0",
						"source_id":  "gorouter",
						"le":         "5",
					},
				},
				{
					Name:  "http_duration_seconds_bucket",
					Value: 3,
					Labels: map[string]string{
						"node_index": "0",
						"source_id":  "gorouter",
						"le":         "10",
					},
				},
				{
					Name:  "http_duration_seconds_bucket",
					Value: 3,
					Labels: map[string]string{
						"node_index": "0",
						"source_id":  "gorouter",
						"le":         "+Inf",
					},
				},
				{
					Name:  "http_duration_seconds_count",
					Value: 3,
					Labels: map[string]string{
						"node_index": "0",
						"source_id":  "gorouter",
					},
				},
				{
					Name:  "http_duration_seconds_sum",
					Value: 0.510,
					Labels: map[string]string{
						"node_index": "0",
						"source_id":  "gorouter",
					},
				},
			}))

			firstPointTimestamp := points[0].Timestamp
			firstPointTime := time.Unix(0, firstPointTimestamp)

			Expect(firstPointTime).To(BeTemporally("~", time.Unix(0, intervalStart), time.Second))
			Expect(firstPointTime).To(Equal(firstPointTime.Truncate(100 * time.Millisecond)))

			for _, point := range points {
				Expect(point.Timestamp).To(Equal(firstPointTimestamp))
			}
		})

		It("ignores other metrics", func() {
			streamConnector.envelopes <- []*loggregator_v2.Envelope{
				{
					SourceId: "source-id",
					// prime number for higher numerical accuracy
					Timestamp: 10000000002065383,
					Message: &loggregator_v2.Envelope_Timer{
						Timer: &loggregator_v2.Timer{
							Name:  "not_http",
							Start: 0,
							Stop:  5,
						},
					},
				},
				{
					SourceId:  "source-id",
					Timestamp: 66606660666066601,
					Message: &loggregator_v2.Envelope_Counter{
						Counter: &loggregator_v2.Counter{
							Name:  "http",
							Total: 4,
						},
					},
				},
			}

			Eventually(metricStore.GetPoints).Should(HaveLen(1))
			Consistently(metricStore.GetPoints, .5).Should(HaveLen(1))
			Expect(metricStore.GetPoints()).To(ConsistOf(
				&rpc.Point{
					Name:      "http",
					Timestamp: 66606660666066601,
					Value:     4,
					Labels: map[string]string{
						"source_id": "source-id",
					},
				},
			))
		})

		It("keeps a total across rollupIntervals", func() {
			baseTimer := loggregator_v2.Envelope{
				SourceId: "source-id",
				Message: &loggregator_v2.Envelope_Timer{
					Timer: &loggregator_v2.Timer{
						Name:  "http",
						Start: 0,
						Stop:  0,
					},
				},
				Tags: map[string]string{
					"tag1": "t1",
					"tag2": "t2",
				},
			}

			firstTimer := baseTimer
			firstTimer.Message.(*loggregator_v2.Envelope_Timer).Timer.Stop = 5 * int64(time.Second)

			secondTimer := baseTimer
			secondTimer.Message.(*loggregator_v2.Envelope_Timer).Timer.Stop = 2 * int64(time.Second)

			thirdTimer := baseTimer
			thirdTimer.Message.(*loggregator_v2.Envelope_Timer).Timer.Stop = 4 * int64(time.Second)

			streamConnector.envelopes <- []*loggregator_v2.Envelope{&firstTimer}

			// wait for emit interval to elapse, adding 15 per series`
			Eventually(metricStore.GetPoints).Should(HaveLen(15))

			streamConnector.envelopes <- []*loggregator_v2.Envelope{&secondTimer, &thirdTimer}

			// wait for emit interval to elapse, adding 15 more per series
			Eventually(metricStore.GetPoints).Should(HaveLen(45))

			point := rpc.Point{
				Labels: map[string]string{
					"node_index": "0",
					"source_id":  "source-id",
					"tag1":       "t1",
					"tag2":       "t2",
				},
			}

			firstIntervalHistogram := point
			firstIntervalHistogram.Name = "http_duration_seconds_count"
			firstIntervalHistogram.Value = 1
			firstIntervalTotal := point
			firstIntervalTotal.Name = "http_total"
			firstIntervalTotal.Value = 1

			secondIntervalHistogram := point
			secondIntervalHistogram.Name = "http_duration_seconds_count"
			secondIntervalHistogram.Value = 3
			secondIntervalTotal := point
			secondIntervalTotal.Name = "http_total"
			secondIntervalTotal.Value = 3

			Expect(metricStore.GetPoints()).To(ContainPoints([]*rpc.Point{
				&firstIntervalHistogram,
				&firstIntervalTotal,
				&secondIntervalHistogram,
				&secondIntervalTotal,
			}))
		})
	})

	Describe("when the envelope is a Counter", func() {
		It("converts the envelope to a Point", func() {
			streamConnector.envelopes <- []*loggregator_v2.Envelope{
				{
					Timestamp: 20,
					SourceId:  "source-id",
					Message: &loggregator_v2.Envelope_Counter{
						Counter: &loggregator_v2.Counter{
							Name:  "failures",
							Total: 8,
						},
					},
				},
			}

			Eventually(metricStore.GetPoints).Should(HaveLen(1))
			point := metricStore.GetPoints()[0]
			Expect(point.Timestamp).To(Equal(int64(20)))
			Expect(point.Name).To(Equal("failures"))
			Expect(point.Value).To(Equal(float64(8)))
			Expect(point.Labels).To(HaveKeyWithValue("source_id", "source-id"))
		})
	})

	Describe("when the envelope is a Gauge", func() {
		It("converts the envelope to a Point(s)", func() {
			streamConnector.envelopes <- []*loggregator_v2.Envelope{
				{
					Timestamp: 20,
					SourceId:  "source-id",
					Message: &loggregator_v2.Envelope_Gauge{
						Gauge: &loggregator_v2.Gauge{
							Metrics: map[string]*loggregator_v2.GaugeValue{
								"input": {
									Value: 50.0,
									Unit:  "mb/s",
								},
								"output": {
									Value: 25.5,
									Unit:  "kb/s",
								},
							},
						},
					},
				},
			}

			Eventually(metricStore.GetPoints).Should(HaveLen(2))

			Expect(metricStore.GetPoints()).To(ContainPoints([]*rpc.Point{
				{
					Name:      "input",
					Timestamp: 20,
					Value:     50.0,
					Labels: map[string]string{
						"unit":      "mb/s",
						"source_id": "source-id",
					},
				},
				{
					Name:      "output",
					Timestamp: 20,
					Value:     25.5,
					Labels: map[string]string{
						"unit":      "kb/s",
						"source_id": "source-id",
					},
				},
			}))
		})

		It("preserves units on tagged envelopes", func() {
			streamConnector.envelopes <- []*loggregator_v2.Envelope{
				{
					Timestamp: 20,
					SourceId:  "source-id",
					Message: &loggregator_v2.Envelope_Gauge{
						Gauge: &loggregator_v2.Gauge{
							Metrics: map[string]*loggregator_v2.GaugeValue{
								"gauge1": {
									Unit:  "unit1",
									Value: 1,
								},
								"gauge2": {
									Unit:  "unit2",
									Value: 2,
								},
							},
						},
					},
					Tags: map[string]string{
						"deployment": "some-deployment",
					},
				},
			}

			Eventually(metricStore.GetPoints).Should(HaveLen(2))

			Expect(metricStore.GetPoints()).To(ContainPoints([]*rpc.Point{
				{
					Name:      "gauge1",
					Timestamp: 20,
					Value:     1,
					Labels: map[string]string{
						"unit":       "unit1",
						"source_id":  "source-id",
						"deployment": "some-deployment",
					},
				},
				{
					Name:      "gauge2",
					Timestamp: 20,
					Value:     2,
					Labels: map[string]string{
						"unit":       "unit2",
						"source_id":  "source-id",
						"deployment": "some-deployment",
					},
				},
			}))
		})
	})

	Describe("collect nozzle metrics", func() {
		It("writes Ingress, Egress and Err metrics", func() {
			addEnvelope(1, "memory", "some-source-id", streamConnector)
			addEnvelope(2, "memory", "some-source-id", streamConnector)
			addEnvelope(3, "memory", "some-source-id", streamConnector)

			Eventually(metricRegistrar.Fetch(debug.NozzleIngressEnvelopesTotal)).Should(Equal(float64(3)))
			Eventually(metricRegistrar.Fetch(debug.NozzleEgressPointsTotal)).Should(Equal(float64(3)))
			Eventually(metricRegistrar.Fetch(debug.NozzleEgressErrorsTotal)).Should(Equal(float64(0)))
		})

		It("writes duration seconds histogram metrics", func() {
			addEnvelope(1, "memory", "some-source-id", streamConnector)
			Eventually(metricRegistrar.FetchHistogram(debug.NozzleEgressDurationSeconds)).Should(HaveLen(1))
		})
	})

	It("forwards all tags", func() {
		streamConnector.envelopes <- []*loggregator_v2.Envelope{
			{
				Timestamp: 20,
				SourceId:  "source-id",
				Message: &loggregator_v2.Envelope_Counter{
					Counter: &loggregator_v2.Counter{
						Name:  "counter",
						Total: 50,
					},
				},
				Tags: map[string]string{
					"forwarded-tag-1": "forwarded value",
					"forwarded-tag-2": "forwarded value",
				},
			},
		}

		Eventually(metricStore.GetPoints).Should(HaveLen(1))

		Expect(metricStore.GetPoints()).To(ContainPoint(&rpc.Point{
			Timestamp: 20,
			Name:      "counter",
			Value:     50.0,
			Labels: map[string]string{
				"forwarded-tag-1": "forwarded value",
				"forwarded-tag-2": "forwarded value",
				"source_id":       "source-id",
			},
		}))
	})
})

func addEnvelope(timestamp int64, name, sourceId string, c *spyStreamConnector) {
	c.envelopes <- []*loggregator_v2.Envelope{
		{
			Timestamp: timestamp,
			SourceId:  sourceId,
			Message: &loggregator_v2.Envelope_Counter{
				Counter: &loggregator_v2.Counter{Name: name, Total: 0},
			},
		},
	}
}

type spyStreamConnector struct {
	mu        sync.Mutex
	requests_ []*loggregator_v2.EgressBatchRequest
	envelopes chan []*loggregator_v2.Envelope
}

func newSpyStreamConnector() *spyStreamConnector {
	return &spyStreamConnector{
		envelopes: make(chan []*loggregator_v2.Envelope, 100),
	}
}

func (s *spyStreamConnector) Stream(ctx context.Context, req *loggregator_v2.EgressBatchRequest) loggregator.EnvelopeStream {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.requests_ = append(s.requests_, req)

	return func() []*loggregator_v2.Envelope {
		select {
		case e := <-s.envelopes:
			return e
		default:
			return nil
		}
	}
}

func (s *spyStreamConnector) requests() []*loggregator_v2.EgressBatchRequest {
	s.mu.Lock()
	defer s.mu.Unlock()

	reqs := make([]*loggregator_v2.EgressBatchRequest, len(s.requests_))
	copy(reqs, s.requests_)

	return reqs
}
