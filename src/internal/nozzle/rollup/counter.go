package rollup

import (
	"sync"

	"github.com/cloudfoundry/metric-store-release/src/pkg/logger"
	"github.com/cloudfoundry/metric-store-release/src/pkg/rpc"
)

type counterRollup struct {
	log *logger.Logger

	nodeIndex                  string
	rollupTags                 []string
	setOfCounterKeysInInterval map[string]struct{}
	counters                   map[string]int64

	mu sync.Mutex
}

func NewCounterRollup(log *logger.Logger, nodeIndex string, rollupTags []string) *counterRollup {
	return &counterRollup{
		log:                        log,
		nodeIndex:                  nodeIndex,
		rollupTags:                 rollupTags,
		setOfCounterKeysInInterval: make(map[string]struct{}),
		counters:                   make(map[string]int64),
	}
}

func (r *counterRollup) Record(sourceId string, tags map[string]string, value int64) {
	key := keyFromTags(r.rollupTags, sourceId, tags)
	r.log.Log("msg", "CounterRollup: Record with key", "key", key)

	r.mu.Lock()
	defer r.mu.Unlock()

	r.setOfCounterKeysInInterval[key] = struct{}{}
	r.counters[key] += value
}

func (r *counterRollup) Rollup(timestamp int64) []*PointsBatch {
	var batches []*PointsBatch

	r.mu.Lock()
	defer r.mu.Unlock()

	for k := range r.setOfCounterKeysInInterval {
		labels, err := labelsFromKey(k, r.nodeIndex, r.rollupTags, r.log)
		if err != nil {
			continue
		}
		r.log.Log("msg", "CounterRollup: Rollup with ts", "timestamp", timestamp, "key", k, "value", float64(r.counters[k]))

		countPoint := &rpc.Point{
			Name:      GorouterHttpMetricName + "_total",
			Timestamp: timestamp,
			Value:     float64(r.counters[k]),
			Labels:    labels,
		}
		r.log.Log("msg", "CounterRollup", "counter", countPoint)

		batches = append(batches, &PointsBatch{
			Points: []*rpc.Point{countPoint},
			Size:   countPoint.EstimatePointSize(),
		})
	}

	r.setOfCounterKeysInInterval = make(map[string]struct{})

	return batches
}
