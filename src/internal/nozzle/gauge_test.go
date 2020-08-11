package nozzle_test

import (
	"time"

	. "github.com/cloudfoundry/metric-store-release/src/internal/nozzle"
	"github.com/cloudfoundry/metric-store-release/src/pkg/logger"
	"github.com/cloudfoundry/metric-store-release/src/pkg/rpc"

	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/cloudfoundry/metric-store-release/src/internal/matchers"
	"github.com/cloudfoundry/metric-store-release/src/internal/testing"
)

var _ = Describe("Nozzle", func() {
	Describe("when the envelope is a Gauge", func() {
		It("converts the envelope to a Point(s)", func() {
			tlsServerConfig, tlsClientConfig := buildTLSConfigs()

			streamConnector := newSpyStreamConnector()
			metricStore := testing.NewSpyMetricStore(tlsServerConfig)
			addrs := metricStore.Start()
			defer metricStore.Stop()

			n := NewNozzle(streamConnector, addrs.EgressAddr, addrs.IngressAddr, tlsClientConfig, "metric-store", 0,
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
			tlsServerConfig, tlsClientConfig := buildTLSConfigs()

			streamConnector := newSpyStreamConnector()
			metricStore := testing.NewSpyMetricStore(tlsServerConfig)
			addrs := metricStore.Start()
			defer metricStore.Stop()

			n := NewNozzle(streamConnector, addrs.EgressAddr, addrs.IngressAddr, tlsClientConfig, "metric-store", 0,
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

})
