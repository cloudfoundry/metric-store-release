package testing

import (
	"context"

	"github.com/prometheus/prometheus/storage"
)

type SpyStorage struct {
	querier storage.Querier
}

func (s *SpyStorage) Querier(ctx context.Context, mint int64, maxt int64) (storage.Querier, error) {
	return s.querier, nil
}

func (s *SpyStorage) StartTime() (int64, error) {
	panic("not implemented")
}

func (s *SpyStorage) Appender() (storage.Appender, error) {
	panic("not implemented")
}

func (s *SpyStorage) Close() error {
	panic("not implemented")
}

func NewSpyStorage(querier storage.Querier) *SpyStorage {
	return &SpyStorage{
		querier: querier,
	}
}
