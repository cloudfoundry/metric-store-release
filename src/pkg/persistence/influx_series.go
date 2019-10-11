package persistence

import (
	"context"
	"errors"
	"strings"

	"github.com/influxdata/influxdb/query"
	"github.com/influxdata/influxql"
	"github.com/prometheus/prometheus/pkg/labels"
)

// TODO - can we make this a type?
//type SeriesSet map[uint64]labels.Labels

func (t *InfluxAdapter) GetSeriesSet(shardIDs []uint64, measurementName string, filterCondition influxql.Expr) (map[uint64]labels.Labels, error) {
	var seriesIterators []query.Iterator
	// TODO - should this be parallelized?
	for _, shardID := range shardIDs {
		iterator, err := t.createSeriesIterator(shardID, measurementName, filterCondition)
		if err != nil {
			return nil, err
		}
		seriesIterators = append(seriesIterators, iterator)
	}

	seriesIterator := query.NewParallelMergeIterator(seriesIterators, query.IteratorOptions{}, len(seriesIterators))
	if seriesIterator == nil {
		return nil, nil
	}

	defer seriesIterator.Close()

	seriesSet := make(map[uint64]labels.Labels)
	// TODO - switch statement necessary? why is the series iterator a float
	// iterator?
	switch typedSeriesIterator := seriesIterator.(type) {
	case query.FloatIterator:
		for {
			series, err := typedSeriesIterator.Next()
			if err != nil {
				return nil, err
			}
			if series == nil {
				break
			}

			// TODO - use Influx provided package to parse this string
			seriesLabels := labels.NewBuilder(nil)
			for i, label := range strings.Split(series.Aux[0].(string), ",") {
				if i == 0 {
					continue
				}
				labelTuple := strings.Split(label, "=")
				seriesLabels.Set(labelTuple[0], labelTuple[1])
			}

			sl := seriesLabels.Labels()
			seriesSet[sl.Hash()] = sl
		}
	// TODO - should we bail out here? is this defined?
	default:
		return nil, errors.New("non-float iterator")
	}

	return seriesSet, nil
}

func (t *InfluxAdapter) createSeriesIterator(shardId uint64, measurementName string, filterCondition influxql.Expr) (query.Iterator, error) {
	filterWithName := &influxql.BinaryExpr{
		LHS: &influxql.VarRef{Val: "_name"},
		RHS: &influxql.StringLiteral{Val: measurementName},
		Op:  influxql.EQ,
	}

	if filterCondition != nil {
		filterWithName = &influxql.BinaryExpr{
			LHS: filterWithName,
			RHS: filterCondition,
			Op:  influxql.AND,
		}
	}

	return t.influx.ShardGroup([]uint64{shardId}).CreateIterator(
		context.Background(),
		&influxql.Measurement{SystemIterator: "_series"},
		query.IteratorOptions{
			Aux:       []influxql.VarRef{{Val: "key", Type: influxql.String}},
			Condition: filterWithName,
			Limit:     0,
		},
	)
}
