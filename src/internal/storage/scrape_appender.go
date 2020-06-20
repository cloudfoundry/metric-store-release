package storage

import (
	"github.com/cloudfoundry/metric-store-release/src/pkg/persistence/transform"
	"github.com/prometheus/prometheus/pkg/labels"
	prom_storage "github.com/prometheus/prometheus/storage"
)

type ScrapeStorage struct {
	store prom_storage.Storage
}

func NewScrapeStorage(store prom_storage.Storage) prom_storage.Appendable {
	return &ScrapeStorage{
		store: store,
	}
}

func (s *ScrapeStorage) Appender() prom_storage.Appender {
	appender := s.store.Appender()

	return &ScrapeAppender{
		appender: appender,
	}
}

type ScrapeAppender struct {
	appender prom_storage.Appender
}

func (s *ScrapeAppender) Add(l labels.Labels, t int64, v float64) (uint64, error) {
	timeInNanoseconds := transform.MillisecondsToNanoseconds(t)

	return s.appender.Add(l, timeInNanoseconds, v)
}

func (s *ScrapeAppender) AddFast(ref uint64, t int64, v float64) error {
	// not implemented
	return nil
}

func (s *ScrapeAppender) Commit() error {
	return s.appender.Commit()
}

func (s *ScrapeAppender) Rollback() error {
	return s.appender.Rollback()
}
