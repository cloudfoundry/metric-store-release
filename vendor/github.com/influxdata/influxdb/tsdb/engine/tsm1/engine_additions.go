package tsm1 // import "github.com/influxdata/influxdb/tsdb/engine/tsm1"

import (
	"bytes"
	"context"

	"github.com/influxdata/influxdb/query"
	"github.com/influxdata/influxdb/tsdb"
	_ "github.com/influxdata/influxdb/tsdb/index"
	"github.com/influxdata/influxql"
	"github.com/prometheus/prometheus/pkg/labels"
)

func (e *Engine) CreateIterators(ctx context.Context, measurement string, opt query.IteratorOptions) ([]query.LabeledIterator, error) {
	itrs, err := e.createVarRefIteratorBySeries(ctx, measurement, opt)
	if err != nil {
		return nil, err
	}

	return itrs, nil
}

func WalkTags(buf []byte) []labels.Label {
	result := []labels.Label{}

	if len(buf) == 0 {
		return result
	}

	pos, name := scanTo(buf, 0, ',')

	// it's an empty key, so there are no tags
	if len(name) == 0 {
		return result
	}

	result = append(result, labels.Label{Name: "__name__", Value: string(name)})

	hasEscape := bytes.IndexByte(buf, '\\') != -1
	i := pos + 1
	var key, value []byte
	for {
		if i >= len(buf) {
			break
		}
		i, key = scanTo(buf, i, '=')
		i, value = scanTagValue(buf, i+1)

		if len(value) == 0 {
			continue
		}

		i++

		if hasEscape {
			result = append(result, labels.Label{Name: string(unescapeTag(key)), Value: string(unescapeTag(value))})
			continue
		}
		result = append(result, labels.Label{Name: string(key), Value: string(value)})

	}

	return result
}

// from points
func scanTagValue(buf []byte, i int) (int, []byte) {
	start := i
	for {
		if i >= len(buf) {
			break
		}

		if buf[i] == ',' && buf[i-1] != '\\' {
			break
		}
		i++
	}
	if i > len(buf) {
		return i, nil
	}
	return i, buf[start:i]
}

func scanTo(buf []byte, i int, stop byte) (int, []byte) {
	start := i
	for {
		// reached the end of buf?
		if i >= len(buf) {
			break
		}

		// Reached unescaped stop value?
		if buf[i] == stop && (i == 0 || buf[i-1] != '\\') {
			break
		}
		i++
	}

	return i, buf[start:i]
}

func unescapeTag(in []byte) []byte {
	type escapeSet struct {
		k   [1]byte
		esc [2]byte
	}
	tagEscapeCodes := [...]escapeSet{
		{k: [1]byte{','}, esc: [2]byte{'\\', ','}},
		{k: [1]byte{' '}, esc: [2]byte{'\\', ' '}},
		{k: [1]byte{'='}, esc: [2]byte{'\\', '='}},
	}

	if bytes.IndexByte(in, '\\') == -1 {
		return in
	}

	for i := range tagEscapeCodes {
		c := &tagEscapeCodes[i]
		if bytes.IndexByte(in, c.k[0]) != -1 {
			in = bytes.Replace(in, c.esc[:], c.k[:], -1)
		}
	}
	return in
}

// createVarRefIterator creates an iterator for a variable reference.
func (e *Engine) createVarRefIteratorBySeries(ctx context.Context, measurement string, opt query.IteratorOptions) ([]query.LabeledIterator, error) {
	ref, _ := opt.Expr.(*influxql.VarRef)

	if exists, err := e.index.MeasurementExists([]byte(measurement)); err != nil {
		return nil, err
	} else if !exists {
		return nil, nil
	}

	var (
		tagSets []*query.TagSet
		err     error
	)
	if e.index.Type() == tsdb.InmemIndexName {
		ts := e.index.(indexTagSets)
		tagSets, err = ts.TagSets([]byte(measurement), opt)
	} else {
		indexSet := tsdb.IndexSet{Indexes: []tsdb.Index{e.index}, SeriesFile: e.sfile}
		tagSets, err = indexSet.TagSets(e.sfile, []byte(measurement), opt)
	}

	if err != nil {
		return nil, err
	}

	// Reverse the tag sets if we are ordering by descending.
	if !opt.Ascending {
		for _, t := range tagSets {
			t.Reverse()
		}
	}

	// Calculate tag sets and apply SLIMIT/SOFFSET.
	tagSets = query.LimitTagSets(tagSets, opt.SLimit, opt.SOffset)
	labeledIterators := make([]query.LabeledIterator, 0, len(tagSets))
	if err := func() error {
		for _, t := range tagSets {
			inputs, err := e.createTagSetIterators(ctx, ref, measurement, t, opt)
			if err != nil {
				return err
			} else if len(inputs) == 0 {
				continue
			}

			// If we have a LIMIT or OFFSET and the grouping of the outer query
			// is different than the current grouping, we need to perform the
			// limit on each of the individual series keys instead to improve
			// performance.
			if (opt.Limit > 0 || opt.Offset > 0) && len(opt.Dimensions) != len(opt.GroupBy) {
				for i, input := range inputs {
					inputs[i] = newLimitIterator(input, opt)
				}
			}

			itr, err := query.Iterators(inputs).Merge(opt)
			if err != nil {
				query.Iterators(inputs).Close()
				return err
			}

			// Apply a limit on the merged iterator.
			if opt.Limit > 0 || opt.Offset > 0 {
				if len(opt.Dimensions) == len(opt.GroupBy) {
					// When the final dimensions and the current grouping are
					// the same, we will only produce one series so we can use
					// the faster limit iterator.
					itr = newLimitIterator(itr, opt)
				} else {
					// When the dimensions are different than the current
					// grouping, we need to account for the possibility there
					// will be multiple series. The limit iterator in the
					// influxql package handles that scenario.
					itr = query.NewLimitIterator(itr, opt)
				}
			}

			seriesLabels := WalkTags([]byte(t.SeriesKeys[0]))
			labeledIterators = append(labeledIterators, query.LabeledIterator{
				Iterator: itr,
				Labels:   labels.Labels(seriesLabels),
			})
		}
		return nil
	}(); err != nil {
		for _, labeledIterator := range labeledIterators {
			labeledIterator.Iterator.Close()
		}
		return nil, err
	}

	return labeledIterators, nil
}
