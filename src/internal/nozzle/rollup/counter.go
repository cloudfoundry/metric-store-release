package rollup

import (
	"sync"
	"time"

	"github.com/cloudfoundry/metric-store-release/src/pkg/logger"
	"github.com/cloudfoundry/metric-store-release/src/pkg/rpc"
)

type counterRollup struct {
	log *logger.Logger

	nodeIndex  string
	rollupTags []string
	counters   map[string]int64
	lastSeen   map[string]int64
	expiration time.Duration

	mu sync.Mutex
}

func NewCounterRollup(log *logger.Logger, nodeIndex string, rollupTags []string, opts ...CounterRollupOption) *counterRollup {
	counterRollup := &counterRollup{
		log:        log,
		nodeIndex:  nodeIndex,
		rollupTags: rollupTags,
		counters:   make(map[string]int64),
		lastSeen:   make(map[string]int64),
		expiration: EXPIRATION,
	}

	for _, o := range opts {
		o(counterRollup)
	}

	return counterRollup
}

type CounterRollupOption func(*counterRollup)

func WithCounterRollupExpiration(expiration time.Duration) CounterRollupOption {
	return func(rollup *counterRollup) {
		rollup.expiration = expiration
	}
}

func (r *counterRollup) Record(timestamp int64, sourceId string, tags map[string]string, value int64) {
	key := keyFromTags(r.rollupTags, sourceId, tags)

	r.mu.Lock()
	defer r.mu.Unlock()

	r.lastSeen[key] = timestamp
	r.counters[key] += value
}

func (r *counterRollup) Rollup(timestamp int64) []*PointsBatch {
	var batches []*PointsBatch

	r.mu.Lock()
	defer r.mu.Unlock()

	for k := range r.counters {
		if expired(r.lastSeen[k], timestamp, r.expiration) {
			delete(r.lastSeen, k)
			delete(r.counters, k)
			continue
		}

		labels, err := labelsFromKey(k, r.nodeIndex, r.rollupTags, r.log)
		if err != nil {
			continue
		}

		countPoint := &rpc.Point{
			Name:      GorouterHttpMetricName + "_total",
			Timestamp: timestamp,
			Value:     float64(r.counters[k]),
			Labels:    labels,
		}

		batches = append(batches, &PointsBatch{
			Points: []*rpc.Point{countPoint},
			Size:   countPoint.EstimatePointSize(),
		})
	}

	return batches
}
