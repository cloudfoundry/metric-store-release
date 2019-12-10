package testing

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

type SpyMetricRegistrar struct {
	sync.Mutex

	metrics    map[string]float64
	histograms map[string]*SpyHistogramObserver
	registerer prometheus.Registerer
	gatherer   prometheus.Gatherer
}

type SpyHistogramObserver struct {
	sync.Mutex
	Observations []float64
}

func NewSpyMetricRegistrar() *SpyMetricRegistrar {
	defaultRegistry := prometheus.NewRegistry()

	return &SpyMetricRegistrar{
		metrics:    make(map[string]float64),
		histograms: make(map[string]*SpyHistogramObserver),
		registerer: defaultRegistry,
		gatherer:   defaultRegistry,
	}
}

func (r *SpyMetricRegistrar) Registerer() prometheus.Registerer {
	return r.registerer
}

func (r *SpyMetricRegistrar) Gatherer() prometheus.Gatherer {
	return r.gatherer
}

func (r *SpyMetricRegistrar) Set(name string, value float64, labels ...string) {
	r.Lock()
	defer r.Unlock()

	r.metrics[name] = value
}

func (r *SpyMetricRegistrar) Add(name string, delta float64, labels ...string) {
	r.Lock()
	defer r.Unlock()

	value, found := r.metrics[name]
	if found {
		r.metrics[name] = value + delta
		return
	}

	r.metrics[name] = delta
}

func (s *SpyHistogramObserver) Observe(value float64) {
	s.Lock()
	defer s.Unlock()
	s.Observations = append(s.Observations, value)
}

func (r *SpyMetricRegistrar) Histogram(name string) prometheus.Observer {
	r.Lock()
	defer r.Unlock()

	spy, ok := r.histograms[name]

	if ok {
		return spy
	}

	spy = &SpyHistogramObserver{}
	r.histograms[name] = spy
	return spy
}

func (r *SpyMetricRegistrar) Inc(name string, labels ...string) {
	r.Add(name, 1)
}

func (r *SpyMetricRegistrar) Fetch(name string) func() float64 {
	return func() float64 {
		r.Lock()
		defer r.Unlock()

		value, found := r.metrics[name]
		if found {
			return value
		}

		return 0
	}
}

func (r *SpyMetricRegistrar) FetchHistogram(name string) func() []float64 {
	return func() []float64 {
		r.Lock()
		defer r.Unlock()

		spy, found := r.histograms[name]
		if found {
			return spy.Observations
		}

		return []float64{}
	}
}

func (r *SpyMetricRegistrar) FetchOption(name string) func() *float64 {
	return func() *float64 {
		r.Lock()
		defer r.Unlock()

		value, found := r.metrics[name]
		if found {
			return &value
		}

		return nil
	}
}
