package debug_test

import (
	"fmt"
	"net"
	"net/http"

	"github.com/cloudfoundry/metric-store-release/src/pkg/debug"
	"github.com/cloudfoundry/metric-store-release/src/pkg/logger"
	"github.com/prometheus/client_golang/prometheus"
	goprom "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Registrar", func() {
	var (
		h   *debug.Registrar
		mf  metricFetcher
		lis net.Listener
	)

	BeforeEach(func() {
		testLogger := logger.NewTestLogger()
		h = debug.NewRegistrar(
			testLogger,
			"source_id",
			debug.WithGauge("gauge", prometheus.GaugeOpts{
				Name: "gauge",
				Help: "Basic gauge metric",
			}),
			debug.WithCounter("count", prometheus.CounterOpts{
				Name: "count",
				Help: "Basic counter metric",
			}),
			debug.WithSummary("summary", "label_name", prometheus.SummaryOpts{
				Name: "summary",
				Help: "Basic summary vec",
			}),
			debug.WithHistogram("histogram", prometheus.HistogramOpts{
				Name: "histogram",
				Help: "Basic histogram",
			}),
		)

		lis = debug.StartServer("127.0.0.1:0", h.Registry(), testLogger)
		mf = newMetricFetcher(lis.Addr().String())
	})

	AfterEach(func() {
		lis.Close()
	})

	Describe("Set()", func() {
		It("sets the value on the gauge", func() {
			h.Set("gauge", 30.0)

			Eventually(func() float64 {
				return mf.fetch("gauge")
			}).Should(Equal(30.0))
		})

		It("panics for unknown metric name", func() {
			Expect(func() {
				h.Set("unknown", 30.0)
			}).To(Panic())
		})
	})

	Describe("Inc()", func() {
		It("increments the counter", func() {
			h.Inc("count")

			Eventually(func() float64 {
				return mf.fetch("count")
			}).Should(Equal(1.0))
		})

		It("panics for unknown metric name", func() {
			Expect(func() {
				h.Inc("unknown")
			}).To(Panic())
		})
	})

	Describe("Add()", func() {
		It("increments the counter", func() {
			h.Add("count", 10.0)

			Eventually(func() float64 {
				return mf.fetch("count")
			}).Should(Equal(10.0))
		})

		It("panics for unknown metric name", func() {
			Expect(func() {
				h.Inc("unknown")
			}).To(Panic())
		})
	})

	Describe("Summary()", func() {
		It("adds the point to the observer", func() {
			h.Summary("summary", "label").Observe(23.0)

			// For simplification this is asserting on the sample count
			Eventually(func() float64 {
				return mf.fetch("summary")
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
				return mf.fetch("histogram")
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
	addr string
}

func newMetricFetcher(hostport string) metricFetcher {
	return metricFetcher{addr: "http://" + hostport + "/metrics"}
}

func (mf metricFetcher) fetch(name string) float64 {
	resp, err := http.Get(mf.addr)
	if err != nil {
		panic(err)
	}

	if resp.StatusCode != http.StatusOK {
		panic(fmt.Sprintf("recieved unexpected HTTP status code %d", resp.StatusCode))
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
				return m.GetGauge().GetValue()
			}
		case goprom.MetricType_COUNTER:
			for _, m := range family.GetMetric() {
				return m.GetCounter().GetValue()
			}
		case goprom.MetricType_SUMMARY:
			for _, m := range family.GetMetric() {
				return float64(m.GetSummary().GetSampleCount())
			}
		case goprom.MetricType_HISTOGRAM:
			for _, m := range family.GetMetric() {
				return float64(m.GetHistogram().GetSampleCount())
			}
		default:
			panic("unhandled metric type")
		}
	}

	return -1.0
}
