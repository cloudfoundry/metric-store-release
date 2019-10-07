package persistence

import (
	"fmt"
	"io"
	"sync"

	"github.com/influxdata/influxdb/query"
	"github.com/influxdata/influxql"
)

type floatPointError struct {
	point *query.FloatPoint
	err   error
}

type floatParallelIterator struct {
	input query.FloatIterator
	ch    chan floatPointError

	once    sync.Once
	closing chan struct{}
	wg      sync.WaitGroup
}

// Stats returns stats from the underlying iterator.
func (itr *floatParallelIterator) Stats() query.IteratorStats { return itr.input.Stats() }

// Close closes the underlying iterators.
func (itr *floatParallelIterator) Close() error {
	itr.once.Do(func() { close(itr.closing) })
	itr.wg.Wait()
	return itr.input.Close()
}

// Next returns the next point from the iterator.
func (itr *floatParallelIterator) Next() (*query.FloatPoint, error) {
	v, ok := <-itr.ch
	if !ok {
		return nil, io.EOF
	}
	return v.point, v.err
}

// monitor runs in a separate goroutine and actively pulls the next point.
func (itr *floatParallelIterator) monitor() {
	defer close(itr.ch)
	defer itr.wg.Done()

	for {
		// Read next point.
		p, err := itr.input.Next()
		if p != nil {
			p = p.Clone()
		}

		select {
		case <-itr.closing:
			return
		case itr.ch <- floatPointError{point: p, err: err}:
		}
	}
}

func newFloatParallelIterator(input query.FloatIterator) *floatParallelIterator {
	itr := &floatParallelIterator{
		input:   input,
		ch:      make(chan floatPointError, 256),
		closing: make(chan struct{}),
	}
	itr.wg.Add(1)
	go itr.monitor()
	return itr
}

func newParallelIterator(input query.Iterator) query.Iterator {
	if input == nil {
		return nil
	}

	switch itr := input.(type) {
	case query.FloatIterator:
		return newFloatParallelIterator(itr)
	default:
		panic(fmt.Sprintf("unsupported parallel iterator type: %T", itr))
	}
}

// Iterators represents a list of iterators.
type Iterators []query.Iterator

// Stats returns the aggregation of all iterator stats.
func (a Iterators) Stats() query.IteratorStats {
	var stats query.IteratorStats
	for _, itr := range a {
		stats.Add(itr.Stats())
	}
	return stats
}

// Close closes all iterators.
func (a Iterators) Close() error {
	for _, itr := range a {
		itr.Close()
	}
	return nil
}

// filterNonNil returns a slice of iterators that removes all nil iterators.
func (a Iterators) filterNonNil() []query.Iterator {
	other := make([]query.Iterator, 0, len(a))
	for _, itr := range a {
		if itr == nil {
			continue
		}
		other = append(other, itr)
	}
	return other
}

// dataType determines what slice type this set of iterators should be.
// An iterator type is chosen by looking at the first element in the slice
// and then returning the data type for that iterator.
func (a Iterators) dataType() influxql.DataType {
	if len(a) == 0 {
		return influxql.Unknown
	}

	switch a[0].(type) {
	case query.FloatIterator:
		return influxql.Float
	default:
		return influxql.Unknown
	}
}

// newFloatIterators converts a slice of Iterator to a slice of FloatIterator.
// Drop and closes any iterator in itrs that is not a FloatIterator and cannot
// be cast to a FloatIterator.
func newFloatIterators(itrs []query.Iterator) []query.FloatIterator {
	a := make([]query.FloatIterator, 0, len(itrs))
	for _, itr := range itrs {
		switch itr := itr.(type) {
		case query.FloatIterator:
			a = append(a, itr)
		default:
			itr.Close()
		}
	}
	return a
}

// coerce forces an array of iterators to be a single type.
// Iterators that are not of the same type as the first element in the slice
// will be closed and dropped.
func (a Iterators) coerce() interface{} {
	typ := a.dataType()
	switch typ {
	case influxql.Float:
		return newFloatIterators(a)
	}
	return a
}

// Merge combines all iterators into a single iterator.
// A sorted merge iterator or a merge iterator can be used based on opt.
func (a Iterators) Merge(opt query.IteratorOptions) (query.Iterator, error) {
	// Check if this is a call expression.
	call, ok := opt.Expr.(*influxql.Call)

	// Merge into a single iterator.
	if !ok && opt.MergeSorted() {
		itr := query.NewSortedMergeIterator(a, opt)
		if itr != nil && opt.InterruptCh != nil {
			itr = query.NewInterruptIterator(itr, opt.InterruptCh)
		}
		return itr, nil
	}

	// We do not need an ordered output so use a merge iterator.
	itr := query.NewMergeIterator(a, opt)
	if itr == nil {
		return nil, nil
	}

	if opt.InterruptCh != nil {
		itr = query.NewInterruptIterator(itr, opt.InterruptCh)
	}

	if !ok {
		// This is not a call expression so do not use a call iterator.
		return itr, nil
	}

	// When merging the count() function, use sum() to sum the counted points.
	if call.Name == "count" {
		opt.Expr = &influxql.Call{
			Name: "sum",
			Args: call.Args,
		}
	}
	return query.NewCallIterator(itr, opt)
}

func NewParallelSortedMergeIterator(inputs []query.Iterator, opt query.IteratorOptions, parallelism int) query.Iterator {
	inputs = Iterators(inputs).filterNonNil()
	if len(inputs) == 0 {
		return nil
	} else if len(inputs) == 1 {
		return inputs[0]
	}

	// Limit parallelism to the number of inputs.
	if len(inputs) < parallelism {
		parallelism = len(inputs)
	}

	// Determine the number of inputs per output iterator.
	n := len(inputs) / parallelism

	// Group iterators together.
	outputs := make([]query.Iterator, parallelism)
	for i := range outputs {
		var slice []query.Iterator
		if i < len(outputs)-1 {
			slice = inputs[i*n : (i+1)*n]
		} else {
			slice = inputs[i*n:]
		}

		outputs[i] = newParallelIterator(query.NewSortedMergeIterator(slice, opt))
	}

	// Merge all groups together.
	return query.NewSortedMergeIterator(outputs, opt)
}
