package main

import (
	"time"

	envstruct "code.cloudfoundry.org/go-envstruct"
	"github.com/cloudfoundry/metric-store/pkg/tls"
)

// Config is the configuration for a MetricStore.
type Config struct {
	Addr                  string        `env:"ADDR, required, report"`
	HealthAddr            string        `env:"HEALTH_ADDR, report"`
	StoragePath           string        `env:"STORAGE_PATH, report"`
	RetentionPeriodInDays uint          `env:"RETENTION_PERIOD_IN_DAYS, report"`
	RetentionPeriod       time.Duration `env:"-, report"`
	DiskFreePercentTarget uint          `env:"DISK_FREE_PERCENT_TARGET, report"`
	LabelTruncationLength uint          `env:"LABEL_TRUNCATION_LENGTH, report"`
	QueryTimeout          time.Duration `env:"QUERY_TIMEOUT, report"`
	TLS                   tls.TLS
}

// LoadConfig creates Config object from environment variables
func LoadConfig() (*Config, error) {
	c := Config{
		Addr:                  ":8080",
		HealthAddr:            "localhost:6060",
		StoragePath:           "/tmp/metric-store",
		RetentionPeriod:       7 * 24 * time.Hour,
		DiskFreePercentTarget: 20,
		LabelTruncationLength: 256,
		QueryTimeout:          10 * time.Second,
	}

	if err := envstruct.Load(&c); err != nil {
		return nil, err
	}

	if c.RetentionPeriodInDays > 0 {
		c.RetentionPeriod = time.Duration(c.RetentionPeriodInDays) * 24 * time.Hour
	}

	return &c, nil
}
