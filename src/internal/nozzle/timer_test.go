package nozzle_test

import (
	"crypto/tls"
	"time"

	. "github.com/cloudfoundry/metric-store-release/src/internal/nozzle"
	"github.com/cloudfoundry/metric-store-release/src/pkg/logger"
	"github.com/cloudfoundry/metric-store-release/src/pkg/rpc"

	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/cloudfoundry/metric-store-release/src/internal/matchers"
	"github.com/cloudfoundry/metric-store-release/src/internal/testing"
	sharedtls "github.com/cloudfoundry/metric-store-release/src/internal/tls"
)

var _ = Describe("when the envelope is a Timer", func() {
	It("rolls up configured metrics", func() {
		tlsServerConfig, tlsClientConfig := buildTLSConfigs()

		streamConnector := newSpyStreamConnector()
		metricStore := testing.NewSpyMetricStore(tlsServerConfig)
		addrs := metricStore.Start()
		defer metricStore.Stop()

		n := NewNozzle(streamConnector,
			addrs.IngressAddr,
			tlsClientConfig,
			"metric-store",
			0,
			true,
			map[string][]string{"tag1": {"val2", "val3"}},
			WithNozzleDebugRegistrar(testing.NewSpyMetricRegistrar()),
			WithNozzleTimerRollup(
				100*time.Millisecond,
				[]string{"tag1", "tag2", "status_code"},
				[]string{"tag1", "tag2"},
			),
			WithNozzleLogger(logger.NewTestLogger(GinkgoWriter)),
		)
		go n.Start()

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
		tlsServerConfig, tlsClientConfig := buildTLSConfigs()

		streamConnector := newSpyStreamConnector()
		metricStore := testing.NewSpyMetricStore(tlsServerConfig)
		addrs := metricStore.Start()
		defer metricStore.Stop()

		n := NewNozzle(streamConnector,
			addrs.IngressAddr,
			tlsClientConfig,
			"metric-store",
			0,
			true,
			map[string][]string{"tag1": {"val2", "val3"}},
			WithNozzleDebugRegistrar(testing.NewSpyMetricRegistrar()),
			WithNozzleTimerRollup(
				100*time.Millisecond,
				[]string{"tag1", "tag2", "status_code"},
				[]string{"tag1", "tag2"},
			),
			WithNozzleLogger(logger.NewTestLogger(GinkgoWriter)),
		)
		go n.Start()

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
		tlsServerConfig, tlsClientConfig := buildTLSConfigs()

		streamConnector := newSpyStreamConnector()
		metricStore := testing.NewSpyMetricStore(tlsServerConfig)
		addrs := metricStore.Start()
		defer metricStore.Stop()

		n := NewNozzle(streamConnector,
			addrs.IngressAddr,
			tlsClientConfig,
			"metric-store",
			0,
			true,
			map[string][]string{"tag1": {"val2", "val3"}},
			WithNozzleDebugRegistrar(testing.NewSpyMetricRegistrar()),
			WithNozzleTimerRollup(
				100*time.Millisecond,
				[]string{"tag1", "tag2", "status_code"},
				[]string{"tag1", "tag2"},
			),
			WithNozzleLogger(logger.NewTestLogger(GinkgoWriter)),
		)
		go n.Start()

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
		tlsServerConfig, tlsClientConfig := buildTLSConfigs()

		streamConnector := newSpyStreamConnector()
		metricStore := testing.NewSpyMetricStore(tlsServerConfig)
		addrs := metricStore.Start()
		defer metricStore.Stop()

		n := NewNozzle(streamConnector,
			addrs.IngressAddr,
			tlsClientConfig,
			"metric-store",
			0,
			true,
			map[string][]string{"tag1": {"val2", "val3"}},
			WithNozzleDebugRegistrar(testing.NewSpyMetricRegistrar()),
			WithNozzleTimerRollup(
				100*time.Millisecond,
				[]string{"tag1", "tag2", "status_code"},
				[]string{"tag1", "tag2"},
			),
			WithNozzleLogger(logger.NewTestLogger(GinkgoWriter)),
		)
		go n.Start()

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
		Eventually(metricStore.GetPoints).Should(HaveLen(30))

		point := rpc.Point{
			Labels: map[string]string{
				"node_index": "0",
				"source_id":  "source-id",
				"tag1":       "t1",
				"tag2":       "t2",
			},
		}

		secondIntervalHistogram := point
		secondIntervalHistogram.Name = "http_duration_seconds_count"
		secondIntervalHistogram.Value = 3
		secondIntervalTotal := point
		secondIntervalTotal.Name = "http_total"
		secondIntervalTotal.Value = 3

		Expect(metricStore.GetPoints()).To(ContainPoints([]*rpc.Point{
			&secondIntervalHistogram,
			&secondIntervalTotal,
		}))
	})
})

func buildTLSConfigs() (server *tls.Config, client *tls.Config) {
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
	return tlsServerConfig, tlsClientConfig
}
