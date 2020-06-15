package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

type NullRegistrar struct {
}

func (*NullRegistrar) Registerer() prometheus.Registerer {
	return &NullRegisterer{}
}

func (*NullRegistrar) Gatherer() prometheus.Gatherer {
	return &NullGatherer{}
}

func (*NullRegistrar) Set(string, float64, ...string) {
}

func (*NullRegistrar) Inc(string, ...string) {
}

func (*NullRegistrar) Add(string, float64, ...string) {
}

func (*NullRegistrar) Histogram(string, ...string) prometheus.Observer {
	return &NullObserver{}
}

type NullObserver struct {
}

func (o *NullObserver) Observe(float64) {
}

type NullRegisterer struct {
}

func (n *NullRegisterer) Register(prometheus.Collector) error {
	return nil
}

func (n *NullRegisterer) MustRegister(...prometheus.Collector) {
}

func (n *NullRegisterer) Unregister(prometheus.Collector) bool {
	return true
}

type NullGatherer struct {
}

func (n *NullGatherer) Gather() ([]*dto.MetricFamily, error) {
	return []*dto.MetricFamily{}, nil
}
