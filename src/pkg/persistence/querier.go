package persistence

import (
	"context"
	"errors"
	"fmt"
	"github.com/prometheus/prometheus/util/annotations"
	"sort"
	"time"

	"github.com/cloudfoundry/metric-store-release/src/internal/metrics"
	"github.com/cloudfoundry/metric-store-release/src/pkg/persistence/transform"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/storage"
)

type Querier struct {
	adapter *InfluxAdapter
	metrics metrics.Registrar
}

func NewQuerier(adapter *InfluxAdapter, metrics metrics.Registrar) *Querier {
	return &Querier{
		adapter: adapter,
		metrics: metrics,
	}
}

func (q *Querier) Select(ctx context.Context, _ bool, params *storage.SelectHints, labelMatchers ...*labels.Matcher) storage.SeriesSet {
	if params == nil {
		params = &storage.SelectHints{
			Start: 0,
			End:   time.Now().UnixNano() / int64(time.Millisecond),
		}
	}
	if params.End != 0 && params.Start > params.End {
		return storage.ErrSeriesSet(fmt.Errorf("Start (%d) must be before End (%d)",
			params.Start, params.End))
	}

	if params.End == 0 {
		params.End = time.Now().UnixNano() / int64(time.Millisecond)
	}

	var name string
	for index, labelMatcher := range labelMatchers {
		if labelMatcher.Name == labels.MetricName {
			if labelMatcher.Type != labels.MatchEqual {
				return storage.ErrSeriesSet(errors.New("only strict equality is " +
					"supported for metric names"))
			}

			name = labelMatcher.Value
			labelMatchers = append(labelMatchers[:index], labelMatchers[index+1:]...)
			break
		}
	}

	startTimeInNanoseconds := transform.MillisecondsToNanoseconds(params.Start)
	endTimeInNanoseconds := transform.MillisecondsToNanoseconds(params.End) - 1

	builder, err := q.adapter.GetPoints(ctx, name, startTimeInNanoseconds, endTimeInNanoseconds, labelMatchers)
	if err != nil {
		q.metrics.Inc(metrics.MetricStoreReadErrorsTotal)
		return storage.ErrSeriesSet(err)
	}

	return builder.SeriesSet()
}

func (q *Querier) LabelNames(ctx context.Context, matchers ...*labels.Matcher) ([]string, annotations.Annotations, error) {
	distinctKeys := make(map[string]struct{})

	tagKeys := q.adapter.AllTagKeys(ctx)
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

func (q *Querier) LabelValues(ctx context.Context, name string, matchers ...*labels.Matcher) ([]string, annotations.Annotations, error) {
	distinctValues := make(map[string]struct{})

	if name == labels.MetricName {
		values := q.adapter.AllMeasurementNames()
		return values, nil, nil
	}

	tagValues := q.adapter.AllTagValues(ctx, name)
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
