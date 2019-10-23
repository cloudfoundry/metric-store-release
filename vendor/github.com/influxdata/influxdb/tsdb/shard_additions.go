package tsdb

import (
	"context"

	"github.com/influxdata/influxdb/query"
	"github.com/influxdata/influxql"
)

func (s *Shard) CreateIterators(ctx context.Context, m *influxql.Measurement, opt query.IteratorOptions) ([]query.Iterator, error) {
	engine, err := s.Engine()
	if err != nil {
		return nil, err
	}
	return engine.CreateIterators(ctx, m.Name, opt)
}
