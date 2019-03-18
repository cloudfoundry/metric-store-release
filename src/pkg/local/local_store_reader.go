package local

import (
	"fmt"
	"time"

	rpc "github.com/cloudfoundry/metric-store/src/pkg/rpc/metricstore_v1"
	"github.com/prometheus/prometheus/pkg/labels"
	"github.com/prometheus/prometheus/storage"
	"golang.org/x/net/context"
)

type Store interface {
	Get(*storage.SelectParams, ...*labels.Matcher) (storage.SeriesSet, error)
	Labels() (*rpc.PromQL_LabelsQueryResult, error)
	LabelValues(*rpc.PromQL_LabelValuesQueryRequest) (*rpc.PromQL_LabelValuesQueryResult, error)
}
type LocalStoreReader struct {
	store Store
}

func NewLocalStoreReader(store Store) *LocalStoreReader {
	return &LocalStoreReader{
		store: store,
	}
}
func (reader LocalStoreReader) Read(ctx context.Context, params *storage.SelectParams, labelMatchers ...*labels.Matcher) (storage.SeriesSet, error) {
	if params.End != 0 && params.Start > params.End {
		return nil, fmt.Errorf("Start (%d) must be before End (%d)", params.Start, params.End)
	}

	if params.End == 0 {
		params.End = time.Now().UnixNano()
	}

	return reader.store.Get(params, labelMatchers...)
}

func (reader LocalStoreReader) Labels(ctx context.Context, in *rpc.PromQL_LabelsQueryRequest) (*rpc.PromQL_LabelsQueryResult, error) {
	return reader.store.Labels()
}

func (reader LocalStoreReader) LabelValues(ctx context.Context, in *rpc.PromQL_LabelValuesQueryRequest) (*rpc.PromQL_LabelValuesQueryResult, error) {
	return reader.store.LabelValues(in)
}
