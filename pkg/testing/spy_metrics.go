package testing

import (
	"sync"
)

const (
	UNDEFINED_METRIC float64 = 99999999999
)

type SpyMetrics struct {
	values              map[string]float64
	units               map[string]string
	summaryObservations map[string][]float64
	sync.Mutex
}

func NewSpyMetrics() *SpyMetrics {
	return &SpyMetrics{
		values:              make(map[string]float64),
		units:               make(map[string]string),
		summaryObservations: make(map[string][]float64),
	}
}

func (s *SpyMetrics) NewCounter(name string) func(uint64) {
	s.Lock()
	defer s.Unlock()
	s.values[name] = 0

	return func(value uint64) {
		s.Lock()
		defer s.Unlock()

		s.values[name] += float64(value)
	}
}

func (s *SpyMetrics) NewGauge(name, unit string) func(float64) {
	s.Lock()
	defer s.Unlock()
	s.values[name] = 0
	s.units[name] = unit

	return func(value float64) {
		s.Lock()
		defer s.Unlock()

		s.values[name] = value
	}
}

func (s *SpyMetrics) NewSummary(name, unit string) func(float64) {
	s.Lock()
	defer s.Unlock()
	s.summaryObservations[name] = make([]float64, 0)
	s.units[name] = unit

	return func(value float64) {
		s.Lock()
		defer s.Unlock()

		s.summaryObservations[name] = append(s.summaryObservations[name], value)
	}
}

func (s *SpyMetrics) Getter(name string) func() float64 {
	return func() float64 {
		s.Lock()
		defer s.Unlock()

		value, ok := s.values[name]
		if !ok {
			return UNDEFINED_METRIC
		}
		return value
	}
}

func (s *SpyMetrics) Get(name string) float64 {
	return s.Getter(name)()
}

func (s *SpyMetrics) GetUnit(name string) string {
	s.Lock()
	defer s.Unlock()

	return s.units[name]
}

func (s *SpyMetrics) SummaryObservationsGetter(name string) func() []float64 {
	return func() []float64 {
		s.Lock()
		defer s.Unlock()

		value := s.summaryObservations[name]
		return value
	}
}
