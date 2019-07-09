package testing

import rpc "github.com/cloudfoundry/metric-store-release/src/pkg/rpc/metricstore_v1"

type SpyAdapter struct {
	CommittedPoints []*rpc.Point
}

func (s *SpyAdapter) WritePoints(points []*rpc.Point) error {
	s.CommittedPoints = points
	return nil
}

func NewSpyAdapter() *SpyAdapter {
	return &SpyAdapter{}
}
