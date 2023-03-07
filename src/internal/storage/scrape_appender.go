package storage

import (
	"context"
	"github.com/cloudfoundry/metric-store-release/src/pkg/persistence/transform"
	"github.com/prometheus/prometheus/model/exemplar"
	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/metadata"
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

func (s *ScrapeStorage) Appender(ctx context.Context) prom_storage.Appender {
	appender := s.store.Appender(ctx)

	return &ScrapeAppender{
		appender: appender,
	}
}

type ScrapeAppender struct {
	appender prom_storage.Appender
}

func (s *ScrapeAppender) Append(ref prom_storage.SeriesRef, l labels.Labels, t int64, v float64) (prom_storage.SeriesRef, error) {
	timeInNanoseconds := transform.MillisecondsToNanoseconds(t)

	return s.appender.Append(ref, l, timeInNanoseconds, v)
}

func (s *ScrapeAppender) AppendExemplar(ref prom_storage.SeriesRef, l labels.Labels, e exemplar.Exemplar) (prom_storage.SeriesRef, error) {
	// not implemented
	return ref, nil
}

func (s *ScrapeAppender) AppendHistogram(ref prom_storage.SeriesRef, l labels.Labels, t int64, h *histogram.Histogram, fh *histogram.FloatHistogram) (prom_storage.SeriesRef, error) {
	// not implemented
	return ref, nil
}

func (s *ScrapeAppender) UpdateMetadata(ref prom_storage.SeriesRef, l labels.Labels, m metadata.Metadata) (prom_storage.SeriesRef, error) {
	// not implemented
	return ref, nil
}

func (s *ScrapeAppender) Commit() error {
	return s.appender.Commit()
}

func (s *ScrapeAppender) Rollback() error {
	return s.appender.Rollback()
}
