package app

import (
	"crypto/tls"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/cloudfoundry/metric-store-release/src/internal/blackbox"
	"github.com/cloudfoundry/metric-store-release/src/internal/blackbox/metricscanner"
	"github.com/cloudfoundry/metric-store-release/src/internal/debug"
	"github.com/cloudfoundry/metric-store-release/src/internal/metricstore"
	sharedtls "github.com/cloudfoundry/metric-store-release/src/internal/tls"
	"github.com/cloudfoundry/metric-store-release/src/pkg/egressclient"
	"github.com/cloudfoundry/metric-store-release/src/pkg/ingressclient"
	"github.com/cloudfoundry/metric-store-release/src/pkg/logger"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

type BlackboxApp struct {
	cfg *blackbox.Config
	log *logger.Logger

	pc *blackbox.PerformanceCalculator
	rc *blackbox.ReliabilityCalculator

	debugMu        sync.Mutex
	debugLis       net.Listener
	debugRegistrar *debug.Registrar
}

func NewBlackboxApp(cfg *blackbox.Config, log *logger.Logger) *BlackboxApp {
	return &BlackboxApp{
		cfg: cfg,
		log: log,
	}
}

func (b *BlackboxApp) Run() {
	tlsMetricsConfig, err := sharedtls.NewMutualTLSServerConfig(
		b.cfg.MetricStoreMetricsTLS.CAPath,
		b.cfg.MetricStoreMetricsTLS.CertPath,
		b.cfg.MetricStoreMetricsTLS.KeyPath,
	)
	if err != nil {
		b.log.Fatal("unable to create metrics TLS config", err)
	}

	b.startDebugServer(tlsMetricsConfig)
	// resolver, _ := naming.NewDNSResolverWithFreq(1 * time.Minute)

	stopChan := make(chan bool)

	egressClient, err := egressclient.NewEgressClient(b.cfg.MetricStoreHTTPAddr, b.cfg.UaaAddr, b.cfg.ClientID, b.cfg.ClientSecret)
	if err != nil {
		b.log.Fatal("error creating egress client", err)
	}

	b.StartCalculators(egressClient, stopChan)
	// not sure exactly what we want from this
	//b.StartMetricScanner(egressClient)

	<-stopChan
}

func (b *BlackboxApp) StartCalculators(egressClient blackbox.QueryableClient, stopChan chan bool) {
	tlsConfig, err := sharedtls.NewMutualTLSClientConfig(
		b.cfg.TLS.CAPath,
		b.cfg.TLS.CertPath,
		b.cfg.TLS.KeyPath,
		metricstore.COMMON_NAME,
	)
	if err != nil {
		b.log.Fatal("invalid mTLS configuration for metric-store communication", err)
	}

	b.StartReliabilityCalculator(tlsConfig, egressClient, stopChan)
	b.StartPerformanceCalculator(tlsConfig, egressClient, stopChan)
}

func (b *BlackboxApp) StartPerformanceCalculator(tlsConfig *tls.Config, egressClient blackbox.QueryableClient, stopChan chan bool) {
	ingressClient, err := ingressclient.NewIngressClient(b.cfg.MetricStoreIngressAddr, tlsConfig)
	if err != nil {
		b.log.Fatal("performance: could not connect metric-store ingress client", err)
	}

	b.pc = blackbox.NewPerformanceCalculator(b.cfg, b.log, b.debugRegistrar)
	go b.pc.CalculatePerformance(egressClient, stopChan)
	go b.pc.EmitPerformanceTestMetrics(b.cfg.SourceId, time.Second, ingressClient, stopChan)
}

func (b *BlackboxApp) StartReliabilityCalculator(tlsConfig *tls.Config, egressClient blackbox.QueryableClient, stopChan chan bool) {
	ingressClient, err := ingressclient.NewIngressClient(b.cfg.MetricStoreIngressAddr, tlsConfig)
	if err != nil {
		b.log.Fatal("reliability: could not connect metric-store ingress client", err)
	}

	rc := &blackbox.ReliabilityCalculator{
		SampleInterval:   b.cfg.SampleInterval,
		WindowInterval:   b.cfg.WindowInterval,
		WindowLag:        b.cfg.WindowLag,
		EmissionInterval: b.cfg.EmissionInterval,
		SourceId:         b.cfg.SourceId,
		Log:              b.log,
		DebugRegistrar:   b.debugRegistrar,
	}
	go rc.EmitReliabilityMetrics(ingressClient, stopChan)
	go rc.CalculateReliability(egressClient, stopChan)
}

func (b *BlackboxApp) StartMetricScanner(egressClient blackbox.QueryableClient) {
	go func() {
		b.log.Info("metric scanner: starting")
		t := time.NewTicker(15 * time.Minute)
		scanner := metricscanner.NewMetricScanner(egressClient, b.debugRegistrar, b.log)
		for range t.C {
			err := scanner.TestCurrentMetrics()
			if err != nil {
				b.log.Error("metric scanner: couldn't get current metrics", err)
			}
		}
	}()
}

// DebugAddr returns the address (host and port) that the debug server is bound
// to. If the debug server has not been started an empty string will be returned.
func (b *BlackboxApp) DebugAddr() string {
	b.debugMu.Lock()
	defer b.debugMu.Unlock()

	if b.debugLis != nil {
		return b.debugLis.Addr().String()
	}

	return ""
}

// Stop stops all the subprocesses for the application.
func (b *BlackboxApp) Stop() {
	b.debugMu.Lock()
	defer b.debugMu.Unlock()

	b.debugLis.Close()
	b.debugLis = nil
}

func (b *BlackboxApp) startDebugServer(tlsConfig *tls.Config) {
	b.debugMu.Lock()
	defer b.debugMu.Unlock()

	b.debugRegistrar = debug.NewRegistrar(
		b.log,
		"blackbox",
		debug.WithDefaultRegistry(),
		debug.WithGauge(blackbox.HttpReliability, prometheus.GaugeOpts{
			Help: "Proportion of expected metrics posted to metrics queried"}),
		debug.WithCounter(blackbox.MalfunctioningMetricsTotal, prometheus.CounterOpts{
			Help: "Number of metric query failues encounters"}),
		debug.WithGauge(blackbox.BlackboxPerformanceLatency, prometheus.GaugeOpts{
			Help: "Time to perform a benchmark query against blackbox_performance_canary"}),
		debug.WithGauge(blackbox.BlackboxPerformanceCount, prometheus.GaugeOpts{
			Help: "Number of metrics retrieved by benchmark query against blackbox_performance_canary"}),
	)

	debugAddr := fmt.Sprintf("localhost:%d", b.cfg.HealthPort)
	b.log.Info("\n serving metrics on", zap.String("debug address", debugAddr))
	b.debugLis = debug.StartServer(
		debugAddr,
		tlsConfig,
		b.debugRegistrar.Gatherer(),
		b.log,
	)
}
