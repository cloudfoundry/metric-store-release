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
	cancel          func()
}

func New(delay Delay, opts ...Option) *VariableTicker {
	t := &VariableTicker{
		C:               make(chan struct{}),
		delay:           delay,
		originalContext: context.Background(), // TODO should not be an unbounded context
	}

	for _, opt := range opts {
		opt(t)
	}

	var ctx context.Context
	ctx, t.cancel = context.WithCancel(t.originalContext)
	go t.tick(ctx)

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

	var ctx context.Context
	ctx, t.cancel = context.WithCancel(t.originalContext)
	go t.tick(ctx)
}

func (t *VariableTicker) Stop() {
	t.cancel()
}

func (t *VariableTicker) tick(ctx context.Context) {
	for {
		time.Sleep(t.delay.Step())
		select {
		case <-ctx.Done():
			return
		default:
			t.C <- struct{}{}
		}
	}
}
