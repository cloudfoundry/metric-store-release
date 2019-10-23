package tsm1 // import "github.com/influxdata/influxdb/tsdb/engine/tsm1"

import (
	"context"

	"github.com/influxdata/influxdb/query"
	_ "github.com/influxdata/influxdb/tsdb/index"
)

func (e *Engine) CreateIterators(ctx context.Context, measurement string, opt query.IteratorOptions) ([]query.Iterator, error) {
	itrs, err := e.createVarRefIterator(ctx, measurement, opt)
	if err != nil {
		return nil, err
	}

	return itrs, nil
}
