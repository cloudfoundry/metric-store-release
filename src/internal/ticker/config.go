package ticker

import (
	"time"
)

type Config struct {
	BaseDelay  time.Duration
	Multiplier float64
	MaxDelay   time.Duration
}

func (cfg *Config) Validate() {
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

	if cfg.MaxDelay < 0 {
		panic("MaxDelay must be non-negative")
	}
}
