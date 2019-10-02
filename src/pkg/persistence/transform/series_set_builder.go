package transform

import (
	"github.com/influxdata/influxdb/query"
	"github.com/prometheus/prometheus/pkg/labels"
	"github.com/prometheus/prometheus/storage"
)

type seriesData struct {
	labels  labels.Labels
	samples []seriesSample
}

func NewSeriesBuilder() *SeriesSetBuilder {
	return &SeriesSetBuilder{
		data: make(map[uint64]seriesData),
	}
}

type SeriesSetBuilder struct {
	data map[uint64]seriesData
	// TODO - maybe a better way to implement the 'source of truth'
	count int
}

func (b *SeriesSetBuilder) AddInfluxPoint(point *query.FloatPoint, fields []string) {
	sample, labels := SeriesDataFromInfluxPoint(point, fields)
	b.add(sample, labels)
}

func (builder *SeriesSetBuilder) Len() int {
	return builder.count
}

func (b *SeriesSetBuilder) SeriesSet() storage.SeriesSet {
	set := &concreteSeriesSet{
		series: []storage.Series{},
	}

	for _, data := range b.data {
		set.series = append(set.series, &concreteSeries{
			labels:  data.labels,
			samples: data.samples,
		})
	}

	return set
}

func (b *SeriesSetBuilder) add(sample seriesSample, labels labels.Labels) {
	seriesID := labels.Hash()
	d, ok := b.data[seriesID]

	if !ok {
		b.data[seriesID] = seriesData{
			labels:  labels,
			samples: make([]seriesSample, 0),
		}

		d = b.data[seriesID]
	}
	d.samples = append(d.samples, sample)
	b.data[seriesID] = d
	b.count++
}
