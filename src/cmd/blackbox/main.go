package main

import (
	"github.com/cloudfoundry/metric-store-release/src/cmd/blackbox/app"
	"github.com/cloudfoundry/metric-store-release/src/internal/blackbox"
	"github.com/cloudfoundry/metric-store-release/src/internal/logger"
	"github.com/cloudfoundry/metric-store-release/src/internal/version"
)

func main() {
	cfg := blackbox.LoadConfig()

	log := logger.NewLogger(cfg.LogLevel, "blackbox")
	log.Info("starting", logger.String("version", version.VERSION))
	defer log.Info("exiting")

	app.NewBlackboxApp(cfg, log).Run()
}
