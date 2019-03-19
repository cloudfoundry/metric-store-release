package local

import (
	"context"
	"log"
	"sort"

	"github.com/cloudfoundry/metric-store-release/src/pkg/persistence/transform"
	"github.com/cloudfoundry/metric-store-release/src/pkg/query"
	rpc "github.com/cloudfoundry/metric-store-release/src/pkg/rpc/metricstore_v1"
)

type EgressReverseProxy struct {
	engine      QueryEngine
	localReader query.DataReader
	log         *log.Logger
}

type erpOption func(*EgressReverseProxy)

func NewEgressReverseProxy(
	localReader query.DataReader,
	engine QueryEngine,
	opts ...erpOption,
) *EgressReverseProxy {
	erp := &EgressReverseProxy{
		engine:      engine,
		localReader: localReader,
	}

	for _, o := range opts {
		o(erp)
	}

	return erp
}

func WithLogger(l *log.Logger) erpOption {
	return func(erp *EgressReverseProxy) {
		erp.log = l
	}
}

type QueryEngine interface {
	InstantQuery(context.Context, *rpc.PromQL_InstantQueryRequest, query.DataReader) (*rpc.PromQL_InstantQueryResult, error)
	RangeQuery(context.Context, *rpc.PromQL_RangeQueryRequest, query.DataReader) (*rpc.PromQL_RangeQueryResult, error)
	SeriesQuery(context.Context, *rpc.PromQL_SeriesQueryRequest, query.DataReader) (*rpc.PromQL_SeriesQueryResult, error)
}

func (erp *EgressReverseProxy) InstantQuery(ctx context.Context, req *rpc.PromQL_InstantQueryRequest) (*rpc.PromQL_InstantQueryResult, error) {
	return erp.engine.InstantQuery(ctx, req, erp.localReader)
}

func (erp *EgressReverseProxy) RangeQuery(ctx context.Context, req *rpc.PromQL_RangeQueryRequest) (*rpc.PromQL_RangeQueryResult, error) {
	return erp.engine.RangeQuery(ctx, req, erp.localReader)
}

func (erp *EgressReverseProxy) SeriesQuery(ctx context.Context, req *rpc.PromQL_SeriesQueryRequest) (*rpc.PromQL_SeriesQueryResult, error) {
	return erp.engine.SeriesQuery(ctx, req, erp.localReader)
}

func (erp *EgressReverseProxy) LabelsQuery(ctx context.Context, req *rpc.PromQL_LabelsQueryRequest) (*rpc.PromQL_LabelsQueryResult, error) {
	result, err := erp.localReader.Labels(ctx, req)

	if result != nil {
		result.Labels = labelFormatter(result.Labels)
	}

	return result, err
}

func labelFormatter(labels []string) []string {
	labels = append(labels, transform.MEASUREMENT_NAME)
	sort.StringSlice(labels).Sort()

	return labels
}

func (erp *EgressReverseProxy) LabelValuesQuery(ctx context.Context, req *rpc.PromQL_LabelValuesQueryRequest) (*rpc.PromQL_LabelValuesQueryResult, error) {
	return erp.localReader.LabelValues(ctx, req)
}
