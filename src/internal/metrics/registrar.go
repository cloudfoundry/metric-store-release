package metrics

import (
	"github.com/cloudfoundry/metric-store-release/src/pkg/logger"

	"github.com/prometheus/client_golang/prometheus"
)

// PrometheusRegistrar maintains a list of metrics to be served by the health endpoint
// server.
type PrometheusRegistrar struct {
	registerer prometheus.Registerer
	gatherer   prometheus.Gatherer
	log        *logger.Logger

	sourceID    string
	constLabels map[string]string

	counters      map[string]prometheus.Counter
	counterVecs   map[string]*prometheus.CounterVec
	gauges        map[string]prometheus.Gauge
	gaugeVecs     map[string]*prometheus.GaugeVec
	summaries     map[string]*prometheus.SummaryVec
	histograms    map[string]prometheus.Histogram
	histogramVecs map[string]*prometheus.HistogramVec
}

// NewRegistrar returns an initialized health endpoint registrar configured
// with the given Prometheus.Registerer and map of Prometheus metrics.
func NewRegistrar(
	log *logger.Logger,
	sourceID string,
	opts ...RegistrarOption,
) *PrometheusRegistrar {
	defaultRegistry := prometheus.NewRegistry()
	defaultRegistry.MustRegister(prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))
	defaultRegistry.MustRegister(prometheus.NewGoCollector())

	r := &PrometheusRegistrar{
		log:           log,
		registerer:    defaultRegistry,
		gatherer:      defaultRegistry,
		sourceID:      sourceID,
		constLabels:   make(map[string]string),
		counters:      make(map[string]prometheus.Counter),
		counterVecs:   make(map[string]*prometheus.CounterVec),
		gauges:        make(map[string]prometheus.Gauge),
		gaugeVecs:     make(map[string]*prometheus.GaugeVec),
		summaries:     make(map[string]*prometheus.SummaryVec),
		histograms:    make(map[string]prometheus.Histogram),
		histogramVecs: make(map[string]*prometheus.HistogramVec),
	}

	for _, o := range opts {
		o(r)
	}

	return r
}

func (r *PrometheusRegistrar) Registerer() prometheus.Registerer {
	return r.registerer
}

func (r *PrometheusRegistrar) Gatherer() prometheus.Gatherer {
	return r.gatherer
}

// Set will set the given value on the gauge metric with the given name. If
// the gauge metric is not found the process will exit with a status code of
// 1.
func (r *PrometheusRegistrar) Set(name string, value float64, labels ...string) {
	g, ok := r.gauges[name]
	if ok {
		g.Set(value)
		return
	}

	gv, ok := r.gaugeVecs[name]
	if ok {
		gv.WithLabelValues(labels...).Set(value)
		return
	}

	r.log.Panic("Set called for unknown health metric", logger.String("name", name))
}

// Inc will increment the counter metric with the given name by 1. If the
// counter metric is not found the process will exit with a status code of 1.
func (r *PrometheusRegistrar) Inc(name string, labels ...string) {
	c, ok := r.counters[name]
	if ok {
		c.Inc()
		return
	}

	cv, ok := r.counterVecs[name]
	if ok {
		cv.WithLabelValues(labels...).Inc()
		return
	}

	r.log.Panic("Inc called for unknown health metric", logger.String("name", name))
}

// Add will add the given value to the counter metric. If the counter
// metric is not found the process will exit with a status code of 1.
func (r *PrometheusRegistrar) Add(name string, delta float64, labels ...string) {
	c, ok := r.counters[name]
	if ok {
		c.Add(delta)
		return
	}

	cv, ok := r.counterVecs[name]
	if ok {
		cv.WithLabelValues(labels...).Add(delta)
		return
	}

	r.log.Panic("Add called for unknown health metric", logger.String("name", name))
}

// Summary will return the observer that matches the name and label value
func (r *PrometheusRegistrar) Summary(name, label string) prometheus.Observer {
	summary, ok := r.summaries[name]
	if !ok {
		r.log.Panic("Summary called for unknown summary", logger.String("name", name))
	}

	return summary.WithLabelValues(label)
}

// Histogram will return the histogram observer that matches the name.
func (r *PrometheusRegistrar) Histogram(name string, labels ...string) prometheus.Observer {
	histogram, ok := r.histograms[name]
	if ok {
		return histogram
	}

	histogramVec, ok := r.histogramVecs[name]
	if ok {
		return histogramVec.WithLabelValues(labels...)
	}

	r.log.Panic("Histogram called for unknown histogram", logger.String("name", name))
	return nil
}

// RegistrarOption is a function that can be used to set optional configuration
// when initializing a new PrometheusRegistrar.
type RegistrarOption func(*PrometheusRegistrar)

func WithConstLabels(labels map[string]string) RegistrarOption {
	return func(r *PrometheusRegistrar) {
		r.constLabels = labels
	}
}

// WithCounter will create and register a new counter metric.
func WithCounter(name string, opts prometheus.CounterOpts) RegistrarOption {
	return func(r *PrometheusRegistrar) {
		opts.Name = name
		opts.ConstLabels = r.addCommonConstLabels(opts.ConstLabels)

		r.counters[name] = prometheus.NewCounter(opts)
		r.registerer.MustRegister(r.counters[name])
	}
}

func WithLabelledCounter(name string, opts prometheus.CounterOpts, labelsNames []string) RegistrarOption {
	return func(r *PrometheusRegistrar) {
		opts.Name = name
		opts.ConstLabels = r.addCommonConstLabels(opts.ConstLabels)

		r.counterVecs[name] = prometheus.NewCounterVec(opts, labelsNames)
		r.registerer.MustRegister(r.counterVecs[name])
	}
}

// WithGauge will create and register a new gauge metric.
func WithGauge(name string, opts prometheus.GaugeOpts) RegistrarOption {
	return func(r *PrometheusRegistrar) {
		opts.Name = name
		opts.ConstLabels = r.addCommonConstLabels(opts.ConstLabels)

		r.gauges[name] = prometheus.NewGauge(opts)
		r.registerer.MustRegister(r.gauges[name])
	}
}

func WithLabelledGauge(name string, opts prometheus.GaugeOpts, labelsNames []string) RegistrarOption {
	return func(r *PrometheusRegistrar) {
		opts.Name = name
		opts.ConstLabels = r.addCommonConstLabels(opts.ConstLabels)

		r.gaugeVecs[name] = prometheus.NewGaugeVec(opts, labelsNames)
		r.registerer.MustRegister(r.gaugeVecs[name])
	}
}

// WithSummary will create and register a new SummaryVec.
func WithSummary(name, label string, opts prometheus.SummaryOpts) RegistrarOption {
	return func(r *PrometheusRegistrar) {
		opts.Name = name
		opts.ConstLabels = r.addCommonConstLabels(opts.ConstLabels)

		r.summaries[name] = prometheus.NewSummaryVec(opts, []string{label})
		r.registerer.MustRegister(r.summaries[name])
	}
}

// WithHistogram will create and register a new Histogram
func WithHistogram(name string, opts prometheus.HistogramOpts) RegistrarOption {
	return func(r *PrometheusRegistrar) {
		opts.Name = name
		opts.ConstLabels = r.addCommonConstLabels(opts.ConstLabels)

		r.histograms[name] = prometheus.NewHistogram(opts)
		r.registerer.MustRegister(r.histograms[name])
	}
}

func (r *PrometheusRegistrar) addCommonConstLabels(constLabels prometheus.Labels) prometheus.Labels {
	if constLabels == nil {
		constLabels = make(prometheus.Labels)
	}
	constLabels["source_id"] = r.sourceID

	for key, value := range r.constLabels {
		constLabels[key] = value
	}

	return constLabels
}
