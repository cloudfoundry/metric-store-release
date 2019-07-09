package main

import (
	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"time"

	envstruct "code.cloudfoundry.org/go-envstruct"
	"github.com/cloudfoundry/metric-store-release/src/pkg/metrics"
	"github.com/cloudfoundry/metric-store-release/src/pkg/metricstore"
	"github.com/cloudfoundry/metric-store-release/src/pkg/persistence"
	"github.com/cloudfoundry/metric-store-release/src/pkg/system_stats"
	sharedtls "github.com/cloudfoundry/metric-store-release/src/pkg/tls"
	"google.golang.org/grpc"
)

func main() {
	metricStoreLog := log.New(os.Stderr, "", log.LstdFlags|log.Lmicroseconds)

	metricStoreLog.Print("Starting Metric Store...")
	defer metricStoreLog.Print("Closing Metric Store.")

	cfg, err := LoadConfig()
	if err != nil {
		metricStoreLog.Fatalf("invalid configuration: %s", err)
	}

	envstruct.WriteReport(cfg)

	tlsServerConfig, err := sharedtls.NewMutualTLSConfig(
		cfg.MetricStoreServerTLS.CAPath,
		cfg.MetricStoreServerTLS.CertPath,
		cfg.MetricStoreServerTLS.KeyPath,
		"metric-store",
	)
	if err != nil {
		log.Fatalf("invalid metric-store TCP server Mutual TLS configuration: %s", err)
	}

	metrics := metrics.New("metric-store")

	persistentStore := persistence.NewStore(
		cfg.StoragePath,
		metrics,
		persistence.WithAppenderLabelTruncationLength(cfg.LabelTruncationLength),
		persistence.WithLogger(metricStoreLog),
	)
	go persistentStore.EmitStorageMetrics()
	diskFreeReporter := newDiskFreeReporter(cfg.StoragePath, metricStoreLog, metrics)

	store := metricstore.New(
		persistentStore,
		diskFreeReporter,
		tlsServerConfig,
		metricstore.WithMetrics(metrics),
		metricstore.WithAddr(cfg.Addr),
		metricstore.WithIngressAddr(cfg.IngressAddr),
		metricstore.WithAlertmanagerAddr(cfg.AlertmanagerAddr),
		metricstore.WithRulesPath(cfg.RulesPath),
		metricstore.WithServerOpts(grpc.Creds(cfg.TLS.Credentials("metric-store"))),
		metricstore.WithRetentionConfig(metricstore.RetentionConfig{
			ExpiryFrequency:       15 * time.Minute,
			RetentionPeriod:       cfg.RetentionPeriod,
			DiskFreePercentTarget: float64(cfg.DiskFreePercentTarget),
		}),
		metricstore.WithLogger(metricStoreLog),
		metricstore.WithQueryTimeout(cfg.QueryTimeout),
	)

	store.Start()

	// Register prometheus-compatible metric endpoint
	http.Handle("/metrics", metrics)

	// Start listening on metrics/health endpoint and block forever
	http.ListenAndServe(fmt.Sprintf("localhost:%d", cfg.HealthPort), nil)
}

func newDiskFreeReporter(storagePath string, metricStoreLog *log.Logger, metrics *metrics.Metrics) func() float64 {
	diskFreeMetric := metrics.NewGauge("metric_store_disk_free_percent", "percent")
	diskFreeErrorsMetric := metrics.NewCounter("metric_store_disk_free_errors")

	return func() float64 {
		diskFree, err := system_stats.DiskFree(storagePath)

		if err != nil {
			metricStoreLog.Printf("failed to get disk free space of %s: %s\n", storagePath, err)
			diskFreeErrorsMetric(1)
			return 0
		}

		diskFreeMetric(diskFree)
		return diskFree
	}
}
