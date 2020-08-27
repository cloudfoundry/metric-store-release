package ticker

import (
	"context"
	"time"
)

type Delay interface {
	Reset()
	Step() time.Duration
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
	t.delay.Reset()
	t.cancel()

	t.ctx, t.cancel = context.WithCancel(t.originalContext)
	go t.tick()
}

func (t *VariableTicker) Stop() {
	t.cancel()
}

func (t *VariableTicker) tick() {
	for {
		time.Sleep(t.delay.Step())
		select {
		case <-t.ctx.Done():
			return
		default:
			t.C <- struct{}{}
		}
	}
}
