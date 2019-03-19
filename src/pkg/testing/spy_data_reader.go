package testing

import (
	"context"
	"time"

	"github.com/cloudfoundry/metric-store-release/src/pkg/persistence/transform"
	rpc "github.com/cloudfoundry/metric-store-release/src/pkg/rpc/metricstore_v1"
	"github.com/prometheus/prometheus/pkg/labels"
	"github.com/prometheus/prometheus/storage"
)

type SpyDataReader struct {
	ReadStarts []int64
	ReadEnds   []int64

	ReadResults []*rpc.PromQL_Matrix
	ReadErrs    []error

	LabelsResponse      *rpc.PromQL_LabelsQueryResult
	LabelsError         error
	LabelValuesResponse *rpc.PromQL_LabelValuesQueryResult
	LabelValuesError    error
}

func (s *SpyDataReader) Read(ctx context.Context, params *storage.SelectParams, labelMatchers ...*labels.Matcher) (storage.SeriesSet, error) {
	s.ReadStarts = append(s.ReadStarts, params.Start)
	s.ReadEnds = append(s.ReadEnds, params.End)

	if len(s.ReadResults) != len(s.ReadErrs) {
		panic("readResults and readErrs are out of sync")
	}

	if len(s.ReadResults) == 0 {
		panic("there are no more ReadResults to provide, please add in setup")
	}

	r := s.ReadResults[0]
	err := s.ReadErrs[0]

	s.ReadResults = s.ReadResults[1:]
	s.ReadErrs = s.ReadErrs[1:]

	builder := transform.NewSeriesBuilder()
	for _, series := range r.GetSeries() {
		builder.AddPromQLSeries(series)
	}

	// Give ourselves some time to capture runtime metrics
	time.Sleep(time.Millisecond)

	return builder.SeriesSet(), err
}

func (s *SpyDataReader) Labels(ctx context.Context, in *rpc.PromQL_LabelsQueryRequest) (*rpc.PromQL_LabelsQueryResult, error) {
	return s.LabelsResponse, s.LabelsError
}

func (s *SpyDataReader) LabelValues(ctx context.Context, in *rpc.PromQL_LabelValuesQueryRequest) (*rpc.PromQL_LabelValuesQueryResult, error) {
	return s.LabelValuesResponse, s.LabelValuesError
}

func NewSpyDataReader() *SpyDataReader {
	return &SpyDataReader{}
}
