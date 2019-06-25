package testing

import (
	"context"
	"errors"

	rpc "github.com/cloudfoundry/metric-store-release/src/pkg/rpc/metricstore_v1"
	"github.com/prometheus/prometheus/storage"
)

type SpyQueryEngine struct {
	InstantQueryDataReader storage.Querier
	RangeQueryDataReader   storage.Querier
	SeriesQueryDataReader  storage.Querier
	RespondWithError       bool
}

func (q *SpyQueryEngine) InstantQuery(ctx context.Context, req *rpc.PromQL_InstantQueryRequest, dataReader storage.Querier) (*rpc.PromQL_InstantQueryResult, error) {
	q.InstantQueryDataReader = dataReader
	if q.RespondWithError {
		return nil, errors.New("instant query engine error")
	}
	return nil, nil
}

func (q *SpyQueryEngine) RangeQuery(ctx context.Context, req *rpc.PromQL_RangeQueryRequest, dataReader storage.Querier) (*rpc.PromQL_RangeQueryResult, error) {
	q.RangeQueryDataReader = dataReader
	if q.RespondWithError {
		return nil, errors.New("range query engine error")
	}
	return nil, nil
}

func (q *SpyQueryEngine) SeriesQuery(ctx context.Context, req *rpc.PromQL_SeriesQueryRequest, dataReader storage.Querier) (*rpc.PromQL_SeriesQueryResult, error) {
	q.SeriesQueryDataReader = dataReader
	if q.RespondWithError {
		return nil, errors.New("series query engine error")
	}
	return nil, nil
}

func (q *SpyQueryEngine) LabelsQuery(ctx context.Context, req *rpc.PromQL_LabelsQueryRequest) (*rpc.PromQL_LabelsQueryResult, error) {
	return nil, nil
}

func NewSpyQueryEngine() *SpyQueryEngine {
	return &SpyQueryEngine{}
}
