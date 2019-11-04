package rollup

import (
	"strconv"
	"sync"
	"time"

	"github.com/cloudfoundry/metric-store-release/src/pkg/logger"
	"github.com/cloudfoundry/metric-store-release/src/pkg/rpc"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

type histogramRollup struct {
	log *logger.Logger

	nodeIndex            string
	metricName           string
	rollupTags           []string
	histogramsInInterval map[string]struct{}
	histograms           map[string]prometheus.Histogram

	mu sync.Mutex
}

func NewHistogramRollup(log *logger.Logger, nodeIndex, metricName string, rollupTags []string) *histogramRollup {
	return &histogramRollup{
		log:                  log,
		nodeIndex:            nodeIndex,
		metricName:           metricName,
		rollupTags:           rollupTags,
		histogramsInInterval: make(map[string]struct{}),
		histograms:           make(map[string]prometheus.Histogram),
	}
}

func (h *histogramRollup) Record(sourceId string, tags map[string]string, timeInNanoseconds int64) {
	key := keyFromTags(h.rollupTags, sourceId, tags)

	h.mu.Lock()
	defer h.mu.Unlock()

	_, found := h.histograms[key]
	if !found {
		h.histograms[key] = prometheus.NewHistogram(prometheus.HistogramOpts{
			Name: h.metricName + "_duration_seconds",
		})
	}

	h.histograms[key].Observe(NanosecondsToSeconds(timeInNanoseconds))
	h.histogramsInInterval[key] = struct{}{}
}

func (h *histogramRollup) Rollup(timestamp int64) []*PointsBatch {
	var batches []*PointsBatch

	h.mu.Lock()
	defer h.mu.Unlock()

	for k, _ := range h.histogramsInInterval {
		var points []*rpc.Point
		var size int

		labels, err := labelsFromKey(k, h.nodeIndex, h.rollupTags, h.log)
		if err != nil {
			continue
		}

		th := h.histograms[k]
		m := &dto.Metric{}
		th.Write(m)
		histogram := m.GetHistogram()

		for _, bucket := range histogram.Bucket {
			bucketLabels := make(map[string]string)
			for key, value := range labels {
				bucketLabels[key] = value
			}
			bucketLabels["le"] = strconv.FormatFloat(*bucket.UpperBound, 'f', 3, 64)

			bucketPoint := &rpc.Point{
				Name:      h.metricName + "_duration_seconds_bucket",
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
			Name:      h.metricName + "_duration_seconds_bucket",
			Timestamp: timestamp,
			Value:     float64(*histogram.SampleCount),
			Labels:    bucketLabels,
		}

		histogramCountPoint := &rpc.Point{
			Name:      h.metricName + "_duration_seconds_count",
			Timestamp: timestamp,
			Value:     float64(*histogram.SampleCount),
			Labels:    labels,
		}
		histogramSumPoint := &rpc.Point{
			Name:      h.metricName + "_duration_seconds_sum",
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

	h.histogramsInInterval = make(map[string]struct{})

	return batches
}

func NanosecondsToSeconds(ns int64) float64 {
	return float64(ns*int64(time.Nanosecond)) / float64(time.Second)
}
