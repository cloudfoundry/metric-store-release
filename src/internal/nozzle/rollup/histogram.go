package rollup

import (
	"strconv"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"

	"github.com/cloudfoundry/metric-store-release/src/pkg/logger"
	"github.com/cloudfoundry/metric-store-release/src/pkg/persistence/transform"
	"github.com/cloudfoundry/metric-store-release/src/pkg/rpc"
)

type histogramRollup struct {
	log *logger.Logger

	nodeIndex            string
	rollupTags           []string
	histogramsInInterval map[string]struct{}
	histograms           map[string]prometheus.Histogram

	mu sync.Mutex
}

func NewHistogramRollup(log *logger.Logger, nodeIndex string, rollupTags []string) *histogramRollup {
	return &histogramRollup{
		log:                  log,
		nodeIndex:            nodeIndex,
		rollupTags:           rollupTags,
		histogramsInInterval: make(map[string]struct{}),
		histograms:           make(map[string]prometheus.Histogram),
	}
}

func (r *histogramRollup) Record(sourceId string, tags map[string]string, value int64) {
	key := keyFromTags(r.rollupTags, sourceId, tags)

	r.mu.Lock()
	defer r.mu.Unlock()

	_, found := r.histograms[key]
	if !found {
		r.histograms[key] = prometheus.NewHistogram(prometheus.HistogramOpts{
			Name: GorouterHttpMetricName + "_duration_seconds",
		})
	}

	r.histograms[key].Observe(transform.NanosecondsToSeconds(value))
	r.histogramsInInterval[key] = struct{}{}
}

func (r *histogramRollup) Rollup(timestamp int64) []*PointsBatch {
	var batches []*PointsBatch

	r.mu.Lock()
	defer r.mu.Unlock()

	for k := range r.histogramsInInterval {
		var points []*rpc.Point
		var size int

		labels, err := labelsFromKey(k, r.nodeIndex, r.rollupTags, r.log)
		if err != nil {
			continue
		}

		m := &dto.Metric{}
		_ = r.histograms[k].Write(m)
		histogram := m.GetHistogram()

		for _, bucket := range histogram.Bucket {
			bucketLabels := make(map[string]string)
			for key, value := range labels {
				bucketLabels[key] = value
			}
			bucketLabels["le"] = strconv.FormatFloat(*bucket.UpperBound, 'f', -1, 64)

			bucketPoint := &rpc.Point{
				Name:      GorouterHttpMetricName + "_duration_seconds_bucket",
				Timestamp: timestamp,
				Value:     float64(*bucket.CumulativeCount),
				Labels:    bucketLabels,
			}

			points = append(points, bucketPoint)
			size += bucketPoint.EstimatePointSize()
		}

		bucketLabels := make(map[string]string)
		for key, value := range labels {
			bucketLabels[key] = value
		}
		bucketLabels["le"] = "+Inf"

		bucketInfPoint := &rpc.Point{
			Name:      GorouterHttpMetricName + "_duration_seconds_bucket",
			Timestamp: timestamp,
			Value:     float64(*histogram.SampleCount),
			Labels:    bucketLabels,
		}

		histogramCountPoint := &rpc.Point{
			Name:      GorouterHttpMetricName + "_duration_seconds_count",
			Timestamp: timestamp,
			Value:     float64(*histogram.SampleCount),
			Labels:    labels,
		}
		histogramSumPoint := &rpc.Point{
			Name:      GorouterHttpMetricName + "_duration_seconds_sum",
			Timestamp: timestamp,
			Value:     float64(*histogram.SampleSum),
			Labels:    labels,
		}

		points = append(points, bucketInfPoint, histogramCountPoint, histogramSumPoint)
		size += bucketInfPoint.EstimatePointSize()
		size += histogramCountPoint.EstimatePointSize()
		size += histogramSumPoint.EstimatePointSize()

		batches = append(batches, &PointsBatch{
			Points: points,
			Size:   size,
		})
	}

	r.histogramsInInterval = make(map[string]struct{})

	return batches
}

