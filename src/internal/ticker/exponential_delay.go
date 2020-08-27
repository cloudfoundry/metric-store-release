package ticker

import (
	"math"
	"sync"
	"time"
)

type ExponentialDelay struct {
	duration  time.Duration

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
		baseDelay:  cfg.BaseDelay,
		maxDelay:   cfg.MaxDelay,
		multiplier: cfg.Multiplier,
	}
}

func (d *ExponentialDelay) Reset() {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.duration = d.baseDelay
}

func (d *ExponentialDelay) Step() time.Duration {
	d.mu.Lock()
	defer d.mu.Unlock()

	duration := d.duration
	d.step()
	return duration
}

func (d *ExponentialDelay) step() {
	nextDelayInNanos := float64(d.duration.Nanoseconds()) * d.multiplier
	if d.maxDelay != 0 {
		nextDelayInNanos = math.Min(nextDelayInNanos, float64(d.maxDelay))
	}

	d.duration = time.Duration(nextDelayInNanos)
}
