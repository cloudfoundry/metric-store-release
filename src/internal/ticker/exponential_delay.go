package ticker

import (
	"math"
	"sync"
	"time"
)

type ExponentialDelay struct {
	duration  time.Duration
	startTime time.Time

	baseDelay  time.Duration
	maxDelay   time.Duration
	multiplier float64

	mu sync.Mutex
}

func NewExponentialDelay(cfg *Config) *ExponentialDelay {
	cfg.Validate()

	initialDuration := cfg.BaseDelay
	return &ExponentialDelay{
		duration:   initialDuration,
		startTime:  time.Now(),
		baseDelay:  cfg.BaseDelay,
		maxDelay:   cfg.MaxDelay,
		multiplier: cfg.Multiplier,
	}
}

func (d *ExponentialDelay) Ready(now time.Time) bool {
	d.mu.Lock()
	defer d.mu.Unlock()

	next := d.startTime.Add(d.duration)
	ready := !now.Before(next)
	if ready {
		d.step(now)
	}

	return ready
}

func (d *ExponentialDelay) Reset(now time.Time) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.duration = d.baseDelay
	d.startTime = now
}

func (d *ExponentialDelay) step(now time.Time) {
	nextDelayInNanos := float64(d.duration.Nanoseconds()) * d.multiplier
	if d.maxDelay != 0 {
		nextDelayInNanos = math.Min(nextDelayInNanos, float64(d.maxDelay))
	}

	d.startTime = now
	d.duration = time.Duration(nextDelayInNanos)
}
