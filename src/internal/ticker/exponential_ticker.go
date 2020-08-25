package ticker

import (
	"context"
	"math"
	"time"
)

type Config struct {
	BaseDelay  time.Duration
	Multiplier float64
	MaxDelay   time.Duration
	Context    context.Context
}

func New(cfg Config) (chan struct{}, func()) {
	var cancel func()
	cfg = validateConfig(cfg)
	cfg.Context, cancel = context.WithCancel(cfg.Context)

	ticker := make(chan struct{})
	go tick(ticker, cfg)

	return ticker, cancel
}

//TODO Config.validate?
func validateConfig(cfg Config) Config {
	if cfg.BaseDelay < 0 {
		panic("BaseDelay must be non-negative")
	}
	if cfg.BaseDelay == 0 {
		cfg.BaseDelay = 100 * time.Millisecond
	}

	if cfg.Multiplier < 0 {
		panic("Multiplier must be non-negative")
	}
	if cfg.Multiplier == 0 {
		cfg.Multiplier = 2
	}

	if cfg.Context == nil {
		cfg.Context = context.Background()
	}

	if cfg.MaxDelay < 0 {
		panic("MaxDelay must be non-negative")
	}
	return cfg
}

func tick(ticker chan struct{}, cfg Config) {
	delay := cfg.BaseDelay
	for {
		time.Sleep(delay)
		
		select {
		case <-cfg.Context.Done():
			return
		default:
			ticker <- struct{}{}
			delay = calculateNextDelay(delay, cfg)
		}
	}
}

func calculateNextDelay(delay time.Duration, cfg Config) time.Duration {
	nextDelayInNanos := float64(delay.Nanoseconds()) * cfg.Multiplier
	if cfg.MaxDelay != 0 {
		nextDelayInNanos = math.Min(nextDelayInNanos, float64(cfg.MaxDelay))
	}
	return time.Duration(nextDelayInNanos)
}
