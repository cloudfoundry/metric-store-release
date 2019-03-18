package testing

import (
	"context"
	"errors"

	"github.com/cloudfoundry/metric-store/src/pkg/query"
	rpc "github.com/cloudfoundry/metric-store/src/pkg/rpc/metricstore_v1"
)

type SpyQueryEngine struct {
	InstantQueryDataReader query.DataReader
	RangeQueryDataReader   query.DataReader
	SeriesQueryDataReader  query.DataReader
	RespondWithError       bool
}

func (q *SpyQueryEngine) InstantQuery(ctx context.Context, req *rpc.PromQL_InstantQueryRequest, dataReader query.DataReader) (*rpc.PromQL_InstantQueryResult, error) {
	q.InstantQueryDataReader = dataReader
	if q.RespondWithError {
		return nil, errors.New("instant query engine error")
	}
	return nil, nil
}

func (q *SpyQueryEngine) RangeQuery(ctx context.Context, req *rpc.PromQL_RangeQueryRequest, dataReader query.DataReader) (*rpc.PromQL_RangeQueryResult, error) {
	q.RangeQueryDataReader = dataReader
	if q.RespondWithError {
		return nil, errors.New("range query engine error")
	}
	return nil, nil
}

func (q *SpyQueryEngine) SeriesQuery(ctx context.Context, req *rpc.PromQL_SeriesQueryRequest, dataReader query.DataReader) (*rpc.PromQL_SeriesQueryResult, error) {
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
