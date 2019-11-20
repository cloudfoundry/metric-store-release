package main

import (
	"github.com/cloudfoundry/metric-store-release/src/cmd/blackbox/app"
	"github.com/cloudfoundry/metric-store-release/src/internal/version"
	"github.com/cloudfoundry/metric-store-release/src/pkg/logger"
)

func main() {
	cfg := app.LoadConfig()

	log := logger.NewLogger(cfg.LogLevel, "blackbox")
	log.Info("starting", logger.String("version", version.VERSION))
	defer log.Info("exiting")

	app.NewBlackboxApp(cfg, log).Run()
}
