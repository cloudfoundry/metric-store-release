package app

import (
	"fmt"
	"github.com/cloudfoundry/metric-store-release/src/internal/cluster-discovery/pks"
	"github.com/cloudfoundry/metric-store-release/src/internal/cluster-discovery/store"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/cloudfoundry/metric-store-release/src/internal/cluster-discovery"
	"github.com/cloudfoundry/metric-store-release/src/internal/debug"
	sharedtls "github.com/cloudfoundry/metric-store-release/src/internal/tls"
	"github.com/cloudfoundry/metric-store-release/src/pkg/auth"
	"github.com/cloudfoundry/metric-store-release/src/pkg/logger"
)

type ClusterDiscoveryApp struct {
	cfg *Config
	log *logger.Logger

	healthMutex  sync.Mutex
	healthServer net.Listener
	metrics      *debug.Registrar
}

func NewClusterDiscoveryApp(cfg *Config, log *logger.Logger) *ClusterDiscoveryApp {
	return &ClusterDiscoveryApp{
		cfg: cfg,
		log: log,
	}
}

// HealthAddr returns the address (host and port) of the health server, if any.
func (app *ClusterDiscoveryApp) HealthAddr() string {
	app.healthMutex.Lock()
	defer app.healthMutex.Unlock()

	if app.healthServer != nil {
		return app.healthServer.Addr().String()
	}

	return ""
}

// Run starts the ClusterDiscoveryApp, this is a blocking method call.
func (app *ClusterDiscoveryApp) Run() {
	app.startHealthServer()
	clusterDiscovery := app.startClusterDiscovery()
	app.waitForSigTerm(clusterDiscovery)
}

func (app *ClusterDiscoveryApp) waitForSigTerm(clusterDiscovery *cluster_discovery.ClusterDiscovery) {
	sigs := make(chan os.Signal, 1)
	done := make(chan bool, 1)

	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigs
		app.log.Info("received signal", logger.String("signal", sig.String()))
		clusterDiscovery.Stop()
		app.Stop()
		close(done)
	}()

	<-done
}

func (app *ClusterDiscoveryApp) startClusterDiscovery() *cluster_discovery.ClusterDiscovery {
	pksTlsConfig, err := sharedtls.NewTLSClientConfig(app.cfg.PKS.CAPath, app.cfg.PKS.CommonName)
	if err != nil {
		app.log.Fatal("unable to create PKS TLS config", err)
	}

	uaaTlsConfig, err := sharedtls.NewUAATLSConfig(app.cfg.UAA.CAPath, true)
	if err != nil {
		app.log.Fatal("unable to create UAA TLS config", err)
	}

	scrapeConfigStore, err := store.LoadCertStore(app.cfg.StoragePath, app.log)
	if err != nil {
		app.log.Fatal("unable to create scrapeConfig store", err)
	}

	clusterDiscovery := cluster_discovery.New(
		scrapeConfigStore,
		pks.NewClusterLookup(
			app.cfg.PKS.API,
			pksTlsConfig,
			app.log,
		),
		auth.NewUAAClient(
			app.cfg.UAA.Addr,
			&http.Client{
				Transport: &http.Transport{TLSClientConfig: uaaTlsConfig},
			},
			app.metrics,
			app.log,
			auth.WithClientCredentials(app.cfg.UAA.Client, app.cfg.UAA.ClientSecret),
		),
		cluster_discovery.WithLogger(app.log),
		cluster_discovery.WithMetrics(app.metrics),
	)

	clusterDiscovery.Start()
	return clusterDiscovery
}

// Stop stops all the subprocesses for the application.
func (app *ClusterDiscoveryApp) Stop() {
	app.healthMutex.Lock()
	defer app.healthMutex.Unlock()

	app.healthServer.Close()
	app.healthServer = nil
}

func (app *ClusterDiscoveryApp) startHealthServer() {
	app.healthMutex.Lock()
	defer app.healthMutex.Unlock()

	tlsConfig, err := sharedtls.NewMutualTLSServerConfig(
		app.cfg.MetricsTLS.CAPath,
		app.cfg.MetricsTLS.CertPath,
		app.cfg.MetricsTLS.KeyPath,
	)
	if err != nil {
		app.log.Fatal("unable to create metrics TLS config", err)
	}

	app.metrics = debug.NewRegistrar(
		app.log,
		"cluster-discovery",
		debug.WithDefaultRegistry(),
		debug.WithConstLabels(map[string]string{
			"source_id": "cluster-discovery",
		}),
	)

	debugAddr := fmt.Sprintf("localhost:%d", app.cfg.HealthPort)
	app.healthServer = debug.StartServer(
		debugAddr,
		tlsConfig,
		app.metrics.Gatherer(),
		app.log,
	)
}
