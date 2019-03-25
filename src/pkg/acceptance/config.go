package acceptance

import (
	"log"

	envstruct "code.cloudfoundry.org/go-envstruct"
)

type TestConfig struct {
	MetricStoreAddr           string `env:"METRIC_STORE_ADDR,    required, report"`
	MetricStoreCFAuthProxyURL string `env:"METRIC_STORE_CF_AUTH_PROXY_URL,  required, report"`

	TLS TLS

	UAAURL       string `env:"UAA_URL, required, report"`
	ClientID     string `env:"CLIENT_ID, required, report"`
	ClientSecret string `env:"CLIENT_SECRET, required, noreport"`

	SkipCertVerify bool `env:"SKIP_CERT_VERIFY, report"`
}

var config *TestConfig

func LoadConfig() (*TestConfig, error) {
	config := &TestConfig{}

	err := envstruct.Load(config)
	if err != nil {
		return nil, err
	}

	return config, nil
}

func Config() *TestConfig {
	if config != nil {
		return config
	}

	cfg, err := LoadConfig()
	if err != nil {
		log.Fatalf("failed to load metric store acceptance test config: %s", err)
	}
	config = cfg
	return config
}
