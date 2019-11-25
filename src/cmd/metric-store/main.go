package main

import (
	_ "net/http/pprof"

	"github.com/cloudfoundry/metric-store-release/src/cmd/metric-store/app"
	"github.com/cloudfoundry/metric-store-release/src/internal/metricstore"
	"github.com/cloudfoundry/metric-store-release/src/internal/version"
	"github.com/cloudfoundry/metric-store-release/src/internal/logger"
)

func main() {
	cfg := app.LoadConfig()

	log := logger.NewLogger(cfg.LogLevel, "metric-store")
	log.Info("starting", logger.String("version", version.VERSION), logger.String("css", metricstore.CSS_SHA), logger.String("oss", metricstore.OSS_SHA))
	defer log.Info("exiting")

	app.NewMetricStoreApp(cfg, log).Run()
}
