package main

import (
	_ "net/http/pprof"

	"github.com/cloudfoundry/metric-store-release/src/cmd/cf-auth-proxy/app"
	"github.com/cloudfoundry/metric-store-release/src/pkg/logger"
)

func main() {
	cfg := app.LoadConfig()

	log := logger.NewLogger(cfg.LogLevel, "cf-auth-proxy")
	log.Info("starting")
	defer log.Info("exiting")

	app.NewCFAuthProxyApp(cfg, log).Run()
}
