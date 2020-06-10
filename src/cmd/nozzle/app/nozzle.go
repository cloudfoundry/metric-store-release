package app

import (
	"crypto/tls"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	loggregator "code.cloudfoundry.org/go-loggregator"
	"github.com/cloudfoundry/metric-store-release/src/internal/debug"
	"github.com/cloudfoundry/metric-store-release/src/internal/metric-store"
	. "github.com/cloudfoundry/metric-store-release/src/internal/nozzle"
	sharedtls "github.com/cloudfoundry/metric-store-release/src/internal/tls"
	"github.com/cloudfoundry/metric-store-release/src/pkg/logger"
	"github.com/prometheus/client_golang/prometheus"
)

type NozzleApp struct {
	cfg *Config
	log *logger.Logger

	debugMu        sync.Mutex
	debugLis       net.Listener
	debugRegistrar *debug.Registrar
}

func NewNozzleApp(cfg *Config, log *logger.Logger) *NozzleApp {
	return &NozzleApp{
		cfg: cfg,
		log: log,
	}
}

// DebugAddr returns the address (host and port) that the debug server is bound
// to. If the debug server has not been started an empty string will be returned.
func (n *NozzleApp) DebugAddr() string {
	n.debugMu.Lock()
	defer n.debugMu.Unlock()

	if n.debugLis != nil {
		return n.debugLis.Addr().String()
	}

	return ""
}

// Run starts the Nozzle, this is a blocking method call.
func (n *NozzleApp) Run() {
	tlsMetricsConfig, err := sharedtls.NewMutualTLSServerConfig(
		n.cfg.MetricStoreMetricsTLS.CAPath,
		n.cfg.MetricStoreMetricsTLS.CertPath,
		n.cfg.MetricStoreMetricsTLS.KeyPath,
	)
	if err != nil {
		n.log.Fatal("unable to create metrics TLS config", err)
	}

	n.startDebugServer(tlsMetricsConfig)

	loggregatorTLSConfig, err := loggregator.NewEgressTLSConfig(
		n.cfg.LogsProviderTLS.LogProviderCA,
		n.cfg.LogsProviderTLS.LogProviderCert,
		n.cfg.LogsProviderTLS.LogProviderKey,
	)
	if err != nil {
		n.log.Fatal("failed to load tls config for loggregator", err)
	}

	streamConnector := loggregator.NewEnvelopeStreamConnector(
		n.cfg.LogProviderAddr,
		loggregatorTLSConfig,
		loggregator.WithEnvelopeStreamLogger(n.log.StdLog("loggregator")),
		loggregator.WithEnvelopeStreamBuffer(10000, func(missed int) {
			n.log.Info("dropped envelope batches", logger.Count(missed))
			n.debugRegistrar.Add(debug.NozzleDroppedEnvelopesTotal, float64(missed))
		}),
	)

	metricStoreTLSConfig, err := sharedtls.NewMutualTLSClientConfig(
		n.cfg.MetricStoreTLS.CAPath,
		n.cfg.MetricStoreTLS.CertPath,
		n.cfg.MetricStoreTLS.KeyPath,
		metric_store.COMMON_NAME,
	)
	if err != nil {
		n.log.Fatal("failed to load tls config for metric store", err)
	}

	nozzle := NewNozzle(
		streamConnector,
		n.cfg.MetricStoreAddr,
		n.cfg.IngressAddr,
		metricStoreTLSConfig,
		n.cfg.ShardId,
		n.cfg.NodeIndex,
		WithNozzleLogger(n.log),
		WithNozzleDebugRegistrar(n.debugRegistrar),
		WithNozzleTimerRollup(
			10*time.Second,
			[]string{
				"status_code", "app_name", "app_id", "space_name",
				"space_id", "organization_name", "organization_id",
				"process_id", "process_instance_id", "process_type",
			},
			[]string{
				"app_name", "app_id", "space_name", "space_id",
				"organization_name", "organization_id", "process_id",
				"process_instance_id", "process_type",
			},
		),
		WithNozzleTimerRollupBufferSize(n.cfg.TimerRollupBufferSize),
	)

	nozzle.Start()

	sigs := make(chan os.Signal, 1)
	done := make(chan bool, 1)

	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigs
		n.log.Info("received signal", logger.String("signal", sig.String()))
		n.Stop()
		close(done)
	}()

	<-done
}

// Stop stops all the subprocesses for the application.
func (n *NozzleApp) Stop() {
	n.debugMu.Lock()
	defer n.debugMu.Unlock()

	n.debugLis.Close()
	n.debugLis = nil
}

func (n *NozzleApp) startDebugServer(tlsConfig *tls.Config) {
	n.debugMu.Lock()
	defer n.debugMu.Unlock()

	n.debugRegistrar = debug.NewRegistrar(
		n.log,
		"metric-store-nozzle",
		debug.WithConstLabels(map[string]string{
			"source_id": "nozzle",
		}),
		debug.WithCounter(debug.NozzleIngressEnvelopesTotal, prometheus.CounterOpts{
			Help: "Total number of envelopes ingressed by the nozzle",
		}),
		debug.WithCounter(debug.NozzleDroppedEnvelopesTotal, prometheus.CounterOpts{
			Help: "Total number of envelopes dropped within the nozzle",
		}),
		debug.WithCounter(debug.NozzleDroppedPointsTotal, prometheus.CounterOpts{
			Help: "Total number of points dropped within the nozzle",
		}),
		debug.WithCounter(debug.NozzleEgressPointsTotal, prometheus.CounterOpts{
			Help: "Total number of points egressed by the nozzle",
		}),
		debug.WithCounter(debug.NozzleEgressErrorsTotal, prometheus.CounterOpts{
			Help: "Total number of egress errors within the nozzle",
		}),
		debug.WithHistogram(debug.NozzleEgressDurationSeconds, prometheus.HistogramOpts{
			Help:    "Total duration in seconds of egress within the nozzle",
			Buckets: []float64{.001, .01, .05, .1, .2, 1},
		}),
	)

	n.debugLis = debug.StartServer(
		n.cfg.MetricsAddr,
		tlsConfig,
		n.debugRegistrar.Gatherer(),
		n.log,
	)
}
