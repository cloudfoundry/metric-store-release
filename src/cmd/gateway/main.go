package main

import (
	"log"
	"os"

	_ "expvar"
	"net/http"
	_ "net/http/pprof"

	envstruct "code.cloudfoundry.org/go-envstruct"
	. "github.com/cloudfoundry/metric-store-release/src/pkg/gateway"
	"google.golang.org/grpc"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	log.Print("Starting Metric Store Gateway...")
	defer log.Print("Closing Metric Store Gateway.")

	cfg, err := LoadConfig()
	if err != nil {
		log.Fatalf("invalid configuration: %s", err)
	}

	envstruct.WriteReport(cfg)

	gateway := NewGateway(cfg.MetricStoreAddr, cfg.Addr, cfg.CertPath, cfg.KeyPath,
		WithGatewayLogger(log.New(os.Stderr, "[GATEWAY] ", log.LstdFlags)),
		WithGatewayMetricStoreDialOpts(
			grpc.WithTransportCredentials(cfg.TLS.Credentials("metric-store")),
		),
	)

	gateway.Start()

	// Start listening on health endpoint and block forever
	http.ListenAndServe(cfg.HealthAddr, nil)
}
