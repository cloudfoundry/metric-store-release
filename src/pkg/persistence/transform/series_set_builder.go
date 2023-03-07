package transform

import (
	"sort"
	"sync"

	"github.com/influxdata/influxdb/query"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/storage"
)

type seriesData struct {
	labels  labels.Labels
	samples shardSeriesSamples
}

type shardSeriesSamples [][]seriesSample

func (s shardSeriesSamples) Len() int {
	return len(s)
}

func (s shardSeriesSamples) Less(i int, j int) bool {
	return s[i][0].TimeInMilliseconds < s[j][0].TimeInMilliseconds
}

func (s shardSeriesSamples) Swap(i int, j int) {
	s[i], s[j] = s[j], s[i]
}

type SeriesSetBuilder struct {
	data  map[uint64]seriesData
	count int

	sync.Mutex
}

func NewSeriesBuilder() *SeriesSetBuilder {
	return &SeriesSetBuilder{
		data: make(map[uint64]seriesData),
	}
}

func (b *SeriesSetBuilder) AddSeriesPoints(seriesLabels labels.Labels, points []*query.FloatPoint) {
	b.Lock()
	defer b.Unlock()

	if len(points) == 0 {
		return
	}

	seriesID := seriesLabels.Hash()

	samples := make([]seriesSample, len(points))
	for i, point := range points {
		samples[i] = seriesSample{
			TimeInMilliseconds: NanosecondsToMilliseconds(point.Time),
			Value:              point.Value,
		}
	}

	sd, ok := b.data[seriesID]
	if ok {
		sd.samples = append(sd.samples, samples)
		b.data[seriesID] = sd
	} else {
		b.data[seriesID] = seriesData{
			labels:  seriesLabels,
			samples: [][]seriesSample{samples},
		}
	}

	b.count += len(points)
}

func (b *SeriesSetBuilder) Len() int {
	b.Lock()
	defer b.Unlock()

	return b.count
}

func (b *SeriesSetBuilder) SeriesSet() storage.SeriesSet {
	set := &concreteSeriesSet{
		series: []storage.Series{},
	}

	for _, datum := range b.data {
		series := concreteSeries{
			labels: datum.labels,
		}
		sort.Sort(datum.samples)
		for _, shardSamples := range datum.samples {
			series.samples = append(series.samples, shardSamples...)
		}

		set.series = append(set.series, &series)
	}

	return set
}
