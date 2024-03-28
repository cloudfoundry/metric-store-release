package app

import (
	"crypto/tls"
	"github.com/prometheus/client_golang/prometheus"
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"code.cloudfoundry.org/go-loggregator"
	metric_store "github.com/cloudfoundry/metric-store-release/src/internal/metric-store"
	"github.com/cloudfoundry/metric-store-release/src/internal/metrics"
	. "github.com/cloudfoundry/metric-store-release/src/internal/nozzle"
	sharedtls "github.com/cloudfoundry/metric-store-release/src/internal/tls"
	"github.com/cloudfoundry/metric-store-release/src/pkg/ingressclient"
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

	otelMutex  sync.Mutex
	otelServer *OtelServer

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
	return app.metricsServer.Addr().String()

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

	metricStoreTLSConfig, err := sharedtls.NewMutualTLSClientConfig(
		app.cfg.MetricStoreTLS.CAPath,
		app.cfg.MetricStoreTLS.CertPath,
		app.cfg.MetricStoreTLS.KeyPath,
		metric_store.COMMON_NAME,
	)
	if err != nil {
		app.log.Fatal("failed to load tls config for metric store", err)
	}

	tagAllowList := append([]string{AppId, ApplicationGuid}, app.cfg.EnvelopSelectorTags...)
	// If there is a firehose connect to that
	if app.cfg.FirehoseEnabled {
		loggregatorTLSConfig, err := loggregator.NewEgressTLSConfig(
			app.cfg.LogsProviderTLS.LogProviderCA,
			app.cfg.LogsProviderTLS.LogProviderCert,
			app.cfg.LogsProviderTLS.LogProviderKey,
		)
		if err != nil {
			app.log.Fatal("failed to load tls config for loggregator", err)
		}

		app.log.Info("Firehose enabled and connecting to that")
		streamConnector := loggregator.NewEnvelopeStreamConnector(
			app.cfg.LogProviderAddr,
			loggregatorTLSConfig,
			loggregator.WithEnvelopeStreamLogger(app.log.StdLog("loggregator")),
			loggregator.WithEnvelopeStreamBuffer(10000, func(missed int) {
				app.log.Info("dropped envelope batches", logger.Count(missed))
				app.metrics.Add(metrics.NozzleDroppedEnvelopesTotal, float64(missed))
			}),
		)

		nozzle := NewNozzle(
			streamConnector,
			app.cfg.IngressAddr,
			metricStoreTLSConfig,
			app.cfg.ShardId,
			app.cfg.NodeIndex,
			app.cfg.FilterMetrics,
			tagAllowList,
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
	} else {
		// otherwise connect to the otel server
		otelTLSConfig, err := sharedtls.NewMutualTLSClientConfig(
			app.cfg.OtelProviderTLS.CAPath,
			app.cfg.OtelProviderTLS.CertPath,
			app.cfg.OtelProviderTLS.KeyPath,
			metric_store.COMMON_NAME,
		)
		if err != nil {
			app.log.Fatal("failed to load otel tls config for metric store", err)
		}
		client, err := ingressclient.NewIngressClient(
			app.cfg.IngressAddr,
			metricStoreTLSConfig,
			ingressclient.WithIngressClientLogger(app.log),
			ingressclient.WithDialTimeout(time.Minute),
		)
		if err != nil {
			log.Panic("Could not create to ingress server for otel")
		}

		app.log.Info("Otel is enabled")
		ms := NewMetricService(client,
			WithMetricOtelLogger(app.log),
			WithMetricOtelDebugRegistrar(app.metrics),
			WithMetricFiltering(tagAllowList, app.cfg.FilterMetrics),
		)
		ts := NewTraceService(client, app.cfg.NodeIndex,
			WithTraceOtelLogger(app.log),
			WithTraceOtelDebugRegistrar(app.metrics),
			WithTraceFiltering(tagAllowList, app.cfg.FilterMetrics),
			WithOtelTimerRollup(
				10*time.Second,
				[]string{
					"app_id",
					"app_name",
					"instance_id",
					"organization_id",
					"organization_name",
					"process_id",
					"process_instance_id",
					"process_type",
					"space_id",
					"space_name",
					"status_code",
					"uri",
				},
				[]string{
					"app_id",
					"app_name",
					"instance_id",
					"organization_id",
					"organization_name",
					"process_id",
					"process_instance_id",
					"process_type",
					"space_id",
					"space_name",
					"uri",
				},
			),
			WithOtelTimerRollupBufferSize(app.cfg.TimerRollupBufferSize))
		app.startOtelServer(ms, ts, otelTLSConfig)
	}

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

	if app.otelServer != nil {
		app.otelMutex.Lock()
		if err := app.otelServer.Close(); err != nil {
			app.log.Error("closing otel server", err)
		}
		app.otelMutex.Unlock()
	}

}

func (app *NozzleApp) startDebugServer(tlsConfig *tls.Config) {
	app.metrics = metrics.NewRegistrar(
		app.log,
		"metric-store-nozzle",
		metrics.WithConstLabels(map[string]string{
			"source_id": "nozzle",
		}),
		metrics.WithCounter(metrics.NozzleIngressEnvelopesTotal, prometheus.CounterOpts{
			Help: "Total number of envelopes ingested by the nozzle",
		}),
		metrics.WithCounter(metrics.OtelIngressSpansTotal, prometheus.CounterOpts{
			Help: "Total number of spans ingested via otel-collector",
		}),
		metrics.WithCounter(metrics.OtelIngressMetricsTotal, prometheus.CounterOpts{
			Help: "Total number of metrics ingested via otel-collector",
		}),
		metrics.WithCounter(metrics.OtelDroppedSpansTotal, prometheus.CounterOpts{
			Help: "Total number of spans dropped from collector",
		}),
		metrics.WithCounter(metrics.OtelDroppedMetricsTotal, prometheus.CounterOpts{
			Help: "Total number of metrics dropped from collector",
		}),
		metrics.WithCounter(metrics.OtelDeniedSpansTotal, prometheus.CounterOpts{
			Help: "Total number of spans lacking required tags from otel-collector",
		}),
		metrics.WithCounter(metrics.OtelDeniedMetricsTotal, prometheus.CounterOpts{
			Help: "Total number of metrics lacking required tags from otel-collector",
		}),
		metrics.WithCounter(metrics.OtelEgressSpansTotal, prometheus.CounterOpts{
			Help: "Total number of spans egressed by the collector",
		}),
		metrics.WithCounter(metrics.OtelEgressMetricsTotal, prometheus.CounterOpts{
			Help: "Total number of metrics egressed by the collector",
		}),
		metrics.WithCounter(metrics.OtelEgressSpansErrorsTotal, prometheus.CounterOpts{
			Help: "Total number of span egress errors within the collector",
		}),
		metrics.WithCounter(metrics.OtelEgressMetricsErrorsTotal, prometheus.CounterOpts{
			Help: "Total number of metrics egress errors within the collector",
		}),
		metrics.WithHistogram(metrics.OtelEgressSpansDurationSeconds, prometheus.HistogramOpts{
			Help:    "Total duration in seconds of span egress with otel",
			Buckets: []float64{.001, .01, .05, .1, .2, 1},
		}),
		metrics.WithHistogram(metrics.OtelEgressMetricsDurationSeconds, prometheus.HistogramOpts{
			Help:    "Total duration in seconds of metric egress within otel",
			Buckets: []float64{.001, .01, .05, .1, .2, 1},
		}),
		metrics.WithCounter(metrics.NozzleDroppedEnvelopesTotal, prometheus.CounterOpts{
			Help: "Total number of envelopes dropped within the nozzle",
		}),
		metrics.WithCounter(metrics.NozzleSkippedEnvelopsByTagTotal, prometheus.CounterOpts{
			Help: "Total number of envelopes skipped by tag within the nozzle",
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

func (app *NozzleApp) startOtelServer(ms *MetricService, ts *TraceService, otelTLSConfig *tls.Config) {
	app.otelMutex.Lock()
	app.otelServer = NewOtelServer(app.log, ms, ts)
	app.otelServer.Start(app.cfg.OtelAddr, otelTLSConfig)
	app.otelMutex.Unlock()
}
