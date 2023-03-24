package nozzle_test

import (
	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/cloudfoundry/metric-store-release/src/internal/matchers"
	. "github.com/cloudfoundry/metric-store-release/src/internal/nozzle"
	"github.com/cloudfoundry/metric-store-release/src/internal/testing"
	"github.com/cloudfoundry/metric-store-release/src/pkg/logger"
	"github.com/cloudfoundry/metric-store-release/src/pkg/rpc"
)

var _ = Describe("Nozzle", func() {
	var (
		streamConnector *spyStreamConnector
		metricStore     *testing.SpyMetricStore
		nozzle          *Nozzle
	)

	AfterEach(func() {
		// nozzle.Stop()
		metricStore.Stop()
	})

	Describe("When disabled platform and system metrics is OFF", func() {
		BeforeEach(func() {
			tlsServerConfig, tlsClientConfig := buildTLSConfigs()

			streamConnector = newSpyStreamConnector()
			metricStore = testing.NewSpyMetricStore(tlsServerConfig)
			addrs := metricStore.Start()

			nozzle = NewNozzle(streamConnector, addrs.IngressAddr, tlsClientConfig, "metric-store", 0, false, []string{},
				WithNozzleDebugRegistrar(testing.NewSpyMetricRegistrar()),
				WithNozzleTimerRollup(
					100*time.Millisecond,
					[]string{"tag1", "tag2", "status_code"},
					[]string{"tag1", "tag2"},
				),
				WithNozzleLogger(logger.NewTestLogger(GinkgoWriter)),
			)
			go nozzle.Start()
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

	Describe("When disabled platform and system metrics is ON", func() {
		BeforeEach(func() {
			tlsServerConfig, tlsClientConfig := buildTLSConfigs()

			streamConnector = newSpyStreamConnector()
			metricStore = testing.NewSpyMetricStore(tlsServerConfig)
			addrs := metricStore.Start()

			nozzle = NewNozzle(streamConnector, addrs.IngressAddr, tlsClientConfig, "metric-store", 0, true, []string{},
				WithNozzleDebugRegistrar(testing.NewSpyMetricRegistrar()),
				WithNozzleTimerRollup(
					100*time.Millisecond,
					[]string{"tag1", "tag2", "status_code"},
					[]string{"tag1", "tag2"},
				),
				WithNozzleLogger(logger.NewTestLogger(GinkgoWriter)),
			)
			go nozzle.Start()
		})

		FIt("writes each envelope as a point when metric have a app_id or applicationGuid tag", func() {
			addEnvelopeWithTag(1, "memory", "some-source-id", map[string]string{AppId: "some-source-id"}, streamConnector)
			addEnvelopeWithTag(2, "memory", "some-source-id", map[string]string{ApplicationGuid: "some-source-id"}, streamConnector)
			addEnvelopeWithTag(3, "memory", "some-source-id", map[string]string{ApplicationGuid: "some-source-id"}, streamConnector)

			Eventually(metricStore.GetPoints).Should(HaveLen(3))
			Expect(metricStore.GetPoints()).To(ContainPoints([]*rpc.Point{
				{
					Name:      "memory",
					Timestamp: 1,
					Value:     0,
					Labels: map[string]string{
						"source_id": "some-source-id",
						AppId:       "some-source-id",
					},
				},
				{
					Name:      "memory",
					Timestamp: 2,
					Value:     0,
					Labels: map[string]string{
						"source_id":     "some-source-id",
						ApplicationGuid: "some-source-id",
					},
				},
				{
					Name:      "memory",
					Timestamp: 3,
					Value:     0,
					Labels: map[string]string{
						"source_id":     "some-source-id",
						ApplicationGuid: "some-source-id",
					},
				},
			}))
		})

		FIt("Should skip metrics which is not application", func() {
			addEnvelopeWithTag(1, "memory", "some-source-id", map[string]string{AppId: "some-source-id"}, streamConnector)
			addEnvelopeWithTag(2, "memory", "some-source-id", map[string]string{ApplicationGuid: "some-source-id"}, streamConnector)
			addEnvelopeWithTag(3, "memory", "some-source-id", map[string]string{"tag1": "some-source-id"}, streamConnector)

			Eventually(metricStore.GetPoints).Should(HaveLen(2))
			Expect(metricStore.GetPoints()).To(ContainPoints([]*rpc.Point{
				{
					Name:      "memory",
					Timestamp: 1,
					Value:     0,
					Labels: map[string]string{
						"source_id": "some-source-id",
						AppId:       "some-source-id",
					},
				},
				{
					Name:      "memory",
					Timestamp: 2,
					Value:     0,
					Labels: map[string]string{
						"source_id":     "some-source-id",
						ApplicationGuid: "some-source-id",
					},
				},
			}))
		})
	})

})
