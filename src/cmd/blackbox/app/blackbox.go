package app

import (
	"crypto/tls"
	"net"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/cloudfoundry/metric-store-release/src/internal/blackbox"
	"github.com/cloudfoundry/metric-store-release/src/internal/metric-store"
	"github.com/cloudfoundry/metric-store-release/src/internal/metrics"
	sharedtls "github.com/cloudfoundry/metric-store-release/src/internal/tls"
	"github.com/cloudfoundry/metric-store-release/src/pkg/egressclient"
	"github.com/cloudfoundry/metric-store-release/src/pkg/ingressclient"
	"github.com/cloudfoundry/metric-store-release/src/pkg/logger"
)

type BlackboxApp struct {
	cfg *blackbox.Config
	log *logger.Logger

	pc *blackbox.PerformanceCalculator
	rc *blackbox.ReliabilityCalculator

	profilingMutex    sync.Mutex
	profilingListener net.Listener

	metricsMutex      sync.Mutex
	metricsServer     *metrics.Server
	metrics           metrics.Registrar
}

func NewBlackboxApp(cfg *blackbox.Config, log *logger.Logger) *BlackboxApp {
	return &BlackboxApp{
		cfg: cfg,
		log: log,
	}
}

func (app *BlackboxApp) Run() {
	tlsMetricsConfig, err := sharedtls.NewMutualTLSServerConfig(
		app.cfg.MetricStoreMetricsTLS.CAPath,
		app.cfg.MetricStoreMetricsTLS.CertPath,
		app.cfg.MetricStoreMetricsTLS.KeyPath,
	)
	if err != nil {
		app.log.Fatal("unable to create metrics TLS config", err)
	}

	app.startMetricsServer(tlsMetricsConfig)
	app.startProfilingServer()
	// resolver, _ := naming.NewDNSResolverWithFreq(1 * time.Minute)

	stopChan := make(chan bool)

	egressClient, err := egressclient.NewEgressClient(app.cfg.MetricStoreHTTPAddr, app.cfg.UaaAddr, app.cfg.ClientID, app.cfg.ClientSecret)
	if err != nil {
		app.log.Fatal("error creating egress client", err)
	}

	app.StartCalculators(egressClient, stopChan)

	<-stopChan
}

func (app *BlackboxApp) StartCalculators(egressClient blackbox.QueryableClient, stopChan chan bool) {
	tlsConfig, err := sharedtls.NewMutualTLSClientConfig(
		app.cfg.TLS.CAPath,
		app.cfg.TLS.CertPath,
		app.cfg.TLS.KeyPath,
		metric_store.COMMON_NAME,
	)
	if err != nil {
		app.log.Fatal("invalid mTLS configuration for metric-store communication", err)
	}

	app.StartReliabilityCalculator(tlsConfig, egressClient, stopChan)
	app.StartPerformanceCalculator(tlsConfig, egressClient, stopChan)
}

func (app *BlackboxApp) StartPerformanceCalculator(tlsConfig *tls.Config, egressClient blackbox.QueryableClient, stopChan chan bool) {
	ingressClient, err := ingressclient.NewIngressClient(app.cfg.MetricStoreIngressAddr, tlsConfig)
	if err != nil {
		app.log.Fatal("performance: could not connect metric-store ingress client", err)
	}

	app.pc = blackbox.NewPerformanceCalculator(app.cfg, app.log, app.metrics)
	go app.pc.CalculatePerformance(egressClient, stopChan)
	go app.pc.EmitPerformanceTestMetrics(app.cfg.SourceId, time.Second, ingressClient, stopChan)
}

func (app *BlackboxApp) StartReliabilityCalculator(tlsConfig *tls.Config, egressClient blackbox.QueryableClient, stopChan chan bool) {
	ingressClient, err := ingressclient.NewIngressClient(app.cfg.MetricStoreIngressAddr, tlsConfig)
	if err != nil {
		app.log.Fatal("reliability: could not connect metric-store ingress client", err)
	}

	rc := &blackbox.ReliabilityCalculator{
		SampleInterval:   app.cfg.SampleInterval,
		WindowInterval:   app.cfg.WindowInterval,
		WindowLag:        app.cfg.WindowLag,
		EmissionInterval: app.cfg.EmissionInterval,
		SourceId:         app.cfg.SourceId,
		Log:              app.log,
		DebugRegistrar:   app.metrics,
	}
	go rc.EmitReliabilityMetrics(ingressClient, stopChan)
	go rc.CalculateReliability(egressClient, stopChan)
}

func (app *BlackboxApp) MetricsAddr() string {
	app.metricsMutex.Lock()
	defer app.metricsMutex.Unlock()

	if app.metricsServer == nil {
		return ""
	}
	return app.metricsServer.Addr().String()

}

// Stop stops all the subprocesses for the application.
func (app *BlackboxApp) Stop() {
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

func (app *BlackboxApp) startMetricsServer(tlsConfig *tls.Config) {
	app.metrics = metrics.NewRegistrar(
		app.log,
		"blackbox",
		metrics.WithGauge(blackbox.HttpReliability, prometheus.GaugeOpts{
			Help: "Proportion of expected metrics posted to metrics queried"}),
		metrics.WithCounter(blackbox.MalfunctioningMetricsTotal, prometheus.CounterOpts{
			Help: "Number of metric query failues encounters"}),
		metrics.WithGauge(blackbox.BlackboxPerformanceLatency, prometheus.GaugeOpts{
			Help: "Time to perform a benchmark query against blackbox_performance_canary"}),
		metrics.WithGauge(blackbox.BlackboxPerformanceCount, prometheus.GaugeOpts{
			Help: "Number of metrics retrieved by benchmark query against blackbox_performance_canary"}),
	)

	app.log.Info("\n serving metrics on", logger.String("address", app.cfg.MetricsAddr))

	app.metricsMutex.Lock()

	app.metricsServer = metrics.StartMetricsServer(
		app.cfg.MetricsAddr,
		tlsConfig,
		app.log,
		app.metrics,
	)

	app.metricsMutex.Unlock()
}

func (app *BlackboxApp) ProfilingAddr() string {
	app.profilingMutex.Lock()
	defer app.profilingMutex.Unlock()

	if app.profilingListener == nil {
		return ""
	}
	return app.profilingListener.Addr().String()
}

func (app *BlackboxApp) startProfilingServer() {
	app.profilingMutex.Lock()
	app.profilingListener = metrics.StartProfilingServer(app.cfg.ProfilingAddr, app.log)
	app.profilingMutex.Unlock()
}
