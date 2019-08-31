package debug

import "github.com/prometheus/client_golang/prometheus"

type NullRegistrar struct {
}

func (*NullRegistrar) Registry() *prometheus.Registry {
	return prometheus.NewRegistry()
}

func (*NullRegistrar) Set(string, float64, ...string) {
}

func (*NullRegistrar) Inc(string, ...string) {
}

func (*NullRegistrar) Add(string, float64, ...string) {
}
