package testing

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

type SpyMetricRegistrar struct {
	sync.Mutex

	metrics    map[string]float64
	registerer prometheus.Registerer
	gatherer   prometheus.Gatherer
}

func NewSpyMetricRegistrar() *SpyMetricRegistrar {
	defaultRegistry := prometheus.NewRegistry()

	return &SpyMetricRegistrar{
		metrics:    make(map[string]float64),
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
