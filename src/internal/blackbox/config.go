package blackbox

import (
	"log"
	"time"

	envstruct "code.cloudfoundry.org/go-envstruct"
	sharedtls "github.com/cloudfoundry/metric-store-release/src/internal/tls"
)

type Config struct {
	EmissionInterval time.Duration `env:"EMISSION_INTERVAL, required, report"`
	SampleInterval   time.Duration `env:"SAMPLE_INTERVAL, required, report"`
	WindowInterval   time.Duration `env:"WINDOW_INTERVAL, required, report"`
	WindowLag        time.Duration `env:"WINDOW_LAG, required, report"`
	SourceId         string        `env:"SOURCE_ID, required, report"`

	MetricsAddr            string `env:"METRICS_ADDR, report"`
	CfBlackboxEnabled      bool   `env:"CF_BLACKBOX_ENABLED, report"`
	MetricStoreHTTPAddr    string `env:"METRIC_STORE_HTTP_ADDR, required, report"`
	MetricStoreIngressAddr string `env:"METRIC_STORE_INGRESS_ADDR, required, report"`
	UaaAddr                string `env:"UAA_ADDR, report"`
	ClientID               string `env:"CLIENT_ID, report"`
	ClientSecret           string `env:"CLIENT_SECRET"`
	SkipTLSVerify          bool   `env:"SKIP_TLS_VERIFY, report"`

	MetricStoreGrpcAddr   string `env:"METRIC_STORE_GRPC_ADDR, required, report"`
	TLS                   sharedtls.TLS
	MetricStoreMetricsTLS MetricStoreMetricsTLS

	LogLevel string `env:"LOG_LEVEL,                      report"`
}

type MetricStoreMetricsTLS struct {
	CAPath   string `env:"METRIC_STORE_METRICS_CA_PATH, required, report"`
	CertPath string `env:"METRIC_STORE_METRICS_CERT_PATH, required, report"`
	KeyPath  string `env:"METRIC_STORE_METRICS_KEY_PATH, required, report"`
}

func LoadConfig() *Config {
	cfg := &Config{
		LogLevel:    "info",
		MetricsAddr: ":6066",
	}

	if err := envstruct.Load(cfg); err != nil {
		log.Fatalf("failed to load config from environment: %s", err)
	}

	_ = envstruct.WriteReport(cfg)

	return cfg
}
