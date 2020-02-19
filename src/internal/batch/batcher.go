package batch

import (
	"time"

	"code.cloudfoundry.org/go-diodes"
	"github.com/cloudfoundry/metric-store-release/src/pkg/rpc"
)

const (
	CHANNEL_SIZE = 512
)

type Batcher struct {
	flushInterval     time.Duration
	maxSizeInBytes    int
	input             *diodes.OneToOne
	output            chan []*rpc.Point
	writer            func([]*rpc.Point)
	droppedMetricFunc func(int)
	done              chan struct{}
}

func NewBatcher(
	flushInterval time.Duration,
	maxSizeInBytes int,
	input *diodes.OneToOne,
	writer func([]*rpc.Point),
	done chan struct{},
	droppedMetricFunc func(int),
) *Batcher {
	batcher := &Batcher{
		flushInterval:     flushInterval,
		maxSizeInBytes:    maxSizeInBytes,
		input:             input,
		output:            make(chan []*rpc.Point, CHANNEL_SIZE),
		writer:            writer,
		done:              done,
		droppedMetricFunc: droppedMetricFunc,
	}

	go batcher.pointWriter()

	return batcher
}

func (b *Batcher) pointWriter() {
	for {
		points := <-b.output
		b.writer(points)
	}
}

func (b *Batcher) Start() {
	go func() {
		var size int

		poller := diodes.NewPoller(b.input)
		points := make([]*rpc.Point, 0)
		t := time.NewTimer(b.flushInterval)

		for {
			data, found := poller.TryNext()

			if found {
				point := (*rpc.Point)(data)
				size += estimatePointSize(point)
				points = append(points, point)
			}

			select {
			case <-b.done:
				b.writeToChannelOrDiscard(points)
				return
			case <-t.C:
				if len(points) > 0 {
					points = b.writeToChannelOrDiscard(points)
				}
				t.Reset(b.flushInterval)
				size = 0
			default:
				if size >= b.maxSizeInBytes {
					points = b.writeToChannelOrDiscard(points)
					t.Reset(b.flushInterval)
					size = 0
				}

				// this sleep keeps us from hammering an empty channel, which
				// would otherwise cause us to peg the cpu when there's no work
				// to be done.
				if !found {
					// CONSIDER: runtime.Gosched()
					time.Sleep(time.Millisecond)
				}
			}
		}
	}()
}

func (b *Batcher) writeToChannelOrDiscard(points []*rpc.Point) []*rpc.Point {
	select {
	case b.output <- points:
		return make([]*rpc.Point, 0)
	default:
		// if we can't write into the channel, it must be full, so
		// we probably need to drop these envelopes on the floor
		b.droppedMetricFunc(len(points))
		return points[:0]
	}
}

func estimatePointSize(point *rpc.Point) (size int) {
	size += len(point.Name)

	// 8 bytes for timestamp (int64), 8 bytes for value (float64)
	size += 16

	// add the size of all label keys and values
	for k, v := range point.Labels {
		size += (len(k) + len(v))
	}

	return size
}
