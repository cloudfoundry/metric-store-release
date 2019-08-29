package app

import (
	"fmt"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/cloudfoundry/metric-store-release/src/pkg/debug"
	"github.com/cloudfoundry/metric-store-release/src/pkg/logger"
	"github.com/cloudfoundry/metric-store-release/src/pkg/metricstore"
	"github.com/cloudfoundry/metric-store-release/src/pkg/persistence"
	"github.com/cloudfoundry/metric-store-release/src/pkg/system_stats"
	sharedtls "github.com/cloudfoundry/metric-store-release/src/pkg/tls"
	"github.com/prometheus/client_golang/prometheus"
)

type MetricStoreApp struct {
	cfg *Config
	log *logger.Logger

	debugMu        sync.Mutex
	debugLis       net.Listener
	debugRegistrar *debug.Registrar
}

func NewMetricStoreApp(cfg *Config, log *logger.Logger) *MetricStoreApp {
	return &MetricStoreApp{
		cfg: cfg,
		log: log,
	}
}

// DebugAddr returns the address (host and port) that the debug server is bound
// to. If the debug server has not been started an empty string will be returned.
func (c *MetricStoreApp) DebugAddr() string {
	c.debugMu.Lock()
	defer c.debugMu.Unlock()

	if c.debugLis != nil {
		return c.debugLis.Addr().String()
	}

	return ""
}

// Run starts the CFAuthProxyApp, this is a blocking method call.
func (m *MetricStoreApp) Run() {
	m.startDebugServer()

	tlsEgressConfig, err := sharedtls.NewMutualTLSConfig(
		m.cfg.TLS.CAPath,
		m.cfg.TLS.CertPath,
		m.cfg.TLS.KeyPath,
		"metric-store",
	)
	if err != nil {
		m.log.Fatal("invalid mTLS configuration for egress", err)
	}

	tlsIngressConfig, err := sharedtls.NewMutualTLSConfig(
		m.cfg.MetricStoreServerTLS.CAPath,
		m.cfg.MetricStoreServerTLS.CertPath,
		m.cfg.MetricStoreServerTLS.KeyPath,
		"metric-store",
	)
	if err != nil {
		m.log.Fatal("invalid mTLS configuration for ingress", err)
	}

	diskFreeReporter := newDiskFreeReporter(m.cfg.StoragePath, m.log, m.debugRegistrar)
	persistentStore := persistence.NewStore(
		m.cfg.StoragePath,
		m.debugRegistrar,
		persistence.WithAppenderLabelTruncationLength(m.cfg.LabelTruncationLength),
		persistence.WithLogger(m.log),
		persistence.WithRetentionConfig(persistence.RetentionConfig{
			ExpiryFrequency:       15 * time.Minute,
			RetentionPeriod:       m.cfg.RetentionPeriod,
			DiskFreePercentTarget: float64(m.cfg.DiskFreePercentTarget),
		}),
		persistence.WithDiskFreeReporter(diskFreeReporter),
	)

	store := metricstore.New(
		persistentStore,
		tlsEgressConfig,
		tlsIngressConfig,
		metricstore.WithMetrics(m.debugRegistrar),
		metricstore.WithAddr(m.cfg.Addr),
		metricstore.WithIngressAddr(m.cfg.IngressAddr),
		metricstore.WithAlertmanagerAddr(m.cfg.AlertmanagerAddr),
		metricstore.WithRulesPath(m.cfg.RulesPath),
		metricstore.WithLogger(m.log),
		metricstore.WithQueryTimeout(m.cfg.QueryTimeout),
	)

	store.Start()

	sigs := make(chan os.Signal, 1)
	done := make(chan bool, 1)

	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigs
		m.log.Info("received signal", logger.String("signal", sig.String()))
		close(done)
	}()

	<-done
}

// Stop stops all the subprocesses for the application.
func (m *MetricStoreApp) Stop() {
	m.debugMu.Lock()
	defer m.debugMu.Unlock()

	m.debugLis.Close()
	m.debugLis = nil
}

func (m *MetricStoreApp) startDebugServer() {
	m.debugMu.Lock()
	defer m.debugMu.Unlock()

	m.debugRegistrar = debug.NewRegistrar(
		m.log,
		"metric_store",
		debug.WithGauge(debug.MetricStoreDiskFreeRatio, prometheus.GaugeOpts{
			Help: "Percentage of free space on persistent disk",
		}),
		debug.WithCounter(debug.MetricStoreWrittenPointsTotal, prometheus.CounterOpts{
			Help: "Number of points successfully written to storage engine",
		}),
		debug.WithCounter(debug.MetricStoreExpiredShardsTotal, prometheus.CounterOpts{
			Help: "Number of shards removed due to time-based expiration",
		}),
		debug.WithCounter(debug.MetricStorePrunedShardsTotal, prometheus.CounterOpts{
			Help: "Number of shards removed due to disk space threshold",
		}),
		debug.WithGauge(debug.MetricStoreStorageDays, prometheus.GaugeOpts{
			Help: "Days of data stored on disk",
		}),
		debug.WithGauge(debug.MetricStoreIndexSize, prometheus.GaugeOpts{
			Help: "Size of the index",
		}),
		debug.WithGauge(debug.MetricStoreSeriesCount, prometheus.GaugeOpts{
			Help: "Number of unique series stored in the index",
		}),
		debug.WithGauge(debug.MetricStoreMeasurementsCount, prometheus.GaugeOpts{
			Help: "Number of unique measurements stored in the index",
		}),
		debug.WithCounter(debug.MetricStoreReadErrorsTotal, prometheus.CounterOpts{
			Help: "Number of errors encountered reading from the storage engine",
		}),
		debug.WithGauge(debug.MetricStoreTagValuesQueryDurationSeconds, prometheus.GaugeOpts{
			Help: "Time spent retrieving tag values from the storage engine",
		}),
		debug.WithGauge(debug.MetricStoreMeasurementNamesQueryDurationSeconds, prometheus.GaugeOpts{
			Help: "Time spent retrieving measurement names from the storage engine",
		}),
	)

	debugAddr := fmt.Sprintf("localhost:%d", m.cfg.HealthPort)
	m.debugLis = debug.StartServer(
		debugAddr,
		m.debugRegistrar.Registry(),
		m.log,
	)
}

func newDiskFreeReporter(storagePath string, log *logger.Logger, metrics debug.MetricRegistrar) func() float64 {
	return func() float64 {
		diskFree, err := system_stats.DiskFree(storagePath)

		if err != nil {
			log.Error("failed to get disk free space", err, logger.String("path", storagePath))
			return 0
		}

		metrics.Set(debug.MetricStoreDiskFreeRatio, diskFree/100)
		return diskFree
	}
}
