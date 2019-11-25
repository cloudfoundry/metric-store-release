package main

import (
	_ "net/http/pprof"

	"github.com/cloudfoundry/metric-store-release/src/cmd/metric-store/app"
	"github.com/cloudfoundry/metric-store-release/src/internal/logger"
	"github.com/cloudfoundry/metric-store-release/src/internal/metricstore"
	"github.com/cloudfoundry/metric-store-release/src/internal/version"
)

func main() {
	cfg := app.LoadConfig()

	log := logger.NewLogger(cfg.LogLevel, "metric-store")
	log.Info("starting", logger.String("version", version.VERSION), logger.String("sha", metricstore.SHA))
	defer log.Info("exiting")

	app.NewMetricStoreApp(cfg, log).Run()
}
