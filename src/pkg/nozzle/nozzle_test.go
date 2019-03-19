package nozzle_test

import (
	"encoding/csv"
	"log"
	"strings"
	"sync"
	"time"

	loggregator "code.cloudfoundry.org/go-loggregator"
	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	. "github.com/cloudfoundry/metric-store-release/src/pkg/nozzle"
	rpc "github.com/cloudfoundry/metric-store-release/src/pkg/rpc/metricstore_v1"
	"github.com/cloudfoundry/metric-store-release/src/pkg/tls"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	. "github.com/cloudfoundry/metric-store-release/src/pkg/matchers"
	"github.com/cloudfoundry/metric-store-release/src/pkg/testing"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Nozzle", func() {
	var (
		n               *Nozzle
		streamConnector *spyStreamConnector
		metricStore     *testing.SpyMetricStore
		metricMap       *testing.SpyMetrics
	)

	BeforeEach(func() {
		tlsConfig, err := tls.NewMutualTLSConfig(
			testing.Cert("metric-store-ca.crt"),
			testing.Cert("metric-store.crt"),
			testing.Cert("metric-store.key"),
			"metric-store",
		)
		Expect(err).ToNot(HaveOccurred())
		streamConnector = newSpyStreamConnector()
		metricMap = testing.NewSpyMetrics()
		metricStore = testing.NewSpyMetricStore(tlsConfig)
		addr := metricStore.Start()

		n = NewNozzle(streamConnector, addr, "metric-store", 0,
			WithNozzleMetrics(metricMap),
			WithNozzleDialOpts(grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig))),
			WithNozzleTimerRollup(100*time.Millisecond, "rolled_timer", []string{"tag1", "tag2"}),
			WithNozzleLogger(log.New(GinkgoWriter, "", 0)),
		)
		go n.Start()
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
							Name:  "rolled_timer",
							Start: 0,
							Stop:  5,
						},
					},
					Tags: map[string]string{
						"tag1": "t1",
						"tag2": "t2",
					},
				},
				{
					Timestamp: intervalStart + 2,
					SourceId:  "source-id",
					Message: &loggregator_v2.Envelope_Timer{
						Timer: &loggregator_v2.Timer{
							Name:  "rolled_timer",
							Start: 5,
							Stop:  9,
						},
					},
					Tags: map[string]string{
						"tag1": "t1",
						"tag2": "t2",
					},
				},
				{
					Timestamp: intervalStart + 3,
					SourceId:  "source-id-2",
					Message: &loggregator_v2.Envelope_Timer{
						Timer: &loggregator_v2.Timer{
							Name:  "rolled_timer",
							Start: 3,
							Stop:  6,
						},
					},
				},
			}

			Eventually(metricStore.GetPoints).Should(HaveLen(4))
			Expect(metricStore.GetPoints()).To(ContainPoints([]*rpc.Point{
				{
					Name:  "rolled_timer_mean",
					Value: 4.5,
					Labels: map[string]string{
						"node_index": "0",
						"source_id":  "source-id",
						"tag1":       "t1",
						"tag2":       "t2",
					},
				},
				{
					Name:  "rolled_timer_count",
					Value: 2,
					Labels: map[string]string{
						"node_index": "0",
						"source_id":  "source-id",
						"tag1":       "t1",
						"tag2":       "t2",
					},
				},
				{
					Name:  "rolled_timer_mean",
					Value: 3,
					Labels: map[string]string{
						"node_index": "0",
						"source_id":  "source-id-2",
					},
				},
				{
					Name:  "rolled_timer_count",
					Value: 1,
					Labels: map[string]string{
						"node_index": "0",
						"source_id":  "source-id-2",
					},
				},
			}))

			firstPointTimestamp := metricStore.GetPoints()[0].Timestamp
			firstPointTime := time.Unix(0, firstPointTimestamp)

			Expect(firstPointTime).To(BeTemporally("~", time.Unix(0, intervalStart), time.Second))
			Expect(firstPointTime).To(Equal(firstPointTime.Truncate(100 * time.Millisecond)))

			for _, point := range metricStore.GetPoints() {
				Expect(point.Timestamp).To(Equal(firstPointTimestamp))
			}

			Expect(metricMap.SummaryObservationsGetter("nozzle_timer_drift")()).To(ConsistOf(
				BeNumerically("~", 0.5, .5),
				BeNumerically("~", 0.5, .5),
				BeNumerically("~", 0.5, .5),
			))
			Expect(metricMap.GetUnit("nozzle_timer_drift")).To(Equal("seconds"))
		})

		It("ignores other metrics", func() {
			streamConnector.envelopes <- []*loggregator_v2.Envelope{
				{
					SourceId: "source-id",
					// prime number for higher numerical accuracy
					Timestamp: 10000000002065383,
					Message: &loggregator_v2.Envelope_Timer{
						Timer: &loggregator_v2.Timer{
							Name:  "unrolled_timer",
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
							Name:  "rolled_timer",
							Total: 4,
						},
					},
				},
			}

			Eventually(metricStore.GetPoints).Should(HaveLen(1))
			Consistently(metricStore.GetPoints, .5).Should(HaveLen(1))
			Expect(metricStore.GetPoints()).To(ConsistOf(
				&rpc.Point{
					Name:      "rolled_timer",
					Timestamp: 66606660666066601,
					Value:     4,
					Labels: map[string]string{
						"source_id": "source-id",
					},
				},
			))
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

	It("writes Ingress, Egress and Err metrics", func() {
		addEnvelope(1, "memory", "some-source-id", streamConnector)
		addEnvelope(2, "memory", "some-source-id", streamConnector)
		addEnvelope(3, "memory", "some-source-id", streamConnector)

		Eventually(metricMap.Getter("nozzle_ingress")).Should(Equal(float64(3)))
		Eventually(metricMap.Getter("nozzle_egress")).Should(Equal(float64(3)))
		Eventually(metricMap.Getter("nozzle_err")).Should(Equal(float64(0)))
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

	Describe("tag info tracking", func() {
		XIt("tracks information correctly", func() {
			envelopes := []*loggregator_v2.Envelope{
				{
					Timestamp: 10,
					SourceId:  "source-id",
					Message: &loggregator_v2.Envelope_Timer{
						Timer: &loggregator_v2.Timer{
							Name: "disk_access",
						},
					},
					Tags: map[string]string{
						"tag-name": "tag value short",
					},
				},
				{
					Timestamp: 20,
					SourceId:  "source-id",
					Message: &loggregator_v2.Envelope_Timer{
						Timer: &loggregator_v2.Timer{
							Name: "disk_access",
						},
					},
					Tags: map[string]string{
						"tag-name": "tag value that is of medium length",
					},
				},
				{
					Timestamp: 30,
					SourceId:  "source-id",
					Message: &loggregator_v2.Envelope_Counter{
						Counter: &loggregator_v2.Counter{
							Name: "cpu",
						},
					},
					Tags: map[string]string{
						"tag-name": "tag-value",
					},
				},
				{
					Timestamp: 40,
					SourceId:  "source-id",
					Message: &loggregator_v2.Envelope_Counter{
						Counter: &loggregator_v2.Counter{
							Name: "cpu",
						},
					},
					Tags: map[string]string{
						"tag-name": "tag-value",
					},
				},
				{
					Timestamp: 50,
					SourceId:  "source-id-other",
					Message: &loggregator_v2.Envelope_Counter{
						Counter: &loggregator_v2.Counter{
							Name: "cpu",
						},
					},
					Tags: map[string]string{
						"tag-name":       "a really verbose tag that computes to a longer or rather higher length, you know",
						"tag-name-other": "tag-value-other",
					},
				},
			}
			streamConnector.envelopes <- envelopes

			Eventually(metricStore.GetPoints).Should(HaveLen(len(envelopes)))
			Expect(n.GetTagInfo()).To(HaveKey("tag-name"))
			Expect(n.GetTagInfo()["tag-name"].MetricNames.ToSlice()).To(ConsistOf("disk_access", "cpu"))
			Expect(n.GetTagInfo()["tag-name"].SourceIds.ToSlice()).To(ConsistOf("source-id", "source-id-other"))
			Expect(n.GetTagInfo()["tag-name"].Count).To(BeEquivalentTo(5))
			Expect(n.GetTagInfo()["tag-name"].AverageValueLength).To(BeEquivalentTo(29.4))
			Expect(n.GetTagInfo()["tag-name"].ShortestValue).To(Equal("tag-value"))
			Expect(n.GetTagInfo()["tag-name"].LongestValue).To(Equal("a really verbose tag that computes to a longer or rather higher length, you know"))
			Expect(n.GetTagInfo()["tag-name"].UniqueValues.Cardinality()).To(BeEquivalentTo(4))

			Expect(n.GetTagInfo()).To(HaveKey("tag-name-other"))
			Expect(n.GetTagInfo()["tag-name-other"].MetricNames.ToSlice()).To(ConsistOf("cpu"))
			Expect(n.GetTagInfo()["tag-name-other"].SourceIds.ToSlice()).To(ConsistOf("source-id-other"))
			Expect(n.GetTagInfo()["tag-name-other"].Count).To(BeEquivalentTo(1))
			Expect(n.GetTagInfo()["tag-name-other"].AverageValueLength).To(BeEquivalentTo(15))
			Expect(n.GetTagInfo()["tag-name-other"].ShortestValue).To(Equal("tag-value-other"))
			Expect(n.GetTagInfo()["tag-name-other"].LongestValue).To(Equal("tag-value-other"))
			Expect(n.GetTagInfo()["tag-name-other"].UniqueValues.Cardinality()).To(BeEquivalentTo(1))
		})

		XIt("can return CSV-serialized output", func() {
			envelopes := []*loggregator_v2.Envelope{
				{
					Timestamp: 10,
					SourceId:  "source-id",
					Message: &loggregator_v2.Envelope_Timer{
						Timer: &loggregator_v2.Timer{
							Name: "disk_access",
						},
					},
					Tags: map[string]string{
						"tag-name": "tag value short",
					},
				},
				{
					Timestamp: 40,
					SourceId:  "source-id",
					Message: &loggregator_v2.Envelope_Counter{
						Counter: &loggregator_v2.Counter{
							Name: "cpu",
						},
					},
					Tags: map[string]string{
						"tag-name": "tag-value",
					},
				},
				{
					Timestamp: 50,
					SourceId:  "source-id-other",
					Message: &loggregator_v2.Envelope_Counter{
						Counter: &loggregator_v2.Counter{
							Name: "cpu",
						},
					},
					Tags: map[string]string{
						"tag-name":       "a really verbose tag that computes to a longer, you know?",
						"tag-name-other": "tag-value-other",
					},
				},
			}
			streamConnector.envelopes <- envelopes

			Eventually(metricStore.GetPoints).Should(HaveLen(len(envelopes)))
			tagInfoCsv := n.GetTagInfoCsv()

			reader := csv.NewReader(strings.NewReader(tagInfoCsv))
			result, err := reader.ReadAll()
			Expect(err).ToNot(HaveOccurred())

			Expect(result).To(HaveLen(3))
			Expect(result[0]).To(Equal([]string{
				"tag_name",
				"num_metric_names",
				"num_source_ids",
				"count",
				"average_value_length",
				"shortest_value",
				"longest_value",
				"cardinality",
			}))
			Expect(result).To(ContainElement([]string{
				"tag-name",
				"2",
				"2",
				"3",
				"27",
				"tag-value",
				"a really verbose tag that computes to a longer, you know?",
				"3",
			}))
			Expect(result).To(ContainElement([]string{
				"tag-name-other",
				"1",
				"1",
				"1",
				"15",
				"tag-value-other",
				"tag-value-other",
				"1",
			}))
		})
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
