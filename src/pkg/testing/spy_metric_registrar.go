package testing

import (
	"sync"
)

type SpyMetricRegistrar struct {
	sync.Mutex

	metrics map[string]float64
}

func NewSpyMetricRegistrar() *SpyMetricRegistrar {
	return &SpyMetricRegistrar{
		metrics: make(map[string]float64),
	}
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
