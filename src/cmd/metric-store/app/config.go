package app

import (
	"log"
	"time"

	envstruct "code.cloudfoundry.org/go-envstruct"
	sharedtls "github.com/cloudfoundry/metric-store-release/src/pkg/tls"
)

// Config is the configuration for a MetricStore.
type Config struct {
	Addr                  string        `env:"ADDR, required, report"`
	IngressAddr           string        `env:"INGRESS_ADDR, required, report"`
	HealthPort            int           `env:"HEALTH_PORT, report"`
	StoragePath           string        `env:"STORAGE_PATH, report"`
	RetentionPeriodInDays uint          `env:"RETENTION_PERIOD_IN_DAYS, report"`
	RetentionPeriod       time.Duration `env:"-, report"`
	DiskFreePercentTarget uint          `env:"DISK_FREE_PERCENT_TARGET, report"`
	LabelTruncationLength uint          `env:"LABEL_TRUNCATION_LENGTH, report"`
	QueryTimeout          time.Duration `env:"QUERY_TIMEOUT, report"`
	TLS                   sharedtls.TLS
	MetricStoreServerTLS  MetricStoreServerTLS

	RulesPath        string `env:"RULES_PATH, report"`
	AlertmanagerAddr string `env:"ALERTMANAGER_ADDR, report"`

	LogLevel string `env:"LOG_LEVEL,                      report"`
}

type MetricStoreServerTLS struct {
	CAPath   string `env:"METRIC_STORE_SERVER_CA_PATH, required, report"`
	CertPath string `env:"METRIC_STORE_SERVER_CERT_PATH, required, report"`
	KeyPath  string `env:"METRIC_STORE_SERVER_KEY_PATH, required, report"`
}

// LoadConfig creates Config object from environment variables
func LoadConfig() *Config {
	cfg := &Config{
		LogLevel:              "info",
		Addr:                  ":8080",
		IngressAddr:           ":8090",
		HealthPort:            6060,
		StoragePath:           "/tmp/metric-store",
		RetentionPeriod:       7 * 24 * time.Hour,
		DiskFreePercentTarget: 20,
		LabelTruncationLength: 256,
		QueryTimeout:          10 * time.Second,
	}

	if err := envstruct.Load(cfg); err != nil {
		log.Fatalf("failed to load config from environment: %s", err)
	}

	if cfg.RetentionPeriodInDays > 0 {
		cfg.RetentionPeriod = time.Duration(cfg.RetentionPeriodInDays) * 24 * time.Hour
	}

	_ = envstruct.WriteReport(cfg)

	return cfg
}
