package query

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/cloudfoundry/metric-store-release/src/pkg/persistence/transform"
	rpc "github.com/cloudfoundry/metric-store-release/src/pkg/rpc/metricstore_v1"
	"github.com/prometheus/prometheus/pkg/labels"
	"github.com/prometheus/prometheus/promql"
	"github.com/prometheus/prometheus/storage"
)

type Store interface {
	Select(*storage.SelectParams, ...*labels.Matcher) (storage.SeriesSet, storage.Warnings, error)
	LabelNames() ([]string, error)
	LabelValues(string) ([]string, error)
}

type Engine struct {
	log          *log.Logger
	queryTimeout time.Duration

	failureCounter    func(uint64)
	instantQueryTimer func(float64)
	rangeQueryTimer   func(float64)
}

func NewEngine(m Metrics, opts ...EngineOption) *Engine {
	engine := &Engine{
		instantQueryTimer: m.NewGauge("metric_store_promql_instant_query_time", "milliseconds"),
		rangeQueryTimer:   m.NewGauge("metric_store_promql_range_query_time", "milliseconds"),
		failureCounter:    m.NewCounter("metric_store_promql_timeout"),
	}

	for _, o := range opts {
		o(engine)
	}

	return engine
}

type EngineOption func(*Engine)

func WithLogger(l *log.Logger) EngineOption {
	return func(engine *Engine) {
		engine.log = l
	}
}

func WithQueryTimeout(queryTimeout time.Duration) EngineOption {
	return func(engine *Engine) {
		engine.queryTimeout = queryTimeout
	}
}

type Metrics interface {
	NewCounter(name string) func(delta uint64)
	NewGauge(name, unit string) func(value float64)
}

func (q *Engine) InstantQuery(ctx context.Context, req *rpc.PromQL_InstantQueryRequest, dataReader Store) (*rpc.PromQL_InstantQueryResult, error) {
	queryable, engine := q.createPromQLEngine(dataReader)

	var err error

	requestTimeInSeconds := time.Now()
	if req.Time != "" {
		requestTimeInSeconds, err = ParseTime(req.Time)
		if err != nil {
			return nil, err
		}
	}

	queryStartTime := time.Now()
	qq, err := engine.NewInstantQuery(queryable, req.Query, requestTimeInSeconds)
	if err != nil {
		return nil, err
	}

	r := qq.Exec(ctx)

	q.instantQueryTimer(float64(time.Since(queryStartTime) / time.Millisecond))

	if queryable.err != nil {
		q.failureCounter(1)
		return nil, queryable.err
	}

	if r.Err != nil {
		return nil, r.Err
	}

	return q.toInstantQueryResult(r), nil
}

func (q *Engine) toInstantQueryResult(r *promql.Result) *rpc.PromQL_InstantQueryResult {
	switch r.Value.Type() {
	case promql.ValueTypeScalar:
		s := r.Value.(promql.Scalar)
		return &rpc.PromQL_InstantQueryResult{
			Result: &rpc.PromQL_InstantQueryResult_Scalar{
				Scalar: &rpc.PromQL_Point{
					Time:  s.T,
					Value: s.V,
				},
			},
		}

	case promql.ValueTypeVector:
		var samples []*rpc.PromQL_Sample
		for _, s := range r.Value.(promql.Vector) {
			metric := make(map[string]string)
			for _, label := range s.Metric {
				metric[label.Name] = label.Value
			}
			samples = append(samples, &rpc.PromQL_Sample{
				Metric: metric,
				Point: &rpc.PromQL_Point{
					Time:  s.T,
					Value: s.V,
				},
			})
		}

		return &rpc.PromQL_InstantQueryResult{
			Result: &rpc.PromQL_InstantQueryResult_Vector{
				Vector: &rpc.PromQL_Vector{
					Samples: samples,
				},
			},
		}

	case promql.ValueTypeMatrix:
		var series []*rpc.PromQL_Series
		for _, s := range r.Value.(promql.Matrix) {
			metric := make(map[string]string)
			for _, label := range s.Metric {
				metric[label.Name] = label.Value
			}
			var points []*rpc.PromQL_Point
			for _, p := range s.Points {
				points = append(points, &rpc.PromQL_Point{
					Time:  p.T,
					Value: p.V,
				})
			}

			series = append(series, &rpc.PromQL_Series{
				Metric: metric,
				Points: points,
			})
		}

		return &rpc.PromQL_InstantQueryResult{
			Result: &rpc.PromQL_InstantQueryResult_Matrix{
				Matrix: &rpc.PromQL_Matrix{
					Series: series,
				},
			},
		}

	default:
		q.log.Panicf("QueryEngine: unknown type: %s", r.Value.Type())
		return nil
	}
}

func (q *Engine) RangeQuery(ctx context.Context, req *rpc.PromQL_RangeQueryRequest, dataReader Store) (*rpc.PromQL_RangeQueryResult, error) {
	queryable, engine := q.createPromQLEngine(dataReader)

	var err error

	requestStartInSeconds, err := ParseTime(req.Start)
	if err != nil {
		return nil, fmt.Errorf("couldn't parse start: %s", err)
	}

	requestEndInSeconds, err := ParseTime(req.End)
	if err != nil {
		return nil, fmt.Errorf("couldn't parse end: %s", err)
	}

	interval, err := ParseStep(req.Step)
	if err != nil {
		return nil, fmt.Errorf("couldn't parse step: %s", err)
	}

	queryStartTime := time.Now()
	qq, err := engine.NewRangeQuery(queryable, req.Query, requestStartInSeconds, requestEndInSeconds, interval)
	if err != nil {
		return nil, err
	}

	r := qq.Exec(ctx)

	q.rangeQueryTimer(float64(time.Since(queryStartTime) / time.Millisecond))

	if queryable.err != nil {
		q.failureCounter(1)
		return nil, queryable.err
	}

	if r.Err != nil {
		return nil, r.Err
	}

	return q.toRangeQueryResult(r), nil
}

func (q *Engine) toRangeQueryResult(r *promql.Result) *rpc.PromQL_RangeQueryResult {
	switch r.Value.Type() {
	case promql.ValueTypeMatrix:
		var series []*rpc.PromQL_Series
		for _, s := range r.Value.(promql.Matrix) {
			metric := make(map[string]string)
			for _, label := range s.Metric {
				metric[label.Name] = label.Value
			}
			var points []*rpc.PromQL_Point
			for _, p := range s.Points {
				points = append(points, &rpc.PromQL_Point{
					Time:  p.T,
					Value: p.V,
				})
			}

			series = append(series, &rpc.PromQL_Series{
				Metric: metric,
				Points: points,
			})
		}

		return &rpc.PromQL_RangeQueryResult{
			Result: &rpc.PromQL_RangeQueryResult_Matrix{
				Matrix: &rpc.PromQL_Matrix{
					Series: series,
				},
			},
		}

	default:
		q.log.Panicf("QueryEngine: unknown type: %s", r.Value.Type())
		return nil
	}
}

func (e *Engine) createPromQLEngine(dataReader Store) (*MetricStoreQueryable, *promql.Engine) {
	msq := &MetricStoreQueryable{
		DataReader: dataReader,
	}

	engineOpts := promql.EngineOpts{
		MaxConcurrent: 10,
		MaxSamples:    1e6,
		Timeout:       e.queryTimeout,
	}
	engine := promql.NewEngine(engineOpts)

	return msq, engine
}

func (q *Engine) SeriesQuery(ctx context.Context, req *rpc.PromQL_SeriesQueryRequest, dataReader Store) (*rpc.PromQL_SeriesQueryResult, error) {
	var err error

	requestStartInSeconds, err := ParseTime(req.Start)
	if err != nil {
		return nil, fmt.Errorf("couldn't parse start: %s", err)
	}

	requestEndInSeconds, err := ParseTime(req.End)
	if err != nil {
		return nil, fmt.Errorf("couldn't parse end: %s", err)
	}

	var validMatchers []string
	for _, matcher := range req.Match {
		if matcher != "" {
			validMatchers = append(validMatchers, matcher)
		}
	}

	if len(validMatchers) == 0 {
		return nil, errors.New("requires at least one matcher")
	}

	params := &storage.SelectParams{
		Start: transform.SecondsToMilliseconds(requestStartInSeconds.Unix()),
		End:   transform.SecondsToMilliseconds(requestEndInSeconds.Unix()),
	}

	var seriesSets []storage.SeriesSet
	for _, s := range validMatchers {
		matchers, err := promql.ParseMetricSelector(s)
		if err != nil {
			return nil, err
		}

		seriesSet, _, err := dataReader.Select(params, matchers...)
		if err != nil {
			return nil, err
		}

		seriesSets = append(seriesSets, seriesSet)
	}

	return q.toSeriesQueryResult(seriesSets), nil
}

func (q *Engine) toSeriesQueryResult(seriesSets []storage.SeriesSet) *rpc.PromQL_SeriesQueryResult {
	var series []*rpc.PromQL_SeriesInfo

	set := storage.NewMergeSeriesSet(seriesSets, nil)
	for set.Next() {
		series = append(series, &rpc.PromQL_SeriesInfo{
			Info: set.At().Labels().Map(),
		})
	}

	return &rpc.PromQL_SeriesQueryResult{Series: series}
}

type MetricStoreQueryable struct {
	DataReader Store
	err        error
}

func (q *MetricStoreQueryable) Querier(ctx context.Context, minTimeInMilliseconds int64, maxTimeInMilliseconds int64) (storage.Querier, error) {
	return &MetricStoreQuerier{
		ctx:        ctx,
		start:      transform.MillisecondsToTime(minTimeInMilliseconds),
		end:        transform.MillisecondsToTime(maxTimeInMilliseconds),
		dataReader: q.DataReader,
		queryable:  q,
	}, nil
}

// Querier provides reading access to time series data.
type MetricStoreQuerier struct {
	ctx        context.Context
	start      time.Time
	end        time.Time
	dataReader Store
	queryable  *MetricStoreQueryable
}

// LabelNames returns all the unique label names present in the block in sorted order.
func (querier *MetricStoreQuerier) LabelNames() ([]string, error) {
	panic("not implemented")
}

// LabelValues returns all potential values for a label name.
func (querier *MetricStoreQuerier) LabelValues(string) ([]string, error) {
	panic("not implemented")
}

// Close releases the resources of the Querier.
func (querier *MetricStoreQuerier) Close() error {
	return nil
}

func (querier *MetricStoreQuerier) Select(params *storage.SelectParams, labelMatchers ...*labels.Matcher) (storage.SeriesSet, storage.Warnings, error) {
	seriesSet, _, err := querier.dataReader.Select(params, labelMatchers...)
	if err != nil {
		querier.queryable.err = err
		return nil, nil, err
	}

	return seriesSet, nil, nil
}
