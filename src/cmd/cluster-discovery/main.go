package main

import (
	_ "net/http/pprof"

	"github.com/cloudfoundry/metric-store-release/src/internal/metric-store"

	"github.com/cloudfoundry/metric-store-release/src/cmd/cluster-discovery/app"
	"github.com/cloudfoundry/metric-store-release/src/internal/version"
	"github.com/cloudfoundry/metric-store-release/src/pkg/logger"
)

func main() {
	cfg := app.LoadConfig()
	log := logger.NewLogger(cfg.LogLevel, "cluster-discovery")
	log.Info("starting", logger.String("version", version.VERSION), logger.String("sha", metric_store.SHA))
	defer log.Info("exiting")

	app.NewClusterDiscoveryApp(cfg, log).Run()
}
