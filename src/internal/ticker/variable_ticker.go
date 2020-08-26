package ticker

import (
	"context"
	"time"
)

type Delay interface {
	Reset(time.Time)
	Ready(time.Time) bool
}

type VariableTicker struct {
	C               chan struct{}
	delay           Delay
	originalContext context.Context
	ctx             context.Context
	cancel          func()

	StartTime time.Time
}

func New(delay Delay, opts ...Option) *VariableTicker {
	t := &VariableTicker{
		C:               make(chan struct{}),
		delay:           delay,
		originalContext: context.Background(),
		cancel:          func() {},
	}

	for _, opt := range opts {
		opt(t)
	}

	t.ctx, t.cancel = context.WithCancel(t.originalContext)
	go t.tick()

	return t
}

type Option func(*VariableTicker)

func WithContext(ctx context.Context) Option {
	return func(t *VariableTicker) {
		t.originalContext = ctx
	}
}

func (t *VariableTicker) Reset() {
	t.delay.Reset(time.Now())
}

func (t *VariableTicker) Stop() {
	t.cancel()
}

func (t *VariableTicker) tick() {
	for {
		select {
		case <-t.ctx.Done():
			return
		default:
			if t.delay.Ready(time.Now()) {
				t.C <- struct{}{}
			} else {
				time.Sleep(time.Millisecond)
			}
		}
	}
}
