package testing

import (
	"time"

	"go.uber.org/atomic"
)

type SpyQueue struct {
	PurgeCalls atomic.Int64
}

func NewSpyQueue(dir string) *SpyQueue {
	return &SpyQueue{}
}

func (spy *SpyQueue) Advance() error {
	panic("not implemented") // TODO: Implement
}

func (spy *SpyQueue) Append(_ []byte) error {
	return nil
}

func (spy *SpyQueue) Close() error {
	panic("not implemented") // TODO: Implement
}

func (spy *SpyQueue) Current() ([]byte, error) {
	panic("not implemented") // TODO: Implement
}

func (spy *SpyQueue) DiskUsage() int64 {
	return 0
}

func (spy *SpyQueue) Open() error {
	return nil
}

func (spy *SpyQueue) PurgeOlderThan(_ time.Time) error {
	spy.PurgeCalls.Inc()
	return nil
}

func (spy *SpyQueue) SetMaxSegmentSize(size int64) error {
	panic("not implemented") // TODO: Implement
}
