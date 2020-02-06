package app

import (
	"log"

	envstruct "code.cloudfoundry.org/go-envstruct"
)

// Config is the configuration for a MetricStore.
type Config struct {
	LogProviderAddr string `env:"LOGS_PROVIDER_ADDR, required, report"`
	LogsProviderTLS LogsProviderTLS

	MetricStoreAddr       string `env:"METRIC_STORE_ADDR, required, report"`
	MetricStoreTLS        MetricStoreClientTLS
	MetricStoreMetricsTLS MetricStoreMetricsTLS

	IngressAddr           string `env:"INGRESS_ADDR, required, report"`
	HealthPort            int    `env:"HEALTH_PORT, report"`
	ShardId               string `env:"SHARD_ID, required, report"`
	TimerRollupBufferSize uint   `env:"TIMER_ROLLUP_BUFFER_SIZE, report"`
	NodeIndex             int    `env:"NODE_INDEX, report"`

	LogLevel string `env:"LOG_LEVEL,                      report"`
}

type MetricStoreClientTLS struct {
	CAPath   string `env:"METRIC_STORE_CLIENT_CA_PATH, required, report"`
	CertPath string `env:"METRIC_STORE_CLIENT_CERT_PATH, required, report"`
	KeyPath  string `env:"METRIC_STORE_CLIENT_KEY_PATH, required, report"`
}

// LogsProviderTLS is the LogsProviderTLS configuration for a MetricStore.
type LogsProviderTLS struct {
	LogProviderCA   string `env:"LOGS_PROVIDER_CA_PATH, required, report"`
	LogProviderCert string `env:"LOGS_PROVIDER_CERT_PATH, required, report"`
	LogProviderKey  string `env:"LOGS_PROVIDER_KEY_PATH, required, report"`
}

type MetricStoreMetricsTLS struct {
	CAPath   string `env:"METRIC_STORE_METRICS_CA_PATH, required, report"`
	CertPath string `env:"METRIC_STORE_METRICS_CERT_PATH, required, report"`
	KeyPath  string `env:"METRIC_STORE_METRICS_KEY_PATH, required, report"`
}

// LoadConfig creates Config object from environment variables
func LoadConfig() *Config {
	cfg := &Config{
		LogLevel:              "info",
		MetricStoreAddr:       ":8080",
		IngressAddr:           ":8090",
		HealthPort:            6061,
		ShardId:               "metric-store",
		TimerRollupBufferSize: 16384,
	}

	if err := envstruct.Load(cfg); err != nil {
		log.Fatalf("failed to load config from environment: %s", err)
	}

	_ = envstruct.WriteReport(cfg)

	return cfg
}
