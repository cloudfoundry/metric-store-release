package main

import (
	envstruct "code.cloudfoundry.org/go-envstruct"
	"github.com/cloudfoundry/metric-store-release/src/pkg/tls"
)

// Config is the configuration for a MetricStore Gateway.
type Config struct {
	Addr            string `env:"ADDR, required, report"`
	MetricStoreAddr string `env:"METRIC_STORE_ADDR, required, report"`
	HealthAddr      string `env:"HEALTH_ADDR, report"`
	ProxyCertPath   string `env:"PROXY_CERT_PATH, report"`
	ProxyKeyPath    string `env:"PROXY_KEY_PATH, report"`
	TLS             tls.TLS
}

// LoadConfig creates Config object from environment variables
func LoadConfig() (*Config, error) {
	c := Config{
		Addr:            "localhost:8081",
		HealthAddr:      "localhost:6063",
		MetricStoreAddr: "localhost:8080",
	}

	if err := envstruct.Load(&c); err != nil {
		return nil, err
	}

	return &c, nil
}
