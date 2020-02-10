package rules

import "github.com/prometheus/client_golang/prometheus"

func NewRegisterer(labels prometheus.Labels, reg prometheus.Registerer) *Registerer {
	return &Registerer{
		registerer: prometheus.WrapRegistererWith(labels, reg),
		collectors: []prometheus.Collector{},
	}
}

type Registerer struct {
	registerer prometheus.Registerer
	collectors []prometheus.Collector
}

func (r *Registerer) Register(c prometheus.Collector) error {
	r.collectors = append(r.collectors, c)
	return r.registerer.Register(c)
}

func (r *Registerer) MustRegister(cs ...prometheus.Collector) {
	for _, c := range cs {
		if err := r.registerer.Register(c); err != nil {
			panic(err)
		}
		r.collectors = append(r.collectors, c)
	}
}

func (r *Registerer) Unregister(c prometheus.Collector) bool {
	return r.registerer.Unregister(c)
}

func (r *Registerer) UnregisterAll() bool {
	success := true

	for _, c := range r.collectors {
		success = r.Unregister(c) && success
	}

	return success
}
