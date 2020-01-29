package main

import (
	_ "net/http/pprof"

	"github.com/cloudfoundry/metric-store-release/src/cmd/nozzle/app"
	"github.com/cloudfoundry/metric-store-release/src/pkg/logger"
)

func main() {
	cfg := app.LoadConfig()

	log := logger.NewLogger(cfg.LogLevel, "nozzle")
	log.Info("starting")
	defer log.Info("exiting")

	app.NewNozzleApp(cfg, log).Run()
}
