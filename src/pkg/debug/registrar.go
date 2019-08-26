package debug

import (
	"github.com/cloudfoundry/metric-store-release/src/pkg/logger"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

// Registrar maintains a list of metrics to be served by the health endpoint
// server.
type Registrar struct {
	registry *prometheus.Registry
	log      *logger.Logger

	sourceID string

	counters   map[string]prometheus.Counter
	gauges     map[string]prometheus.Gauge
	summaries  map[string]*prometheus.SummaryVec
	histograms map[string]prometheus.Histogram
}

// NewRegistrar returns an initialized health endpoint registrar configured
// with the given Prometheus.Registerer and map of Prometheus metrics.
func NewRegistrar(
	log *logger.Logger,
	sourceID string,
	opts ...RegistrarOption,
) *Registrar {
	r := &Registrar{
		log:        log,
		registry:   prometheus.NewRegistry(),
		sourceID:   sourceID,
		counters:   make(map[string]prometheus.Counter),
		gauges:     make(map[string]prometheus.Gauge),
		summaries:  make(map[string]*prometheus.SummaryVec),
		histograms: make(map[string]prometheus.Histogram),
	}

	for _, o := range opts {
		o(r)
	}

	return r
}

// Registry returns the Prometheus registry used by the registrar.
func (h *Registrar) Registry() *prometheus.Registry {
	return h.registry
}

// Set will set the given value on the gauge metric with the given name. If
// the gauge metric is not found the process will exit with a status code of
// 1.
func (h *Registrar) Set(name string, value float64) {
	g, ok := h.gauges[name]
	if !ok {
		h.log.Panic("Set called for unknown health metric", logger.String("name", name))
	}

	g.Set(value)
}

// Inc will increment the counter metric with the given name by 1. If the
// counter metric is not found the process will exit with a status code of 1.
func (h *Registrar) Inc(name string) {
	g, ok := h.counters[name]
	if !ok {
		h.log.Panic("Inc called for unknown health metric", logger.String("name", name))
	}

	g.Inc()
}

// Add will add the given value to the counter metric. If the counter
// metric is not found the process will exit with a status code of 1.
func (h *Registrar) Add(name string, delta float64) {
	g, ok := h.counters[name]
	if !ok {
		h.log.Panic("Inc called for unknown health metric", logger.String("name", name))
	}

	g.Add(delta)
}

// Summary will return the observer that matches the name and label value
func (h *Registrar) Summary(name, label string) prometheus.Observer {
	summary, ok := h.summaries[name]
	if !ok {
		h.log.Panic("Summary called for unknown summary", logger.String("name", name))
	}

	return summary.WithLabelValues(label)
}

// Histogram will return the histogram observer that matches the name.
func (h *Registrar) Histogram(name string) prometheus.Observer {
	histogram, ok := h.histograms[name]
	if !ok {
		zap.L().Panic("Histogram called for unknown histogram", logger.String("name", name))
	}

	return histogram
}

// RegistrarOption is a function that can be used to set optional configuration
// when initializing a new Registrar.
type RegistrarOption func(*Registrar)

// WithCounter will create and register a new counter metric.
func WithCounter(name string, opts prometheus.CounterOpts) RegistrarOption {
	return func(r *Registrar) {
		opts.Name = name

		if opts.ConstLabels == nil {
			opts.ConstLabels = make(prometheus.Labels)
		}
		opts.ConstLabels["source_id"] = r.sourceID

		r.counters[name] = prometheus.NewCounter(opts)

		r.registry.MustRegister(r.counters[name])
	}
}

// WithGauge will create and register a new gauge metric.
func WithGauge(name string, opts prometheus.GaugeOpts) RegistrarOption {
	return func(r *Registrar) {
		opts.Name = name

		if opts.ConstLabels == nil {
			opts.ConstLabels = make(prometheus.Labels)
		}
		opts.ConstLabels["source_id"] = r.sourceID

		r.gauges[name] = prometheus.NewGauge(opts)

		r.registry.MustRegister(r.gauges[name])
	}
}

// WithSummary will create and register a new SummaryVec.
func WithSummary(name, label string, opts prometheus.SummaryOpts) RegistrarOption {
	return func(r *Registrar) {
		opts.Name = name

		if opts.ConstLabels == nil {
			opts.ConstLabels = make(prometheus.Labels)
		}
		opts.ConstLabels["source_id"] = r.sourceID

		r.summaries[name] = prometheus.NewSummaryVec(opts, []string{label})

		r.registry.MustRegister(r.summaries[name])
	}
}

// WithHistogram will create and register a new Histogram
func WithHistogram(name string, opts prometheus.HistogramOpts) RegistrarOption {
	return func(r *Registrar) {
		opts.Name = name

		if opts.ConstLabels == nil {
			opts.ConstLabels = make(prometheus.Labels)
		}
		opts.ConstLabels["source_id"] = r.sourceID

		r.histograms[name] = prometheus.NewHistogram(opts)

		r.registry.MustRegister(r.histograms[name])
	}
}
