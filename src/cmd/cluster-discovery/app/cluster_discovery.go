package app

import (
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cloudfoundry/metric-store-release/src/internal/cluster-discovery/pks"
	"github.com/cloudfoundry/metric-store-release/src/internal/cluster-discovery/store"

	cluster_discovery "github.com/cloudfoundry/metric-store-release/src/internal/cluster-discovery"
	"github.com/cloudfoundry/metric-store-release/src/internal/metrics"
	sharedtls "github.com/cloudfoundry/metric-store-release/src/internal/tls"
	"github.com/cloudfoundry/metric-store-release/src/pkg/auth"
	"github.com/cloudfoundry/metric-store-release/src/pkg/logger"
)

type ClusterDiscoveryApp struct {
	cfg *Config
	log *logger.Logger

	metrics        *metrics.Server
	debugRegistrar metrics.Registrar
}

func NewClusterDiscoveryApp(cfg *Config, log *logger.Logger) *ClusterDiscoveryApp {
	return &ClusterDiscoveryApp{
		cfg: cfg,
		log: log,
	}
}

// HealthAddr returns the address (host and port) of the health server, if any.
func (app *ClusterDiscoveryApp) HealthAddr() string {
	if app.metrics != nil {
		return app.metrics.Addr()
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

	tlsConfig, err := sharedtls.NewMutualTLSClientConfig(app.cfg.MetricStoreAPI.CAPath, app.cfg.MetricStoreAPI.CertPath, app.cfg.MetricStoreAPI.KeyPath, app.cfg.MetricStoreAPI.CommonName)

	if err != nil {
		panic(err)
	}
	metricStoreAPIClient := &http.Client{
		Transport: &http.Transport{TLSClientConfig: tlsConfig},
		Timeout:   10 * time.Second,
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
			app.debugRegistrar,
			app.log,
			auth.WithClientCredentials(app.cfg.UAA.Client, app.cfg.UAA.ClientSecret),
		),
		app.cfg.MetricStoreAPI.Address,
		metricStoreAPIClient,
		cluster_discovery.WithLogger(app.log),
		cluster_discovery.WithMetrics(app.debugRegistrar),
		cluster_discovery.WithRefreshInterval(app.cfg.RefreshInterval),
	)

	clusterDiscovery.Start()
	return clusterDiscovery
}

// Stop stops all the subprocesses for the application.
func (app *ClusterDiscoveryApp) Stop() {
	app.metrics.Close()
	app.metrics = nil
}

func (app *ClusterDiscoveryApp) startHealthServer() {
	tlsConfig, err := sharedtls.NewMutualTLSServerConfig(
		app.cfg.MetricsTLS.CAPath,
		app.cfg.MetricsTLS.CertPath,
		app.cfg.MetricsTLS.KeyPath,
	)
	if err != nil {
		app.log.Fatal("unable to create metrics TLS config", err)
	}

	app.debugRegistrar = metrics.NewRegistrar(
		app.log,
		"cluster-discovery",
		metrics.WithConstLabels(map[string]string{
			"source_id": "cluster-discovery",
		}),
	)

	app.metrics = metrics.StartMetricsServer(
		app.cfg.MetricsAddr,
		tlsConfig,
		app.log,
		app.debugRegistrar,
	)
}
