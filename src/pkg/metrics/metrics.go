package metrics

import (
	"net/http"
	"regexp"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	matchFirstCap = regexp.MustCompile("(.)([A-Z][a-z]+)")
	matchAllCap   = regexp.MustCompile("([a-z0-9])([A-Z])")
)

// Metrics registers Counter and Gauge metrics.
type Initializer interface {
	// NewCounter returns a function to increment for the given metric.
	NewCounter(name string) func(delta uint64)

	// NewGauge returns a function to set the value for the given metric.
	NewGauge(name, unit string) func(value float64)
}

// nopMetrics are the default metrics.
type NullMetrics struct{}

func (m NullMetrics) NewCounter(name string) func(uint64) {
	return func(uint64) {}
}

func (m NullMetrics) NewGauge(name, unit string) func(float64) {
	return func(float64) {}
}

func (m NullMetrics) NewSummary(name, unit string) func(float64) {
	return func(float64) {}
}

// Metrics stores health metrics for the process. It has a gauge and counter
// metrics.
type Metrics struct {
	Registry *prometheus.Registry
}

// New returns a new Metrics.
func New() *Metrics {
	return &Metrics{
		Registry: prometheus.NewRegistry(),
	}
}

// NewCounter returns a func to be used increment the counter total.
func (m *Metrics) NewCounter(name string) func(delta uint64) {

	prometheusCounterMetric := prometheus.NewCounter(prometheus.CounterOpts{
		Name: name,
	})
	m.Registry.MustRegister(prometheusCounterMetric)

	return func(d uint64) {
		prometheusCounterMetric.Add(float64(d))
	}
}

// NewGauge returns a func to be used to set the value of a gauge metric.
func (m *Metrics) NewGauge(name, unit string) func(value float64) {
	prometheusGaugeMetric := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: name,
		ConstLabels: map[string]string{
			"unit": unit,
		},
	})
	m.Registry.MustRegister(prometheusGaugeMetric)

	return prometheusGaugeMetric.Set
}

// NewSummary returns a func to be used to set the value of a summary metric.
func (m *Metrics) NewSummary(name, unit string) func(value float64) {
	prometheusSummaryMetric := prometheus.NewSummary(prometheus.SummaryOpts{
		Name: name,
		ConstLabels: map[string]string{
			"unit": unit,
		},
		Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
	})
	m.Registry.MustRegister(prometheusSummaryMetric)

	return prometheusSummaryMetric.Observe
}

func (m *Metrics) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	promhttp.HandlerFor(m.Registry, promhttp.HandlerOpts{}).ServeHTTP(w, r)
}
