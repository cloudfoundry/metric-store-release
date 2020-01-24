package app

import (
	"fmt"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/cloudfoundry/metric-store-release/src/internal/debug"
	"github.com/cloudfoundry/metric-store-release/src/internal/logger"
	"github.com/cloudfoundry/metric-store-release/src/internal/metrics"
	"github.com/cloudfoundry/metric-store-release/src/internal/metricstore"
	"github.com/cloudfoundry/metric-store-release/src/pkg/persistence"
	"github.com/cloudfoundry/metric-store-release/src/pkg/system_stats"
	sharedtls "github.com/cloudfoundry/metric-store-release/src/pkg/tls"
	"github.com/prometheus/client_golang/prometheus"
	config_util "github.com/prometheus/common/config"
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

	tlsEgressConfig := &config_util.TLSConfig{
		CAFile:     m.cfg.TLS.CAPath,
		CertFile:   m.cfg.TLS.CertPath,
		KeyFile:    m.cfg.TLS.KeyPath,
		ServerName: metricstore.COMMON_NAME,
	}

	tlsIngressConfig, err := sharedtls.NewMutualTLSConfig(
		m.cfg.MetricStoreServerTLS.CAPath,
		m.cfg.MetricStoreServerTLS.CertPath,
		m.cfg.MetricStoreServerTLS.KeyPath,
		metricstore.COMMON_NAME,
	)
	if err != nil {
		m.log.Fatal("invalid mTLS configuration for ingress", err)
	}

	tlsInternodeConfig, err := sharedtls.NewMutualTLSConfig(
		m.cfg.MetricStoreInternodeTLS.CAPath,
		m.cfg.MetricStoreInternodeTLS.CertPath,
		m.cfg.MetricStoreInternodeTLS.KeyPath,
		metricstore.COMMON_NAME,
	)
	if err != nil {
		m.log.Fatal("invalid mTLS configuration for internode communication", err)
	}

	diskFreeReporter := system_stats.NewDiskFreeReporter(m.cfg.StoragePath, m.log, m.debugRegistrar)
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
		m.cfg.StoragePath,
		tlsIngressConfig,
		tlsInternodeConfig,
		tlsEgressConfig,
		metricstore.WithMetrics(m.debugRegistrar),
		metricstore.WithAddr(m.cfg.Addr),
		metricstore.WithIngressAddr(m.cfg.IngressAddr),
		metricstore.WithInternodeAddr(m.cfg.InternodeAddr),
		metricstore.WithAlertmanagerAddr(m.cfg.AlertmanagerAddr),
		metricstore.WithRulesPath(m.cfg.RulesPath),
		metricstore.WithClustered(
			m.cfg.NodeIndex,
			m.cfg.NodeAddrs,
			m.cfg.InternodeAddrs,
		),
		metricstore.WithReplicationFactor(m.cfg.ReplicationFactor),
		metricstore.WithHandoffStoragePath(filepath.Join(m.cfg.StoragePath, "handoff")),
		metricstore.WithLogger(m.log),
		metricstore.WithQueryTimeout(m.cfg.QueryTimeout),
		metricstore.WithQueryLogger(filepath.Join(m.cfg.StoragePath, "query.log")),
	)

	store.Start()

	sigs := make(chan os.Signal, 1)
	done := make(chan bool, 1)

	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigs
		m.log.Info("received signal", logger.String("signal", sig.String()))
		store.Close()
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
		"metric-store",
		debug.WithDefaultRegistry(),
		debug.WithConstLabels(map[string]string{"nodeIndex": strconv.Itoa(m.cfg.NodeIndex)}),
		debug.WithCounter(debug.MetricStoreIngressPointsTotal, prometheus.CounterOpts{
			Help: "Number of points ingressed by metric-store",
		}),
		debug.WithCounter(debug.MetricStoreWrittenPointsTotal, prometheus.CounterOpts{
			Help: "Number of points successfully written to storage engine",
		}),
		debug.WithHistogram(debug.MetricStoreWriteDurationSeconds, prometheus.HistogramOpts{
			Help:    "Time spent writing points to the storage engine",
			Buckets: []float64{.001, .01, .05, .1, .2, 1},
		}),
		debug.WithGauge(debug.MetricStoreDiskFreeRatio, prometheus.GaugeOpts{
			Help: "Percentage of free space on persistent disk",
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
		debug.WithGauge(debug.MetricStoreSeriesCount, prometheus.GaugeOpts{
			Help: "Number of unique series stored in the index",
		}),
		debug.WithGauge(debug.MetricStoreMeasurementsCount, prometheus.GaugeOpts{
			Help: "Number of unique measurements stored in the index",
		}),
		debug.WithCounter(debug.MetricStoreReadErrorsTotal, prometheus.CounterOpts{
			Help: "Number of errors encountered reading from the storage engine",
		}),
		debug.WithHistogram(debug.MetricStoreTagValuesQueryDurationSeconds, prometheus.HistogramOpts{
			Help:    "Time spent retrieving tag values from the storage engine",
			Buckets: []float64{.001, .01, .05, .1, .2, 1},
		}),
		debug.WithHistogram(debug.MetricStoreMeasurementNamesQueryDurationSeconds, prometheus.HistogramOpts{
			Help:    "Time spent retrieving measurement names from the storage engine",
			Buckets: []float64{.1, .5, 1, 4, 16},
		}),
		debug.WithLabelledGauge(metrics.MetricStoreReplayerDiskUsageBytes, prometheus.GaugeOpts{
			Help: "Size of a replayer queue",
		}, []string{"targetNodeIndex"}),
		debug.WithLabelledCounter(metrics.MetricStoreReplayerQueueErrorsTotal, prometheus.CounterOpts{
			Help: "Number of errors encountered writing to a replayer queue",
		}, []string{"targetNodeIndex"}),
		debug.WithLabelledCounter(metrics.MetricStoreReplayerQueuedBytesTotal, prometheus.CounterOpts{
			Help: "Number of bytes written to a replayer queue",
		}, []string{"targetNodeIndex"}),
		debug.WithLabelledCounter(metrics.MetricStoreReplayerReadErrorsTotal, prometheus.CounterOpts{
			Help: "Number of errors encountered reading from a replayer queue",
		}, []string{"targetNodeIndex"}),
		debug.WithLabelledCounter(metrics.MetricStoreReplayerReplayErrorsTotal, prometheus.CounterOpts{
			Help: "Number of errors encountered replaying writes to a remote node",
		}, []string{"targetNodeIndex"}),
		debug.WithLabelledCounter(metrics.MetricStoreReplayerReplayedBytesTotal, prometheus.CounterOpts{
			Help: "Number of bytes successfully replayed to a remote node",
		}, []string{"targetNodeIndex"}),
		debug.WithLabelledCounter(metrics.MetricStoreDroppedPointsTotal, prometheus.CounterOpts{
			Help: "Number of points dropped while writing to a remote node",
		}, []string{"targetNodeIndex"}),
		debug.WithLabelledCounter(metrics.MetricStoreDistributedPointsTotal, prometheus.CounterOpts{
			Help: "Number of points successfully distributed to a remote node",
		}, []string{"targetNodeIndex"}),
		debug.WithLabelledGauge(metrics.MetricStoreDistributedRequestDurationSeconds, prometheus.GaugeOpts{
			Help: "Time spent distributing points to a remote node",
		}, []string{"targetNodeIndex"}),
		debug.WithCounter(metrics.MetricStoreCollectedPointsTotal, prometheus.CounterOpts{
			Help: "Number of points collected by a metric-store instance from remote nodes",
		}),
	)

	debugAddr := fmt.Sprintf("localhost:%d", m.cfg.HealthPort)
	m.debugLis = debug.StartServer(
		debugAddr,
		m.debugRegistrar.Gatherer(),
		m.log,
	)
}
