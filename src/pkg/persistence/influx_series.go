package persistence

import (
	"fmt"
	"strings"

	"github.com/influxdata/influxdb/query"
	"github.com/influxdata/influxql"
	"github.com/prometheus/prometheus/pkg/labels"
)

func (t *InfluxAdapter) GetSeriesSet(shardIDs []uint64, measurementName string, filterCondition influxql.Expr) (map[uint64]labels.Labels, error) {
	seriesCondition := &influxql.BinaryExpr{
		LHS: &influxql.VarRef{Val: "_name"},
		RHS: &influxql.StringLiteral{Val: measurementName},
		Op:  influxql.EQ,
	}
	if filterCondition != nil {
		seriesCondition = &influxql.BinaryExpr{
			LHS: seriesCondition,
			RHS: filterCondition,
			Op:  influxql.AND,
		}
	}

	seriesIteratorOptions := query.IteratorOptions{
		Aux:       []influxql.VarRef{{Val: "key", Type: influxql.String}},
		Condition: seriesCondition,
		Limit:     0,
	}
	parallelIteratorOptions := query.IteratorOptions{
		Ascending: true,
		Ordered:   true,
	}

	seriesIterator, err := t.ShardwiseParallelIterator(
		shardIDs,
		&influxql.Measurement{SystemIterator: "_series"},
		seriesIteratorOptions,
		parallelIteratorOptions,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create shardwise parallel iterator: %s", err.Error())
	}
	if seriesIterator == nil {
		return map[uint64]labels.Labels{}, nil
	}
	defer seriesIterator.Close()

	seriesSet := make(map[uint64]labels.Labels)
	for {
		series, err := seriesIterator.(query.FloatIterator).Next()
		if err != nil {
			return nil, err
		}
		if series == nil {
			break
		}

		// TODO - use Influx provided package to parse this string
		seriesLabels := labels.NewBuilder(nil)
		for _, label := range strings.Split(series.Aux[0].(string), ",") {
			labelTuple := strings.Split(label, "=")
			if len(labelTuple) != 2 {
				continue
			}
			seriesLabels.Set(labelTuple[0], labelTuple[1])
		}

		sl := seriesLabels.Labels()
		seriesSet[sl.Hash()] = sl
	}

	return seriesSet, nil
}
