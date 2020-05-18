package rollup

import (
	"strconv"
	"sync"
	"time"

	"github.com/cloudfoundry/metric-store-release/src/pkg/logger"
	"github.com/cloudfoundry/metric-store-release/src/pkg/persistence/transform"
	"github.com/cloudfoundry/metric-store-release/src/pkg/rpc"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

type histogramRollup struct {
	log *logger.Logger

	nodeIndex  string
	rollupTags []string
	histograms map[string]prometheus.Histogram
	lastSeen   map[string]int64
	expiration time.Duration

	mu sync.Mutex
}

func NewHistogramRollup(log *logger.Logger, nodeIndex string, rollupTags []string, opts ...HistogramRollupOption) *histogramRollup {
	histogramRollup := &histogramRollup{
		log:        log,
		nodeIndex:  nodeIndex,
		rollupTags: rollupTags,
		histograms: make(map[string]prometheus.Histogram),
		lastSeen:   make(map[string]int64),
		expiration: EXPIRATION,
	}

	for _, o := range opts {
		o(histogramRollup)
	}

	return histogramRollup
}

type HistogramRollupOption func(*histogramRollup)

func WithHistogramRollupExpiration(expiration time.Duration) HistogramRollupOption {
	return func(rollup *histogramRollup) {
		rollup.expiration = expiration
	}
}

func (r *histogramRollup) Record(timestamp int64, sourceId string, tags map[string]string, value int64) {
	key := keyFromTags(r.rollupTags, sourceId, tags)

	r.mu.Lock()
	defer r.mu.Unlock()

	r.lastSeen[key] = timestamp

	_, found := r.histograms[key]
	if !found {
		r.histograms[key] = prometheus.NewHistogram(prometheus.HistogramOpts{})
	}

	r.histograms[key].Observe(transform.NanosecondsToSeconds(value))
}

func (r *histogramRollup) Rollup(timestamp int64) []*PointsBatch {
	var batches []*PointsBatch

	r.mu.Lock()
	defer r.mu.Unlock()

	for k := range r.histograms {
		if expired(r.lastSeen[k], timestamp, r.expiration) {
			delete(r.lastSeen, k)
			delete(r.histograms, k)
			continue
		}

		var points []*rpc.Point
		var size int

		labels, err := labelsFromKey(k, r.nodeIndex, r.rollupTags, r.log)
		if err != nil {
			continue
		}

		m := &dto.Metric{}
		r.histograms[k].Write(m)
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

	return batches
}
