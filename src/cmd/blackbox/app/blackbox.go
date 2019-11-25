package app

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/cloudfoundry/metric-store-release/src/internal/blackbox"
	"github.com/cloudfoundry/metric-store-release/src/internal/blackbox/metricscanner"
	"github.com/cloudfoundry/metric-store-release/src/internal/metricstore"
	"github.com/cloudfoundry/metric-store-release/src/internal/debug"
	"github.com/cloudfoundry/metric-store-release/src/pkg/egressclient"
	"github.com/cloudfoundry/metric-store-release/src/pkg/ingressclient"
	"github.com/cloudfoundry/metric-store-release/src/internal/logger"
	sharedtls "github.com/cloudfoundry/metric-store-release/src/pkg/tls"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

type BlackboxApp struct {
	cfg *Config
	log *logger.Logger

	debugMu        sync.Mutex
	debugLis       net.Listener
	debugRegistrar *debug.Registrar
}

func NewBlackboxApp(cfg *Config, log *logger.Logger) *BlackboxApp {
	return &BlackboxApp{
		cfg: cfg,
		log: log,
	}
}

func (b *BlackboxApp) Run() {
	b.startDebugServer()
	// resolver, _ := naming.NewDNSResolverWithFreq(1 * time.Minute)

	stopChan := make(chan bool)

	egressClient, err := egressclient.NewEgressClient(b.cfg.MetricStoreHTTPAddr, b.cfg.UaaAddr, b.cfg.ClientID, b.cfg.ClientSecret)
	if err != nil {
		b.log.Fatal("error creating egress client", err)
	}

	b.StartReliabilityCalculator(egressClient, stopChan)
	b.StartMetricScanner(egressClient)

	<-stopChan
}

func (b *BlackboxApp) StartReliabilityCalculator(egressClient blackbox.QueryableClient, stopChan chan bool) {
	tlsConfig, err := sharedtls.NewMutualTLSConfig(
		b.cfg.TLS.CAPath,
		b.cfg.TLS.CertPath,
		b.cfg.TLS.KeyPath,
		metricstore.COMMON_NAME,
	)
	if err != nil {
		b.log.Fatal("reliability: invalid mTLS configuration for metric-store communication", err)
	}

	ingressClient, err := ingressclient.NewIngressClient(b.cfg.MetricStoreIngressAddr, tlsConfig)
	if err != nil {
		b.log.Fatal("reliability: could not connect metric-store ingress client", err)
	}

	bb := blackbox.NewBlackbox(b.log)
	go bb.StartEmittingTestMetrics(b.cfg.SourceId, b.cfg.EmissionInterval, ingressClient, stopChan)
	labels := map[string][]string{
		"app_id":              []string{"bde5831e-a819-4a34-9a46-012fd2e821e6b"},
		"app_name":            []string{"bblog"},
		"bosh_environment":    []string{"vpc-bosh-run-pivotal-io"},
		"deployment":          []string{"pws-diego-cellblock-09"},
		"index":               []string{"9b74a5b1-9af9-4715-a57a-bd28ad7e7f1b"},
		"instance_id":         []string{"0", "1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11", "12", "13", "14", "15"},
		"ip":                  []string{"10.10.148.146"},
		"job":                 []string{"diego-cell"},
		"organization_id":     []string{"ab2de77b-a484-4690-9201-b8eaf707fd87"},
		"organization_name":   []string{"blars"},
		"origin":              []string{"rep"},
		"process_id":          []string{"328de02b-79f1-4f8d-b3b2-b81112809603"},
		"process_instance_id": []string{"2652349b-4d40-4b51-4165-7129"},
		"process_type":        []string{"web"},
		"source_id":           []string{"5ee5831e-a819-4a34-9a46-012fd2e821e7"},
		"space_id":            []string{"eb94778d-66b5-4804-abcb-e9efd7f725aa"},
		"space_name":          []string{"bblog"},
		"unit":                []string{"percentage"},
	}
	go bb.StartEmittingPerformanceTestMetrics(b.cfg.SourceId, time.Second, ingressClient, stopChan, labels)

	go func() {
		b.log.Info("reliability: starting calculator")
		t := time.NewTicker(b.cfg.SampleInterval)

		rc := blackbox.ReliabilityCalculator{
			SampleInterval:   b.cfg.SampleInterval,
			WindowInterval:   b.cfg.WindowInterval,
			WindowLag:        b.cfg.WindowLag,
			EmissionInterval: b.cfg.EmissionInterval,
			SourceId:         b.cfg.SourceId,
			Log:              b.log,
		}

		pc := blackbox.NewPerformanceCalculator(b.cfg.SourceId)

		for range t.C {
			httpReliability, err := rc.Calculate(egressClient)
			if err != nil {
				b.log.Error("reliability: error calculating reliability", err)
			}

			b.debugRegistrar.Set(blackbox.BlackboxHTTPReliability, httpReliability)
			b.log.Info("calculated reliability", zap.Float64("percent", httpReliability*100))

			perf, err := pc.Calculate(egressClient)
			if err != nil {
				b.log.Error("error calculating performance", err)
				continue
			}

			latency := perf.Latency.Seconds()
			quantity := float64(perf.Magnitude)

			b.debugRegistrar.Set(blackbox.BlackboxPerformanceLatency, latency)
			b.debugRegistrar.Set(blackbox.BlackboxPerformanceCount, quantity)
			b.log.Info("calculated performance", zap.Float64("duration", latency), zap.Float64("count", quantity))
		}
	}()
}

func (b *BlackboxApp) StartMetricScanner(egressClient blackbox.QueryableClient) {
	go func() {
		b.log.Info("metric scanner: starting")
		t := time.NewTicker(2 * time.Minute)
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

func (b *BlackboxApp) startDebugServer() {
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
		b.debugRegistrar.Gatherer(),
		b.log,
	)
}
