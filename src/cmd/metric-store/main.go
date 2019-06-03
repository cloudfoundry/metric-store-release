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
	"github.com/cloudfoundry/metric-store-release/src/pkg/tls"
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

	tlsServerConfig, err := tls.NewMutualTLSConfig(
		cfg.MetricStoreServerTLS.CAPath,
		cfg.MetricStoreServerTLS.CertPath,
		cfg.MetricStoreServerTLS.KeyPath,
		"metric-store",
	)
	if err != nil {
		log.Fatalf("invalid metric-store TCP server Mutual TLS configuration: %s", err)
	}

	tsStore, err := persistence.OpenTsStore(cfg.StoragePath)
	if err != nil {
		metricStoreLog.Fatalf("failed to open store: %v", err)
	}

	metrics := metrics.New()
	indexSizeGauge := metrics.NewGauge("metric_store_index_size", "byte")
	numberOfSeriesGauge := metrics.NewGauge("metric_store_num_series", "series")
	numberOfMeasurementsGauge := metrics.NewGauge("metric_store_num_measurements", "measurement")

	go func() {
		for {
			time.Sleep(time.Minute)

			statistics := tsStore.Statistics(map[string]string{})
			indexSizeGauge(float64(tsStore.IndexBytes()))

			for _, statistic := range statistics {
				if statistic.Values["numSeries"] != nil {
					numberOfSeriesGauge(float64(statistic.Values["numSeries"].(int64)))
				}

				if statistic.Values["numMeasurements"] != nil {
					numberOfMeasurementsGauge(float64(statistic.Values["numMeasurements"].(int64)))
				}
			}
		}
	}()

	persistentStore := persistence.NewStore(
		tsStore,
		metrics,
		persistence.WithLabelTruncationLength(cfg.LabelTruncationLength),
	)
	diskFreeReporter := newDiskFreeReporter(cfg.StoragePath, metricStoreLog, metrics)

	store := metricstore.New(
		persistentStore,
		diskFreeReporter,
		tlsServerConfig,
		metricstore.WithMetrics(metrics),
		metricstore.WithAddr(cfg.Addr),
		metricstore.WithIngressAddr(cfg.IngressAddr),
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
