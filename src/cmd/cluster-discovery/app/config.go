package app

import (
	"log"

	"code.cloudfoundry.org/go-envstruct"
)

// Config is the configuration for a ClusterDiscovery.
type Config struct {
	MetricsAddr    string `env:"METRICS_ADDR, report"`
	StoragePath    string `env:"STORAGE_DIR, required, report"`
	LogLevel       string `env:"LOG_LEVEL, report"`
	MetricsTLS     ClusterDiscoveryMetricsTLS
	MetricStoreAPI MetricStoreAPI
	PKS            PKSConfig
	UAA            UAAConfig
}

// LoadConfig creates Config object from environment variables
func LoadConfig() *Config {
	cfg := &Config{
		LogLevel:    "info",
		MetricsAddr: ":6060",
	}

	if err := envstruct.Load(cfg); err != nil {
		log.Fatalf("failed to load config from environment: %s", err)
	}

	_ = envstruct.WriteReport(cfg)

	return cfg
}

type ClusterDiscoveryMetricsTLS struct {
	CAPath   string `env:"METRICS_CA_PATH, required, report"`
	CertPath string `env:"METRICS_CERT_PATH, required, report"`
	KeyPath  string `env:"METRICS_KEY_PATH, required, report"`
}

type MetricStoreAPI struct {
	Address    string `env:"METRIC_STORE_API_ADDRESS, required, report"`
	CAPath     string `env:"METRIC_STORE_API_CA_PATH, required, report"`
	CertPath   string `env:"METRIC_STORE_API_CERT_PATH, required, report"`
	KeyPath    string `env:"METRIC_STORE_API_KEY_PATH, required, report"`
	CommonName string `env:"METRIC_STORE_API_COMMON_NAME, required, report"`
}

type PKSConfig struct {
	API                string `env:"PKS_API_ADDR, required, report"` // TODO actually a hostname, rename env var
	CAPath             string `env:"PKS_CA_PATH, required, report"`
	CommonName         string `env:"PKS_SERVER_NAME, required, report"`
	InsecureSkipVerify bool   `env:"PKS_SKIP_SSL_VALIDATION, required, report"`
}

type UAAConfig struct {
	Addr         string `env:"PKS_UAA_ADDR, required, report"` // TODO actually a hostname, rename env var
	CAPath       string `env:"PKS_UAA_CA_PATH, required, report"`
	Client       string `env:"PKS_UAA_CLIENT, required, report"`
	ClientSecret string `env:"PKS_UAA_CLIENT_SECRET, required"`
}
