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
	InternodeAddr         string        `env:"INTERNODE_ADDR, required, report"`
	HealthPort            int           `env:"HEALTH_PORT, report"`
	StoragePath           string        `env:"STORAGE_PATH, report"`
	RetentionPeriodInDays uint          `env:"RETENTION_PERIOD_IN_DAYS, report"`
	RetentionPeriod       time.Duration `env:"-, report"`
	DiskFreePercentTarget uint          `env:"DISK_FREE_PERCENT_TARGET, report"`
	ReplicationFactor     uint          `env:"REPLICATION_FACTOR, report"`
	LabelTruncationLength uint          `env:"LABEL_TRUNCATION_LENGTH, report"`
	QueryTimeout          time.Duration `env:"QUERY_TIMEOUT, report"`

	// NodeIndex determines what data the node stores. It splits up the range
	// of 0 - 18446744073709551615 evenly. If data falls out of range of the
	// given node, it will be routed to theh correct one.
	NodeIndex int `env:"NODE_INDEX, report"`

	// NodeAddrs are all the MetricStore addresses (including the current
	// address). They are in order according to their NodeIndex.
	//
	// If NodeAddrs is emptpy or size 1, then data is not routed as it is
	// assumed that the current node is the only one.
	NodeAddrs []string `env:"NODE_ADDRS, report"`

	// InternodeAddrs are all the internode addresses (including the current
	// address). They are in order according to their NodeIndex.
	//
	// If InternodeAddrs is emptpy or size 1, then data is not routed as it is
	// assumed that the current node is the only one.
	InternodeAddrs []string `env:"INTERNODE_ADDRS, report"`

	TLS                     sharedtls.TLS
	MetricStoreServerTLS    MetricStoreServerTLS
	MetricStoreInternodeTLS MetricStoreInternodeTLS

	RulesPath        string `env:"RULES_PATH, report"`
	AlertmanagerAddr string `env:"ALERTMANAGER_ADDR, report"`

	LogLevel string `env:"LOG_LEVEL,                      report"`
}

type MetricStoreServerTLS struct {
	CAPath   string `env:"METRIC_STORE_SERVER_CA_PATH, required, report"`
	CertPath string `env:"METRIC_STORE_SERVER_CERT_PATH, required, report"`
	KeyPath  string `env:"METRIC_STORE_SERVER_KEY_PATH, required, report"`
}

type MetricStoreInternodeTLS struct {
	CAPath   string `env:"METRIC_STORE_INTERNODE_CA_PATH, required, report"`
	CertPath string `env:"METRIC_STORE_INTERNODE_CERT_PATH, required, report"`
	KeyPath  string `env:"METRIC_STORE_INTERNODE_KEY_PATH, required, report"`
}

// LoadConfig creates Config object from environment variables
func LoadConfig() *Config {
	cfg := &Config{
		LogLevel:              "info",
		Addr:                  ":8080",
		IngressAddr:           ":8090",
		InternodeAddr:         ":8091",
		HealthPort:            6060,
		StoragePath:           "/tmp/metric-store",
		RetentionPeriod:       7 * 24 * time.Hour,
		DiskFreePercentTarget: 20,
		ReplicationFactor:     1,
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
