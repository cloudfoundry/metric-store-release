package storage

import (
	prom_storage "github.com/prometheus/prometheus/storage"
	"math"
	"time"
)

func DefaultTimeRangeAndMatches() (time.Time, time.Time, []string) {
	minTime := time.Unix(math.MinInt64/1000+62135596801, 0).UTC()
	maxTime := time.Unix(math.MaxInt64/1000-62135596801, 999999999).UTC()

	return minTime, maxTime, nil
}

// GlobalSeriesSet implements storage.SeriesSet.
type GlobalSeriesSet struct {
	err error
}

func (c *GlobalSeriesSet) Next() bool {
	return false
}

func (c *GlobalSeriesSet) At() prom_storage.Series {
	return nil
}

func (c *GlobalSeriesSet) Err() error {
	return c.err
}

func (c *GlobalSeriesSet) Warnings() prom_storage.Warnings {
	return nil
}

func NewGlobalSeriesSet(err error) *GlobalSeriesSet {
	return &GlobalSeriesSet{
		err: err,
	}
}
