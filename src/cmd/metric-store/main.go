package main

import (
	_ "net/http/pprof"

	"github.com/cloudfoundry/metric-store-release/src/cmd/metric-store/app"
	"github.com/cloudfoundry/metric-store-release/src/internal/metric-store"
	"github.com/cloudfoundry/metric-store-release/src/internal/version"
	"github.com/cloudfoundry/metric-store-release/src/pkg/logger"
)

func main() {
	cfg := app.LoadConfig()

	log := logger.NewLogger(cfg.LogLevel, "metric-store")
	log.Info("starting", logger.String("version", version.VERSION), logger.String("sha", metric_store.SHA))
	defer log.Info("exiting")

	app.NewMetricStoreApp(cfg, log).Run()
}
