package persistence

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/cloudfoundry/metric-store-release/src/internal/debug"
	"github.com/cloudfoundry/metric-store-release/src/pkg/persistence/transform"
	"github.com/prometheus/prometheus/pkg/labels"
	"github.com/prometheus/prometheus/storage"
)

type Querier struct {
	ctx     context.Context
	adapter *InfluxAdapter
	metrics debug.MetricRegistrar
}

func NewQuerier(ctx context.Context, adapter *InfluxAdapter, metrics debug.MetricRegistrar) *Querier {
	return &Querier{
		ctx:     ctx,
		adapter: adapter,
		metrics: metrics,
	}
}

func (q *Querier) Select(params *storage.SelectParams, labelMatchers ...*labels.Matcher) (storage.SeriesSet, storage.Warnings, error) {
	if params == nil {
		params = &storage.SelectParams{
			Start: 0,
			End:   time.Now().UnixNano() / int64(time.Millisecond),
		}
	}
	if params.End != 0 && params.Start > params.End {
		return nil, nil, fmt.Errorf("Start (%d) must be before End (%d)", params.Start, params.End)
	}

	if params.End == 0 {
		params.End = time.Now().UnixNano() / int64(time.Millisecond)
	}

	var name string
	for index, labelMatcher := range labelMatchers {
		if labelMatcher.Name == labels.MetricName {
			if labelMatcher.Type != labels.MatchEqual {
				return nil, nil, errors.New("only strict equality is supported for metric names")
			}

			name = labelMatcher.Value
			labelMatchers = append(labelMatchers[:index], labelMatchers[index+1:]...)
			break
		}
	}

	startTimeInNanoseconds := transform.MillisecondsToNanoseconds(params.Start)
	endTimeInNanoseconds := transform.MillisecondsToNanoseconds(params.End) - 1

	builder, err := q.adapter.GetPoints(q.ctx, name, startTimeInNanoseconds, endTimeInNanoseconds, labelMatchers)
	if err != nil {
		q.metrics.Inc(debug.MetricStoreReadErrorsTotal)
		return nil, nil, err
	}

	return builder.SeriesSet(), nil, nil
}

func (q *Querier) LabelNames() ([]string, storage.Warnings, error) {
	distinctKeys := make(map[string]struct{})

	tagKeys := q.adapter.AllTagKeys()
	for _, tagKey := range tagKeys {
		distinctKeys[tagKey] = struct{}{}
	}

	var labelNames []string

	for k := range distinctKeys {
		labelNames = append(labelNames, k)
	}

	labelNames = append(labelNames, labels.MetricName)
	sort.Strings(labelNames)

	return labelNames, nil, nil
}

func (q *Querier) LabelValues(name string) ([]string, storage.Warnings, error) {
	distinctValues := make(map[string]struct{})

	if name == labels.MetricName {
		values := q.adapter.AllMeasurementNames()
		return values, nil, nil
	}

	tagValues := q.adapter.AllTagValues(name)
	for _, tagValue := range tagValues {
		distinctValues[tagValue] = struct{}{}
	}

	var values []string
	for v := range distinctValues {
		values = append(values, v)
	}
	sort.Strings(values)

	return values, nil, nil
}

func (q *Querier) Close() error {
	return nil
}
