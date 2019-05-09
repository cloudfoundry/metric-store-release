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
	"google.golang.org/grpc"

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

	metrics := metrics.New()
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

	nozzle := NewNozzle(
		streamConnector,
		cfg.MetricStoreAddr,
		cfg.ShardId,
		cfg.NodeIndex,
		WithNozzleLogger(log.New(os.Stderr, "", log.LstdFlags)),
		WithNozzleMetrics(metrics),
		WithNozzleDialOpts(
			grpc.WithTransportCredentials(
				cfg.MetricStoreTLS.Credentials("metric-store"),
			),
		),
		WithNozzleTimerRollup(10*time.Second, "http", []string{"index", "status_code"}),
		WithNozzleTimerRollupBufferSize(cfg.TimerRollupBufferSize),
	)

	go nozzle.Start()

	// Register prometheus-compatible metric endpoint
	http.Handle("/metrics", metrics)

	// Start listening on metrics/health endpoint and block forever
	http.ListenAndServe(fmt.Sprintf("localhost:%d", cfg.HealthPort), nil)
}
