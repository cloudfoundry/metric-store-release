package testing

import (
	"context"
	"github.com/prometheus/prometheus/model/exemplar"
	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/metadata"

	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/storage"
)

type SpyStorage struct {
	querier storage.Querier
}

func (s *SpyStorage) Querier(ctx context.Context, mint, maxt int64) (storage.Querier, error) {
	return s.querier, nil
}

func (s *SpyStorage) ChunkQuerier(ctx context.Context, mint, maxt int64) (storage.ChunkQuerier, error) {
	//TODO implement me
	panic("implement me")
}

func (s *SpyStorage) Appender(ctx context.Context) storage.Appender {
	return &SpyAppender{}
}

func (s *SpyStorage) StartTime() (int64, error) {
	//TODO implement me
	panic("implement me")
}

func (s *SpyStorage) Close() error {
	return nil
}

func NewSpyStorage(querier storage.Querier) *SpyStorage {
	return &SpyStorage{
		querier: querier,
	}
}

type SpyAppender struct {
}

func (s SpyAppender) Append(ref storage.SeriesRef, l labels.Labels, t int64, v float64) (storage.SeriesRef, error) {
	return 0, nil
}

func (s SpyAppender) AppendExemplar(ref storage.SeriesRef, l labels.Labels, e exemplar.Exemplar) (storage.SeriesRef, error) {
	panic("implement me")
}

func (s SpyAppender) AppendHistogram(ref storage.SeriesRef, l labels.Labels, t int64, h *histogram.Histogram, fh *histogram.FloatHistogram) (storage.SeriesRef, error) {
	panic("implement me")
}

func (s SpyAppender) UpdateMetadata(ref storage.SeriesRef, l labels.Labels, m metadata.Metadata) (storage.SeriesRef, error) {
	panic("implement me")
}

func (s SpyAppender) Commit() error {
	return nil
}

func (s SpyAppender) Rollback() error {
	panic("implement me")
}
