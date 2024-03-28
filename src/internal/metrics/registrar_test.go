package metrics_test

import (
	"crypto/tls"
	"fmt"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	goprom "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"

	"github.com/cloudfoundry/metric-store-release/src/internal/metrics"
	"github.com/cloudfoundry/metric-store-release/src/internal/testing"
	shared "github.com/cloudfoundry/metric-store-release/src/internal/tls"
	"github.com/cloudfoundry/metric-store-release/src/pkg/logger"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("PrometheusRegistrar", func() {
	var (
		h   *metrics.PrometheusRegistrar
		mf  metricFetcher
		lis *metrics.Server
	)

	BeforeEach(func() {
		testLogger := logger.NewNop()
		h = metrics.NewRegistrar(
			testLogger,
			"source_id",
			metrics.WithConstLabels(map[string]string{"fuz": "baz"}),
			metrics.WithCounter("count", prometheus.CounterOpts{
				Help: "Basic counter metric",
			}),
			metrics.WithLabelledCounter("labelled_count", prometheus.CounterOpts{
				Help: "Basic counter metric",
			}, []string{"foo"}),
			metrics.WithGauge("gauge", prometheus.GaugeOpts{
				Help: "Basic gauge metric",
			}),
			metrics.WithLabelledGauge("labelled_gauge", prometheus.GaugeOpts{
				Help: "Basic gauge metric",
			}, []string{"foo"}),
			metrics.WithSummary("summary", "label_name", prometheus.SummaryOpts{
				Help: "Basic summary vec",
			}),
			metrics.WithHistogram("histogram", prometheus.HistogramOpts{
				Help: "Basic histogram",
			}),
		)

		tlsServerConfig, err := shared.NewMutualTLSServerConfig(
			testing.Cert("metric-store-ca.crt"),
			testing.Cert("metric-store.crt"),
			testing.Cert("metric-store.key"),
		)
		Expect(err).NotTo(HaveOccurred())

		tlsClientConfig, err := shared.NewMutualTLSClientConfig(
			testing.Cert("metric-store-ca.crt"),
			testing.Cert("metric-store.crt"),
			testing.Cert("metric-store.key"),
			"metric-store",
		)
		Expect(err).NotTo(HaveOccurred())

		lis = metrics.StartMetricsServer(
			"127.0.0.1:0",
			tlsServerConfig,
			testLogger,
			h,
		)
		mf = newMetricFetcher(lis.Addr().String(), tlsClientConfig)
	})

	AfterEach(func() {
		err := lis.Close()
		if err != nil {
			fmt.Println("this should never happen")
		}
	})

	Describe("Inc()", func() {
		It("increments a counter", func() {
			h.Inc("count")

			Eventually(func() float64 {
				value, _ := mf.fetch("count")
				return value
			}).Should(Equal(1.0))
		})

		It("increments a labelled counter", func() {
			h.Inc("labelled_count", "bar")

			Eventually(func() float64 {
				value, _ := mf.fetch("labelled_count")
				return value
			}).Should(Equal(1.0))
		})

		It("sets the constant labels on a counter", func() {
			h.Inc("count")

			Eventually(func() []*goprom.LabelPair {
				_, labels := mf.fetch("count")
				return labels
			}).Should(ContainElement(&goprom.LabelPair{Name: str("fuz"), Value: str("baz")}))
		})

		It("sets the dynamic labels on a labelled counter", func() {
			h.Inc("labelled_count", "bar")

			Eventually(func() []*goprom.LabelPair {
				_, labels := mf.fetch("labelled_count")
				return labels
			}).Should(ContainElement(&goprom.LabelPair{Name: str("foo"), Value: str("bar")}))
		})

		It("sets the constant labels on a labelled counter", func() {
			h.Inc("labelled_count", "bar")

			Eventually(func() []*goprom.LabelPair {
				_, labels := mf.fetch("labelled_count")
				return labels
			}).Should(ContainElement(&goprom.LabelPair{Name: str("fuz"), Value: str("baz")}))
		})

		It("panics when incrementing a labelled counter without labels", func() {
			Expect(func() {
				h.Inc("labelled_count")
			}).To(Panic())
		})

		It("panics for unknown metric name", func() {
			Expect(func() {
				h.Inc("unknown")
			}).To(Panic())
		})
	})

	Describe("Add()", func() {
		It("adds to a counter", func() {
			h.Add("count", 10.0)

			Eventually(func() float64 {
				value, _ := mf.fetch("count")
				return value
			}).Should(Equal(10.0))
		})

		It("adds to a labelled counter", func() {
			h.Add("labelled_count", 10.0, "bar")

			Eventually(func() float64 {
				value, _ := mf.fetch("labelled_count")
				return value
			}).Should(Equal(10.0))
		})

		It("sets the constant labels on a counter", func() {
			h.Add("count", 30.0)

			Eventually(func() []*goprom.LabelPair {
				_, labels := mf.fetch("count")
				return labels
			}).Should(ContainElement(&goprom.LabelPair{Name: str("fuz"), Value: str("baz")}))
		})

		It("sets the dynamic labels on a labelled counter", func() {
			h.Add("labelled_count", 30.0, "bar")

			Eventually(func() []*goprom.LabelPair {
				_, labels := mf.fetch("labelled_count")
				return labels
			}).Should(ContainElement(&goprom.LabelPair{Name: str("foo"), Value: str("bar")}))
		})

		It("sets the constant labels on a labelled counter", func() {
			h.Add("labelled_count", 30.0, "bar")

			Eventually(func() []*goprom.LabelPair {
				_, labels := mf.fetch("labelled_count")
				return labels
			}).Should(ContainElement(&goprom.LabelPair{Name: str("fuz"), Value: str("baz")}))
		})

		It("panics when adding to a labelled counter without labels", func() {
			Expect(func() {
				h.Add("labelled_count", 10.0)
			}).To(Panic())
		})

		It("panics for unknown metric name", func() {
			Expect(func() {
				h.Add("unknown", 10.0)
			}).To(Panic())
		})
	})

	Describe("Set()", func() {
		It("sets the value on a gauge", func() {
			h.Set("gauge", 30.0)

			Eventually(func() float64 {
				value, _ := mf.fetch("gauge")
				return value
			}).Should(Equal(30.0))
		})

		It("sets the value on a labelled gauge", func() {
			h.Set("labelled_gauge", 30.0, "bar")

			Eventually(func() float64 {
				value, _ := mf.fetch("labelled_gauge")
				return value
			}).Should(Equal(30.0))
		})

		It("sets the constant labels on a gauge", func() {
			h.Set("gauge", 30.0)

			Eventually(func() []*goprom.LabelPair {
				_, labels := mf.fetch("gauge")
				return labels
			}).Should(ContainElement(&goprom.LabelPair{Name: str("fuz"), Value: str("baz")}))
		})

		It("sets the dynamic labels on a labelled gauge", func() {
			h.Set("labelled_gauge", 30.0, "bar")

			Eventually(func() []*goprom.LabelPair {
				_, labels := mf.fetch("labelled_gauge")
				return labels
			}).Should(ContainElement(&goprom.LabelPair{Name: str("foo"), Value: str("bar")}))
		})

		It("sets the constant labels on a labelled gauge", func() {
			h.Set("labelled_gauge", 30.0, "bar")

			Eventually(func() []*goprom.LabelPair {
				_, labels := mf.fetch("labelled_gauge")
				return labels
			}).Should(ContainElement(&goprom.LabelPair{Name: str("fuz"), Value: str("baz")}))
		})

		It("panics when setting the value on a labelled gauge without labels", func() {
			Expect(func() {
				h.Set("labelled_gauge", 30.0)
			}).To(Panic())
		})

		It("panics for unknown metric name", func() {
			Expect(func() {
				h.Set("unknown", 30.0)
			}).To(Panic())
		})
	})

	Describe("Summary()", func() {
		It("adds the point to the observer", func() {
			h.Summary("summary", "label").Observe(23.0)

			// For simplification this is asserting on the sample count
			Eventually(func() float64 {
				value, _ := mf.fetch("summary")
				return value
			}).Should(Equal(1.0))
		})

		It("panics for unknown metric name", func() {
			Expect(func() {
				h.Summary("unknown", "label")
			}).To(Panic())
		})
	})

	Describe("Histogram()", func() {
		It("adds the point to the histogram observer", func() {
			h.Histogram("histogram").Observe(23.0)

			// For simplification this is asserting on the sample count
			Eventually(func() float64 {
				value, _ := mf.fetch("histogram")
				return value
			}).Should(Equal(1.0))
		})

		It("adds the point to a labeled histogram observer", func() {
			h.Histogram("histogram", "bar").Observe(10.0)

			// For simplification this is asserting on the sample count
			Eventually(func() float64 {
				value, _ := mf.fetch("histogram")
				return value
			}).Should(Equal(1.0))
		})

		It("panics for unknown metric name", func() {
			Expect(func() {
				h.Histogram("unknown")
			}).To(Panic())
		})
	})
})

type metricFetcher struct {
	addr      string
	tlsConfig *tls.Config
}

func newMetricFetcher(hostport string, tlsConfig *tls.Config) metricFetcher {
	return metricFetcher{
		addr:      "https://" + hostport + "/metrics",
		tlsConfig: tlsConfig,
	}
}

func (mf metricFetcher) fetch(name string) (float64, []*goprom.LabelPair) {
	httpClient := &http.Client{
		Transport: &http.Transport{TLSClientConfig: mf.tlsConfig},
	}

	resp, err := httpClient.Get(mf.addr)
	if err != nil {
		panic(err)
	}

	if resp.StatusCode != http.StatusOK {
		panic(fmt.Sprintf("received unexpected HTTP status code %d", resp.StatusCode))
	}

	p := &expfmt.TextParser{}
	res, err := p.TextToMetricFamilies(resp.Body)
	if err != nil {
		panic(err)
	}

	for _, family := range res {
		if family.GetName() != name {
			continue
		}

		switch family.GetType() {
		case goprom.MetricType_GAUGE:
			for _, m := range family.GetMetric() {
				return m.GetGauge().GetValue(), m.GetLabel()
			}
		case goprom.MetricType_COUNTER:
			for _, m := range family.GetMetric() {
				return m.GetCounter().GetValue(), m.GetLabel()
			}
		case goprom.MetricType_SUMMARY:
			for _, m := range family.GetMetric() {
				return float64(m.GetSummary().GetSampleCount()), m.GetLabel()
			}
		case goprom.MetricType_HISTOGRAM:
			for _, m := range family.GetMetric() {
				return float64(m.GetHistogram().GetSampleCount()), m.GetLabel()
			}
		default:
			panic("unhandled metric type")
		}
	}

	return -1.0, nil
}

func str(s string) *string {
	return &s
}
