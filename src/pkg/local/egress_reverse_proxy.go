package local

import (
	"context"
	"log"
	"sort"

	"github.com/cloudfoundry/metric-store-release/src/pkg/persistence/transform"
	rpc "github.com/cloudfoundry/metric-store-release/src/pkg/rpc/metricstore_v1"
	"github.com/prometheus/prometheus/storage"
)

type EgressReverseProxy struct {
	engine  QueryEngine
	storage storage.Storage
	log     *log.Logger
}

type erpOption func(*EgressReverseProxy)

func NewEgressReverseProxy(
	storage storage.Storage,
	engine QueryEngine,
	opts ...erpOption,
) *EgressReverseProxy {
	erp := &EgressReverseProxy{
		engine:  engine,
		storage: storage,
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
	InstantQuery(context.Context, *rpc.PromQL_InstantQueryRequest, storage.Storage) (*rpc.PromQL_InstantQueryResult, error)
	RangeQuery(context.Context, *rpc.PromQL_RangeQueryRequest, storage.Storage) (*rpc.PromQL_RangeQueryResult, error)
	SeriesQuery(context.Context, *rpc.PromQL_SeriesQueryRequest, storage.Storage) (*rpc.PromQL_SeriesQueryResult, error)
}

func (erp *EgressReverseProxy) InstantQuery(ctx context.Context, req *rpc.PromQL_InstantQueryRequest) (*rpc.PromQL_InstantQueryResult, error) {
	return erp.engine.InstantQuery(ctx, req, erp.storage)
}

func (erp *EgressReverseProxy) RangeQuery(ctx context.Context, req *rpc.PromQL_RangeQueryRequest) (*rpc.PromQL_RangeQueryResult, error) {
	return erp.engine.RangeQuery(ctx, req, erp.storage)
}

func (erp *EgressReverseProxy) SeriesQuery(ctx context.Context, req *rpc.PromQL_SeriesQueryRequest) (*rpc.PromQL_SeriesQueryResult, error) {
	return erp.engine.SeriesQuery(ctx, req, erp.storage)
}

func (erp *EgressReverseProxy) LabelsQuery(ctx context.Context, req *rpc.PromQL_LabelsQueryRequest) (*rpc.PromQL_LabelsQueryResult, error) {
	result := &rpc.PromQL_LabelsQueryResult{}

	querier, err := erp.storage.Querier(ctx, 0, 0)
	if err != nil {
		return result, err
	}

	labels, err := querier.LabelNames()

	if labels != nil {
		result.Labels = labelFormatter(labels)
	}

	return result, err
}

func labelFormatter(labels []string) []string {
	labels = append(labels, transform.MEASUREMENT_NAME)
	sort.StringSlice(labels).Sort()

	return labels
}

func (erp *EgressReverseProxy) LabelValuesQuery(ctx context.Context, req *rpc.PromQL_LabelValuesQueryRequest) (*rpc.PromQL_LabelValuesQueryResult, error) {
	querier, err := erp.storage.Querier(ctx, 0, 0)
	if err != nil {
		return &rpc.PromQL_LabelValuesQueryResult{}, err
	}

	values, err := querier.LabelValues(req.GetName())

	result := &rpc.PromQL_LabelValuesQueryResult{
		Values: values,
	}

	return result, err
}
