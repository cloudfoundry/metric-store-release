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
	. "github.com/cloudfoundry/metric-store-release/src/pkg/nozzle"
	"github.com/cloudfoundry/metric-store-release/src/pkg/tls"

	loggregator "code.cloudfoundry.org/go-loggregator"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	log.Print("Starting MetricStore Nozzle...")
	defer log.Print("Closing MetricStore Nozzle.")

	cfg, err := LoadConfig()
	if err != nil {
		log.Fatalf("invalid configuration: %s", err)
	}

	envstruct.WriteReport(cfg)

	tlsCfg, err := loggregator.NewEgressTLSConfig(
		cfg.LogsProviderTLS.LogProviderCA,
		cfg.LogsProviderTLS.LogProviderCert,
		cfg.LogsProviderTLS.LogProviderKey,
	)
	if err != nil {
		log.Fatalf("invalid LogsProviderTLS configuration: %s", err)
	}

	metrics := metrics.New("metric-store-nozzle")
	loggr := log.New(os.Stderr, "[LOGGR] ", log.LstdFlags)

	dropped := metrics.NewCounter("nozzle_dropped")
	streamConnector := loggregator.NewEnvelopeStreamConnector(
		cfg.LogProviderAddr,
		tlsCfg,
		loggregator.WithEnvelopeStreamLogger(loggr),
		loggregator.WithEnvelopeStreamBuffer(10000, func(missed int) {
			loggr.Printf("dropped %d envelope batches", missed)
			dropped(uint64(missed))
		}),
	)

	tlsConfig, err := tls.NewMutualTLSConfig(
		cfg.MetricStoreTLS.CAPath,
		cfg.MetricStoreTLS.CertPath,
		cfg.MetricStoreTLS.KeyPath,
		"metric-store",
	)
	if err != nil {
		log.Fatalf("invalid nozzle Mutual TLS for metric-store configuration: %s", err)
	}

	nozzle := NewNozzle(
		streamConnector,
		cfg.MetricStoreAddr,
		cfg.IngressAddr,
		tlsConfig,
		cfg.ShardId,
		cfg.NodeIndex,
		WithNozzleLogger(log.New(os.Stderr, "", log.LstdFlags)),
		WithNozzleMetrics(metrics),
		WithNozzleTimerRollup(10*time.Second, "http", []string{"index", "status_code"}),
		WithNozzleTimerRollupBufferSize(cfg.TimerRollupBufferSize),
	)

	go nozzle.Start()

	// Register prometheus-compatible metric endpoint
	http.Handle("/metrics", metrics)

	// Start listening on metrics/health endpoint and block forever
	http.ListenAndServe(fmt.Sprintf("localhost:%d", cfg.HealthPort), nil)
}
