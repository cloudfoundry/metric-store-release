package transform

import (
	"github.com/prometheus/prometheus/model/histogram"
	"sort"

	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/storage"

	prom_storage "github.com/prometheus/prometheus/storage"
	"github.com/prometheus/prometheus/tsdb/chunkenc"
)

type seriesSample struct {
	TimeInMilliseconds int64
	Value              float64
}

// concreteSeriesSet implements storage.SeriesSet.
type concreteSeriesSet struct {
	cur      int
	series   []storage.Series
	err      error
	warnings prom_storage.Warnings
}

func (c *concreteSeriesSet) Next() bool {
	c.cur++
	return c.cur-1 < len(c.series)
}

func (c *concreteSeriesSet) At() storage.Series {
	return c.series[c.cur-1]
}

func (c *concreteSeriesSet) Err() error {
	return c.err
}

func (c *concreteSeriesSet) Warnings() storage.Warnings {
	return c.warnings
}

// concreteSeries implements storage.Series.
type concreteSeries struct {
	labels  labels.Labels
	samples []seriesSample
}

func (c *concreteSeries) Labels() labels.Labels {
	return labels.New(c.labels...)
}

func (c *concreteSeries) Iterator(it chunkenc.Iterator) chunkenc.Iterator {
	if csi, ok := it.(*concreteSeriesIterator); ok {
		csi.reset(c)
		return csi
	}
	return newConcreteSeriersIterator(c)
}

// concreteSeriesIterator implements storage.SeriesIterator.
type concreteSeriesIterator struct {
	cur    int
	series *concreteSeries
}

func newConcreteSeriersIterator(series *concreteSeries) chunkenc.Iterator {
	return &concreteSeriesIterator{
		cur:    -1,
		series: series,
	}
}

// Seek implements storage.SeriesIterator.
func (c *concreteSeriesIterator) Seek(t int64) chunkenc.ValueType {
	c.cur = sort.Search(len(c.series.samples), func(n int) bool {
		return c.series.samples[n].TimeInMilliseconds >= t
	})
	if c.cur < len(c.series.samples) {
		return chunkenc.ValFloat
	}
	return chunkenc.ValNone
}

// At implements storage.SeriesIterator.
func (c *concreteSeriesIterator) At() (t int64, v float64) {
	s := c.series.samples[c.cur]

	return s.TimeInMilliseconds, s.Value
}

// Next implements storage.SeriesIterator.
func (c *concreteSeriesIterator) Next() chunkenc.ValueType {
	c.cur++
	if c.cur < len(c.series.samples) {
		return chunkenc.ValFloat
	}
	return chunkenc.ValNone
}

// Err implements storage.SeriesIterator.
func (c *concreteSeriesIterator) Err() error {
	return nil
}

func (c *concreteSeriesIterator) AtHistogram() (int64, *histogram.Histogram) {
	return c.AtHistogram()
}

func (c *concreteSeriesIterator) AtFloatHistogram() (int64, *histogram.FloatHistogram) {
	return c.AtFloatHistogram()
}

func (c *concreteSeriesIterator) AtT() int64 {
	s := c.series.samples[c.cur]

	return s.TimeInMilliseconds
}

func (c *concreteSeriesIterator) reset(series *concreteSeries) {
	c.cur = -1
	c.series = series
}
