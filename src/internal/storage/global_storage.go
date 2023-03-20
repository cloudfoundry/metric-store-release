package storage

import (
	prom_storage "github.com/prometheus/prometheus/storage"
)

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
