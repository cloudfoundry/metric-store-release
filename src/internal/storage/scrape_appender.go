package storage

import (
	"github.com/cloudfoundry/metric-store-release/src/pkg/persistence/transform"
	"github.com/prometheus/prometheus/pkg/labels"
	"github.com/prometheus/prometheus/scrape"
	prom_storage "github.com/prometheus/prometheus/storage"
)

type ScrapeStorage struct {
	store prom_storage.Storage
}

func NewScrapeStorage(store prom_storage.Storage) scrape.Appendable {
	return &ScrapeStorage{
		store: store,
	}
}

func (s *ScrapeStorage) Appender() (prom_storage.Appender, error) {
	appender, err := s.store.Appender()
	if err != nil {
		return nil, err
	}

	return &ScrapeAppender{
		appender: appender,
	}, nil
}

type ScrapeAppender struct {
	appender prom_storage.Appender
}

func (s *ScrapeAppender) Add(l labels.Labels, t int64, v float64) (uint64, error) {
	timeInNanoseconds := transform.MillisecondsToNanoseconds(t)

	return s.appender.Add(l, timeInNanoseconds, v)
}

func (s *ScrapeAppender) AddFast(l labels.Labels, ref uint64, t int64, v float64) error {
	timeInNanoseconds := transform.MillisecondsToNanoseconds(t)

	return s.appender.AddFast(l, ref, timeInNanoseconds, v)
}

func (s *ScrapeAppender) Commit() error {
	return s.appender.Commit()
}

func (s *ScrapeAppender) Rollback() error {
	return s.appender.Rollback()
}
