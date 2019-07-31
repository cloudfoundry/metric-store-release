package transform

import (
	"sort"

	"github.com/influxdata/influxdb/query"
	"github.com/prometheus/prometheus/pkg/labels"
	"github.com/prometheus/prometheus/storage"
)

type seriesData struct {
	labels  map[string]string
	samples []seriesSample
}

func NewSeriesBuilder() *SeriesSetBuilder {
	return &SeriesSetBuilder{
		data: make(map[string]seriesData),
	}
}

type SeriesSetBuilder struct {
	data map[string]seriesData
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
			labels:  convertToLabels(data.labels),
			samples: data.samples,
		})
	}

	return set
}

func (b *SeriesSetBuilder) add(sample seriesSample, labels map[string]string) {
	seriesID := b.getSeriesID(labels)
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

func convertToLabels(tags map[string]string) []labels.Label {
	ls := make([]labels.Label, 0, len(tags))
	for n, v := range tags {
		ls = append(ls, labels.Label{
			Name:  n,
			Value: v,
		})
	}
	return ls
}

func (b *SeriesSetBuilder) getSeriesID(tags map[string]string) string {
	keys := make([]string, 0, len(tags))
	for k := range tags {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	var seriesID string
	for _, k := range keys {
		seriesID = seriesID + "-" + k + "-" + tags[k]
	}

	return seriesID
}
