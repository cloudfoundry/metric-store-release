package app

import (
	"log"

	"code.cloudfoundry.org/go-envstruct"
)

type Config struct {
	LogProviderAddr string `env:"LOGS_PROVIDER_ADDR, required, report"`
	LogsProviderTLS LogsProviderTLS

	OtelAddr        string `env:"OTEL_ADDR, required, report"`
	OtelProviderTLS OtelProviderTLS

	MetricStoreTLS        MetricStoreClientTLS
	MetricStoreMetricsTLS MetricStoreMetricsTLS

	IngressAddr           string   `env:"INGRESS_ADDR, required, report"`
	MetricsAddr           string   `env:"METRICS_ADDR, report"`
	ShardId               string   `env:"SHARD_ID, required, report"`
	TimerRollupBufferSize uint     `env:"TIMER_ROLLUP_BUFFER_SIZE, report"`
	NodeIndex             int      `env:"NODE_INDEX, required, report"`
	FilterMetrics         bool     `env:"ENABLE_ENVELOPE_SELECTOR, required, report"`
	EnvelopSelectorTags   []string `env:"ENVELOP_SELECTOR_TAGS, report"`
	LogLevel              string   `env:"LOG_LEVEL, report"`
	ProfilingAddr         string   `env:"PROFILING_ADDR, report"`
	FirehoseEnabled       bool     `env:"FIREHOSE_ENABLED, required, report"`
}

type MetricStoreClientTLS struct {
	CAPath   string `env:"METRIC_STORE_CLIENT_CA_PATH, required, report"`
	CertPath string `env:"METRIC_STORE_CLIENT_CERT_PATH, required, report"`
	KeyPath  string `env:"METRIC_STORE_CLIENT_KEY_PATH, required, report"`
}

type LogsProviderTLS struct {
	LogProviderCA   string `env:"LOGS_PROVIDER_CA_PATH, required, report"`
	LogProviderCert string `env:"LOGS_PROVIDER_CERT_PATH, required, report"`
	LogProviderKey  string `env:"LOGS_PROVIDER_KEY_PATH, required, report"`
}

type OtelProviderTLS struct {
	CAPath   string `env:"OTEL_PROVIDER_CA_PATH, required, report"`
	CertPath string `env:"OTEL_PROVIDER_CERT_PATH, required, report"`
	KeyPath  string `env:"OTEL_PROVIDER_KEY_PATH, required, report"`
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
		IngressAddr:           ":8090",
		MetricsAddr:           ":6061",
		ProfilingAddr:         "localhost:6071",
		ShardId:               "metric-store",
		TimerRollupBufferSize: 16384,
		FilterMetrics:         false,
		FirehoseEnabled:       true,
	}

	if err := envstruct.Load(cfg); err != nil {
		log.Fatalf("failed to load config from environment: %s", err)
	}

	_ = envstruct.WriteReport(cfg)

	return cfg
}
