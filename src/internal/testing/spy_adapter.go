package testing

import "github.com/cloudfoundry/metric-store-release/src/pkg/rpc"

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
