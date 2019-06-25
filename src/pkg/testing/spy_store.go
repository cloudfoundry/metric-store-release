package testing

import (
	rpc "github.com/cloudfoundry/metric-store-release/src/pkg/rpc/metricstore_v1"
	"github.com/prometheus/prometheus/pkg/labels"
	"github.com/prometheus/prometheus/storage"
)

type SpyStore struct {
	GetPoints storage.SeriesSet
	GetErr    error

	Name  string
	Start int64
	End   int64
}

func NewSpyStoreReader() *SpyStore {
	return &SpyStore{}
}

func (s *SpyStore) Select(params *storage.SelectParams, labelMatchers ...*labels.Matcher) (storage.SeriesSet, storage.Warnings, error) {
	for _, matcher := range labelMatchers {
		if matcher.Name == "__name__" {
			s.Name = matcher.Value
		}
	}
	s.Start = params.Start
	s.End = params.End

	return s.GetPoints, nil, s.GetErr
}

func (s *SpyStore) LabelNames() (*rpc.PromQL_LabelsQueryResult, error) {
	return nil, nil
}

func (s *SpyStore) LabelValues(*rpc.PromQL_LabelValuesQueryRequest) (*rpc.PromQL_LabelValuesQueryResult, error) {
	return nil, nil
}
