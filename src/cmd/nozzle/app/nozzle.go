package app

import (
	"crypto/tls"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"code.cloudfoundry.org/go-loggregator"
	"github.com/prometheus/client_golang/prometheus"

	metric_store "github.com/cloudfoundry/metric-store-release/src/internal/metric-store"
	"github.com/cloudfoundry/metric-store-release/src/internal/metrics"
	. "github.com/cloudfoundry/metric-store-release/src/internal/nozzle"
	sharedtls "github.com/cloudfoundry/metric-store-release/src/internal/tls"
	"github.com/cloudfoundry/metric-store-release/src/pkg/logger"
)

// All application metrics should have AppId or ApplicationGuid tags
const (
	AppId           = "app_id"
	ApplicationGuid = "applicationGuid"
)

type NozzleApp struct {
	cfg *Config
	log *logger.Logger

	metricsMutex  sync.Mutex
	metricsServer *metrics.Server
	metrics       metrics.Registrar

	profilingMutex    sync.Mutex
	profilingListener net.Listener
}

func NewNozzleApp(cfg *Config, log *logger.Logger) *NozzleApp {
	return &NozzleApp{
		cfg: cfg,
		log: log,
	}
}

func (app *NozzleApp) MetricsAddr() string {
	app.metricsMutex.Lock()
	defer app.metricsMutex.Unlock()

	if app.metricsServer == nil {
		return ""
	}
	return app.metricsServer.Addr()

}

// Run starts the Nozzle, this is a blocking method call.
func (app *NozzleApp) Run() {
	tlsMetricsConfig, err := sharedtls.NewMutualTLSServerConfig(
		app.cfg.MetricStoreMetricsTLS.CAPath,
		app.cfg.MetricStoreMetricsTLS.CertPath,
		app.cfg.MetricStoreMetricsTLS.KeyPath,
	)
	if err != nil {
		app.log.Fatal("unable to create metrics TLS config", err)
	}

	app.startDebugServer(tlsMetricsConfig) // TODO rename
	app.startProfilingServer()

	loggregatorTLSConfig, err := loggregator.NewEgressTLSConfig(
		app.cfg.LogsProviderTLS.LogProviderCA,
		app.cfg.LogsProviderTLS.LogProviderCert,
		app.cfg.LogsProviderTLS.LogProviderKey,
	)
	if err != nil {
		app.log.Fatal("failed to load tls config for loggregator", err)
	}

	streamConnector := loggregator.NewEnvelopeStreamConnector(
		app.cfg.LogProviderAddr,
		loggregatorTLSConfig,
		loggregator.WithEnvelopeStreamLogger(app.log.StdLog("loggregator")),
		loggregator.WithEnvelopeStreamBuffer(10000, func(missed int) {
			app.log.Info("dropped envelope batches", logger.Count(missed))
			app.metrics.Add(metrics.NozzleDroppedEnvelopesTotal, float64(missed))
		}),
	)

	metricStoreTLSConfig, err := sharedtls.NewMutualTLSClientConfig(
		app.cfg.MetricStoreTLS.CAPath,
		app.cfg.MetricStoreTLS.CertPath,
		app.cfg.MetricStoreTLS.KeyPath,
		metric_store.COMMON_NAME,
	)
	if err != nil {
		app.log.Fatal("failed to load tls config for metric store", err)
	}

	envelopSelectorTags := append([]string{AppId, ApplicationGuid}, app.cfg.EnvelopSelectorTags...)

	nozzle := NewNozzle(
		streamConnector,
		app.cfg.IngressAddr,
		metricStoreTLSConfig,
		app.cfg.ShardId,
		app.cfg.NodeIndex,
		app.cfg.EnableEnvelopeSelector,
		envelopSelectorTags,
		WithNozzleLogger(app.log),
		WithNozzleDebugRegistrar(app.metrics),
		WithNozzleTimerRollup(
			10*time.Second,
			[]string{
				"status_code", "app_name", "app_id", "space_name",
				"space_id", "organization_name", "organization_id",
				"process_id", "process_instance_id", "process_type",
				"instance_id",
			},
			[]string{
				"app_name", "app_id", "space_name", "space_id",
				"organization_name", "organization_id", "process_id",
				"process_instance_id", "process_type", "instance_id",
			},
		),
		WithNozzleTimerRollupBufferSize(app.cfg.TimerRollupBufferSize),
	)

	nozzle.Start()

	sigs := make(chan os.Signal, 1)
	done := make(chan bool, 1)

	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigs
		app.log.Info("received signal", logger.String("signal", sig.String()))
		app.Stop()
		close(done)
	}()

	<-done
}

// Stop stops all the subprocesses for the application.
func (app *NozzleApp) Stop() {
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

func (app *NozzleApp) startDebugServer(tlsConfig *tls.Config) {
	app.metrics = metrics.NewRegistrar(
		app.log,
		"metric-store-nozzle",
		metrics.WithConstLabels(map[string]string{
			"source_id": "nozzle",
		}),
		metrics.WithCounter(metrics.NozzleIngressEnvelopesTotal, prometheus.CounterOpts{
			Help: "Total number of envelopes ingressed by the nozzle",
		}),
		metrics.WithCounter(metrics.NozzleDroppedEnvelopesTotal, prometheus.CounterOpts{
			Help: "Total number of envelopes dropped within the nozzle",
		}),
		metrics.WithCounter(metrics.NozzleSkippedEnvelopsByTagTotal, prometheus.CounterOpts{
			Help: "Total number of envelopes skipped within the nozzle",
		}),
		metrics.WithCounter(metrics.NozzleDroppedPointsTotal, prometheus.CounterOpts{
			Help: "Total number of points dropped within the nozzle",
		}),
		metrics.WithCounter(metrics.NozzleEgressPointsTotal, prometheus.CounterOpts{
			Help: "Total number of points egressed by the nozzle",
		}),
		metrics.WithCounter(metrics.NozzleEgressErrorsTotal, prometheus.CounterOpts{
			Help: "Total number of egress errors within the nozzle",
		}),
		metrics.WithHistogram(metrics.NozzleEgressDurationSeconds, prometheus.HistogramOpts{
			Help:    "Total duration in seconds of egress within the nozzle",
			Buckets: []float64{.001, .01, .05, .1, .2, 1},
		}),
	)

	app.metricsMutex.Lock()
	app.metricsServer = metrics.StartMetricsServer(
		app.cfg.MetricsAddr,
		tlsConfig,
		app.log,
		app.metrics,
	)
	app.metricsMutex.Unlock()
}

func (app *NozzleApp) ProfilingAddr() string {
	app.profilingMutex.Lock()
	defer app.profilingMutex.Unlock()

	if app.profilingListener == nil {
		return ""
	}

	return app.profilingListener.Addr().String()

}

func (app *NozzleApp) startProfilingServer() {
	app.profilingMutex.Lock()
	app.profilingListener = metrics.StartProfilingServer(app.cfg.ProfilingAddr, app.log)
	app.profilingMutex.Unlock()
}
