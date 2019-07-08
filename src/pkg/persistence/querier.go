package persistence

import (
	"fmt"
	"time"

	"github.com/cloudfoundry/metric-store-release/src/pkg/persistence/transform"
	"github.com/prometheus/prometheus/pkg/labels"
	"github.com/prometheus/prometheus/storage"
)

type Querier struct {
	adapter *InfluxAdapter
	metrics querierMetrics
}

type querierMetrics struct {
	incNumGetErrors func(delta uint64)
}

func NewQuerier(adapter *InfluxAdapter, m MetricsInitializer) *Querier {
	return &Querier{
		adapter: adapter,
		metrics: querierMetrics{
			incNumGetErrors: m.NewCounter("metric_store_num_get_errors"),
		},
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
		if labelMatcher.Name == "__name__" {
			name = labelMatcher.Value
			labelMatchers = append(labelMatchers[:index], labelMatchers[index+1:]...)
			break
		}
	}

	startTimeInNanoseconds := transform.MillisecondsToNanoseconds(params.Start)
	endTimeInNanoseconds := transform.MillisecondsToNanoseconds(params.End) - 1

	builder, err := q.adapter.GetPoints(name, startTimeInNanoseconds, endTimeInNanoseconds, labelMatchers)
	if err != nil {
		q.metrics.incNumGetErrors(1)
		return nil, nil, err
	}

	return builder.SeriesSet(), nil, nil
}

func (q *Querier) LabelNames() ([]string, error) {
	distinctKeys := make(map[string]struct{})

	tagKeys := q.adapter.AllTagKeys()
	for _, tagKey := range tagKeys {
		distinctKeys[tagKey] = struct{}{}
	}

	var labels []string
	for k := range distinctKeys {
		labels = append(labels, k)
	}

	return labels, nil
}

func (q *Querier) LabelValues(name string) ([]string, error) {
	distinctValues := make(map[string]struct{})

	if name == transform.MEASUREMENT_NAME {
		values := q.adapter.AllMeasurementNames()
		return values, nil
	}

	tagValues := q.adapter.AllTagValues(name)
	for _, tagValue := range tagValues {
		distinctValues[tagValue] = struct{}{}
	}

	var values []string
	for v := range distinctValues {
		values = append(values, v)
	}

	return values, nil
}

func (q *Querier) Close() error {
	return nil
}
