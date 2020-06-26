package app

import (
	"crypto/tls"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/config"

	"github.com/cloudfoundry/metric-store-release/src/internal/metric-store"
	"github.com/cloudfoundry/metric-store-release/src/internal/metrics"
	"github.com/cloudfoundry/metric-store-release/src/internal/routing"
	"github.com/cloudfoundry/metric-store-release/src/internal/scraping"
	sharedtls "github.com/cloudfoundry/metric-store-release/src/internal/tls"
	"github.com/cloudfoundry/metric-store-release/src/pkg/logger"
	"github.com/cloudfoundry/metric-store-release/src/pkg/persistence"
	"github.com/cloudfoundry/metric-store-release/src/pkg/system_stats"
)

type MetricStoreApp struct {
	cfg *Config
	log *logger.Logger

	profilingMutex    sync.Mutex
	profilingListener net.Listener

	metricsMutex  sync.Mutex
	metricsServer *metrics.Server
}

func NewMetricStoreApp(cfg *Config, log *logger.Logger) *MetricStoreApp {
	return &MetricStoreApp{
		cfg: cfg,
		log: log,
	}
}

// Run starts the Metric Store, this is a blocking method call.
func (app *MetricStoreApp) Run() {
	tlsMetricsConfig, err := sharedtls.NewMutualTLSServerConfig(
		app.cfg.MetricStoreMetricsTLS.CAPath,
		app.cfg.MetricStoreMetricsTLS.CertPath,
		app.cfg.MetricStoreMetricsTLS.KeyPath,
	)
	if err != nil {
		app.log.Fatal("unable to create metrics TLS config", err)
	}

	app.startMetricsServer(tlsMetricsConfig)

	tlsEgressConfig := &config.TLSConfig{
		CAFile:     app.cfg.TLS.CAPath,
		CertFile:   app.cfg.TLS.CertPath,
		KeyFile:    app.cfg.TLS.KeyPath,
		ServerName: metric_store.COMMON_NAME,
	}

	tlsIngressConfig, err := sharedtls.NewMutualTLSServerConfig(
		app.cfg.MetricStoreServerTLS.CAPath,
		app.cfg.MetricStoreServerTLS.CertPath,
		app.cfg.MetricStoreServerTLS.KeyPath,
	)
	if err != nil {
		app.log.Fatal("invalid mTLS configuration for ingress", err)
	}

	tlsInternodeServerConfig, err := sharedtls.NewMutualTLSServerConfig(
		app.cfg.MetricStoreInternodeTLS.CAPath,
		app.cfg.MetricStoreInternodeTLS.CertPath,
		app.cfg.MetricStoreInternodeTLS.KeyPath,
	)
	if err != nil {
		app.log.Fatal("invalid mTLS configuration for internode server", err)
	}

	tlsInternodeClientConfig, err := sharedtls.NewMutualTLSClientConfig(
		app.cfg.MetricStoreInternodeTLS.CAPath,
		app.cfg.MetricStoreInternodeTLS.CertPath,
		app.cfg.MetricStoreInternodeTLS.KeyPath,
		metric_store.COMMON_NAME,
	)
	if err != nil {
		app.log.Fatal("invalid mTLS configuration for internode client", err)
	}

	diskFreeReporter := system_stats.NewDiskFreeReporter(app.cfg.StoragePath, app.log, app.metricsServer.Registrar())
	persistentStore := persistence.NewStore(
		app.cfg.StoragePath,
		app.metricsServer.Registrar(),
		persistence.WithAppenderLabelTruncationLength(app.cfg.LabelTruncationLength),
		persistence.WithLogger(app.log),
		persistence.WithRetentionConfig(persistence.RetentionConfig{
			ExpiryFrequency:       15 * time.Minute,
			RetentionPeriod:       app.cfg.RetentionPeriod,
			DiskFreePercentTarget: float64(app.cfg.DiskFreePercentTarget),
		}),
		persistence.WithDiskFreeReporter(diskFreeReporter),
	)

	// TODO: doesn't apply logic inside of metric store to have default single NodeAddr
	routingTable, err := routing.NewRoutingTable(app.cfg.NodeIndex, app.cfg.NodeAddrs, app.cfg.ReplicationFactor)
	if err != nil {
		app.log.Fatal("creating routing table", err)
	}
	scraper := scraping.New(app.cfg.ScrapeConfigPath, app.cfg.AdditionalScrapeConfigDir, app.log, routingTable)

	store := metric_store.New(
		persistentStore,
		app.cfg.StoragePath,
		tlsIngressConfig,
		tlsInternodeServerConfig,
		tlsInternodeClientConfig,
		tlsEgressConfig,
		metric_store.WithMetrics(app.metricsServer.Registrar()),
		metric_store.WithAddr(app.cfg.Addr),
		metric_store.WithIngressAddr(app.cfg.IngressAddr),
		metric_store.WithInternodeAddr(app.cfg.InternodeAddr),
		metric_store.WithScraper(scraper),
		metric_store.WithClustered(
			app.cfg.NodeIndex,
			app.cfg.NodeAddrs,
			app.cfg.InternodeAddrs,
		),
		metric_store.WithReplicationFactor(app.cfg.ReplicationFactor),
		metric_store.WithHandoffStoragePath(filepath.Join(app.cfg.StoragePath, "handoff")),
		metric_store.WithLogger(app.log),
		metric_store.WithQueryTimeout(app.cfg.QueryTimeout),
		metric_store.WithActiveQueryLogging(filepath.Join(app.cfg.StoragePath, "queryengine")),
		metric_store.WithQueryLogging(app.cfg.LogQueries),
	)

	store.Start()

	sigs := make(chan os.Signal, 1)
	done := make(chan bool, 1)

	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigs
		app.log.Info("received signal", logger.String("signal", sig.String()))
		if err := store.Close(); err != nil {
			app.log.Error("closing metric store", err)
		}
		if err := persistentStore.Close(); err != nil {
			app.log.Error("closing persistent store", err)
		}
		app.Stop()
		close(done)
	}()

	app.startDebugServer()

	<-done
}

// Stop stops all the subprocesses for the application.
func (app *MetricStoreApp) Stop() {
	app.metricsMutex.Lock()
	if err := app.metricsServer.Close(); err != nil {
		app.log.Error("closing metrics server", err)
	}
	app.metricsMutex.Unlock()

	app.profilingMutex.Lock()
	if err := app.profilingListener.Close(); err != nil {
		app.log.Error("closing profiling server", err)
	}
	app.profilingMutex.Unlock()
}

func (app *MetricStoreApp) startMetricsServer(tlsConfig *tls.Config) {
	registrar := metrics.NewRegistrar(
		app.log,
		"metric-store",
		metrics.WithConstLabels(map[string]string{
			"source_id": "metric-store",
		}),
		metrics.WithCounter(metrics.MetricStoreIngressPointsTotal, prometheus.CounterOpts{
			Help: "Number of points ingressed by metric-store",
		}),
		metrics.WithCounter(metrics.MetricStoreWrittenPointsTotal, prometheus.CounterOpts{
			Help: "Number of points successfully written to storage engine",
		}),
		metrics.WithHistogram(metrics.MetricStoreWriteDurationSeconds, prometheus.HistogramOpts{
			Help:    "Time spent writing points to the storage engine",
			Buckets: []float64{.001, .01, .05, .1, .2, 1},
		}),
		metrics.WithGauge(metrics.MetricStoreDiskFreeRatio, prometheus.GaugeOpts{
			Help: "Percentage of free space on persistent disk",
		}),
		metrics.WithCounter(metrics.MetricStoreExpiredShardsTotal, prometheus.CounterOpts{
			Help: "Number of shards removed due to time-based expiration",
		}),
		metrics.WithCounter(metrics.MetricStorePrunedShardsTotal, prometheus.CounterOpts{
			Help: "Number of shards removed due to disk space threshold",
		}),
		metrics.WithGauge(metrics.MetricStoreStorageDays, prometheus.GaugeOpts{
			Help: "Days of data stored on disk",
		}),
		metrics.WithGauge(metrics.MetricStoreSeriesCount, prometheus.GaugeOpts{
			Help: "Number of unique series stored in the index",
		}),
		metrics.WithGauge(metrics.MetricStoreMeasurementsCount, prometheus.GaugeOpts{
			Help: "Number of unique measurements stored in the index",
		}),
		metrics.WithCounter(metrics.MetricStoreReadErrorsTotal, prometheus.CounterOpts{
			Help: "Number of errors encountered reading from the storage engine",
		}),
		metrics.WithHistogram(metrics.MetricStoreTagValuesQueryDurationSeconds, prometheus.HistogramOpts{
			Help:    "Time spent retrieving tag values from the storage engine",
			Buckets: []float64{.001, .01, .05, .1, .2, 1},
		}),
		metrics.WithHistogram(metrics.MetricStoreMeasurementNamesQueryDurationSeconds, prometheus.HistogramOpts{
			Help:    "Time spent retrieving measurement names from the storage engine",
			Buckets: []float64{.1, .5, 1, 4, 16},
		}),
		metrics.WithLabelledGauge(metrics.MetricStoreReplayerDiskUsageBytes, prometheus.GaugeOpts{
			Help: "Size of a replayer queue",
		}, []string{"targetNodeIndex"}),
		metrics.WithLabelledCounter(metrics.MetricStoreReplayerQueueErrorsTotal, prometheus.CounterOpts{
			Help: "Number of errors encountered writing to a replayer queue",
		}, []string{"targetNodeIndex"}),
		metrics.WithLabelledCounter(metrics.MetricStoreReplayerQueuedBytesTotal, prometheus.CounterOpts{
			Help: "Number of bytes written to a replayer queue",
		}, []string{"targetNodeIndex"}),
		metrics.WithLabelledCounter(metrics.MetricStoreReplayerReadErrorsTotal, prometheus.CounterOpts{
			Help: "Number of errors encountered reading from a replayer queue",
		}, []string{"targetNodeIndex"}),
		metrics.WithLabelledCounter(metrics.MetricStoreReplayerReplayErrorsTotal, prometheus.CounterOpts{
			Help: "Number of errors encountered replaying writes to a remote node",
		}, []string{"targetNodeIndex"}),
		metrics.WithLabelledCounter(metrics.MetricStoreReplayerReplayedBytesTotal, prometheus.CounterOpts{
			Help: "Number of bytes successfully replayed to a remote node",
		}, []string{"targetNodeIndex"}),
		metrics.WithLabelledCounter(metrics.MetricStoreDroppedPointsTotal, prometheus.CounterOpts{
			Help: "Number of points dropped while writing to a remote node",
		}, []string{"targetNodeIndex"}),
		metrics.WithLabelledCounter(metrics.MetricStoreDistributedPointsTotal, prometheus.CounterOpts{
			Help: "Number of points successfully distributed to a remote node",
		}, []string{"targetNodeIndex"}),
		metrics.WithLabelledGauge(metrics.MetricStoreDistributedRequestDurationSeconds, prometheus.GaugeOpts{
			Help: "Time spent distributing points to a remote node",
		}, []string{"targetNodeIndex"}),
		metrics.WithCounter(metrics.MetricStoreCollectedPointsTotal, prometheus.CounterOpts{
			Help: "Number of points collected by a metric-store instance from remote nodes",
		}),
	)

	app.metricsMutex.Lock()
	app.metricsServer = metrics.StartMetricsServer(
		app.cfg.MetricsAddr,
		tlsConfig,
		app.log,
		registrar,
	)
	app.metricsMutex.Unlock()
}

func (app *MetricStoreApp) ProfilingAddr() string {
	app.profilingMutex.Lock()
	defer app.profilingMutex.Unlock()

	if app.profilingListener == nil {
		return ""
	}
	return app.profilingListener.Addr().String()
}

func (app *MetricStoreApp) MetricsAddr() string {
	app.metricsMutex.Lock()
	defer app.metricsMutex.Unlock()

	if app.metricsServer == nil {
		return ""
	}
	return app.metricsServer.Addr()
}

func (app *MetricStoreApp) startDebugServer() {
	app.profilingMutex.Lock()
	app.profilingListener = metrics.StartProfilingServer(app.cfg.ProfilingAddr, app.log)
	app.profilingMutex.Unlock()
}
