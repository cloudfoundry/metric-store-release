package app

import (
	"crypto/tls"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/cloudfoundry/metric-store-release/src/internal/metric-store"
	"github.com/cloudfoundry/metric-store-release/src/internal/metrics"
	sharedtls "github.com/cloudfoundry/metric-store-release/src/internal/tls"
	"github.com/cloudfoundry/metric-store-release/src/pkg/auth"
	. "github.com/cloudfoundry/metric-store-release/src/pkg/cfauthproxy"
	"github.com/cloudfoundry/metric-store-release/src/pkg/logger"
)

type CFAuthProxyApp struct {
	cfg *Config
	log *logger.Logger

	profilingMutex    sync.Mutex
	profilingListener net.Listener

	metricsMutex  sync.Mutex
	metricsServer *metrics.Server
	metrics       metrics.Registrar
}

func NewCFAuthProxyApp(cfg *Config, log *logger.Logger) *CFAuthProxyApp {
	return &CFAuthProxyApp{
		cfg: cfg,
		log: log,
	}
}

func (app *CFAuthProxyApp) MetricsAddr() string {
	app.metricsMutex.Lock()
	defer app.metricsMutex.Unlock()

	if app.metricsServer == nil {
		return ""
	}
	return app.metricsServer.Addr()

}

// Run starts the CFAuthProxyApp, this is a blocking method call.
func (app *CFAuthProxyApp) Run() {
	tlsMetricsConfig, err := sharedtls.NewMutualTLSServerConfig(
		app.cfg.MetricStoreMetricsTLS.CAPath,
		app.cfg.MetricStoreMetricsTLS.CertPath,
		app.cfg.MetricStoreMetricsTLS.KeyPath,
	)
	if err != nil {
		app.log.Fatal("unable to create metrics TLS config", err)
	}

	app.startMetricsServer(tlsMetricsConfig)
	app.startProfilingServer()

	uaaClient := auth.NewUAAClient(
		app.cfg.UAA.Addr,
		buildUAAClient(app.cfg, app.log),
		app.metrics,
		app.log,
	)

	// try to get our first token key, but bail out if we can't talk to UAA
	err = uaaClient.RefreshTokenKeys()
	if err != nil {
		app.log.Fatal("failed to fetch token from UAA", err)
	}

	capiClient := auth.NewCAPIClient(
		app.cfg.CAPI.ExternalAddr,
		buildCAPIClient(app.cfg, app.log),
		app.metrics,
		app.log,
	)

	queryParser := &QueryParser{}

	middlewareProvider := auth.NewCFAuthMiddlewareProvider(
		uaaClient,
		capiClient,
		queryParser,
		app.metrics,
		app.log,
	)

	proxy := NewCFAuthProxy(
		app.cfg.MetricStoreAddr,
		app.cfg.Addr,
		app.cfg.ProxyCAPath,
		app.log,
		WithAuthMiddleware(middlewareProvider.Middleware),
		WithClientTLS(
			app.cfg.ProxyCAPath,
			app.cfg.MetricStoreClientTLS.CertPath,
			app.cfg.MetricStoreClientTLS.KeyPath,
			metric_store.COMMON_NAME,
		),
		WithServerTLS(
			app.cfg.CertPath,
			app.cfg.KeyPath,
		),
	)

	if app.cfg.SecurityEventLog != "" {
		accessLog, err := os.OpenFile(app.cfg.SecurityEventLog, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0666)
		if err != nil {
			app.log.Panic("unable to open access log", logger.Error(err))
		}
		defer func() {
			accessLog.Sync()
			accessLog.Close()
		}()

		_, localPort, err := net.SplitHostPort(app.cfg.Addr)
		if err != nil {
			app.log.Panic("unable to determine local port", logger.Error(err))
		}

		accessLogger := auth.NewAccessLogger(accessLog, app.log)
		accessMiddleware := auth.NewAccessMiddleware(accessLogger, app.cfg.InternalIP, localPort, app.log)
		WithAccessMiddleware(accessMiddleware)(proxy)
	}

	proxy.Start()

	sigs := make(chan os.Signal, 1)
	done := make(chan bool, 1)

	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigs
		app.log.Info("received signal", logger.String("signal", sig.String()))
		app.Stop()
		close(done)
	}()

	<-done
}

// Stop stops all the subprocesses for the application.
func (app *CFAuthProxyApp) Stop() {
	app.metricsMutex.Lock()
	app.metricsServer.Close()
	app.metricsServer = nil
	app.metricsMutex.Unlock()

	app.profilingMutex.Lock()
	app.profilingListener.Close()
	app.profilingListener = nil
	app.profilingMutex.Unlock()
}

func (app *CFAuthProxyApp) startMetricsServer(tlsConfig *tls.Config) {
	app.metrics = metrics.NewRegistrar(
		app.log,
		"metric_store_cf_auth_proxy",
		metrics.WithConstLabels(map[string]string{
			"source_id": "cf-auth-proxy",
		}),
		metrics.WithHistogram(metrics.AuthProxyRequestDurationSeconds, prometheus.HistogramOpts{
			Help:    "Duration in seconds of requests made to the auth proxy",
			Buckets: []float64{.001, .01, .05, .1, .2, 1, 2, 5, 10, 30},
		}),
		metrics.WithHistogram(metrics.AuthProxyCAPIRequestDurationSeconds, prometheus.HistogramOpts{
			Help:    "Duration in seconds of external requests made to CAPI",
			Buckets: []float64{.001, .01, .05, .1, .2, 1, 2, 5, 10, 30},
		}),
	)

	app.metricsMutex.Lock()
	app.metricsServer = metrics.StartMetricsServer(
		app.cfg.MetricsAddr,
		tlsConfig,
		app.log,
		app.metrics,
	)
	app.metricsMutex.Unlock()
}

func (app *CFAuthProxyApp) ProfilingAddr() string {
	app.profilingMutex.Lock()
	defer app.profilingMutex.Unlock()

	if app.profilingListener == nil {
		return ""
	}
	return app.profilingListener.Addr().String()
}

func (app *CFAuthProxyApp) startProfilingServer() {
	app.profilingMutex.Lock()
	app.profilingListener = metrics.StartProfilingServer(app.cfg.ProfilingAddr, app.log)
	app.profilingMutex.Unlock()
}

func buildUAAClient(cfg *Config, log *logger.Logger) *http.Client {
	tlsConfig, err := sharedtls.NewUAATLSConfig(cfg.UAA.CAPath, cfg.SkipCertVerify)
	if err != nil {
		log.Fatal("failed to load UAA CA certificate", err)
	}

	transport := &http.Transport{
		TLSHandshakeTimeout: 10 * time.Second,
		TLSClientConfig:     tlsConfig,
		DisableKeepAlives:   true,
	}

	return &http.Client{
		Timeout:   20 * time.Second,
		Transport: transport,
	}
}

func buildCAPIClient(cfg *Config, log *logger.Logger) *http.Client {
	tlsConfig, err := sharedtls.NewCAPITLSConfig(cfg.CAPI.CAPath, cfg.SkipCertVerify, cfg.CAPI.CommonName)
	if err != nil {
		log.Fatal("failed to load CAPI CA certificate", err)
	}

	transport := &http.Transport{
		TLSHandshakeTimeout: 10 * time.Second,
		TLSClientConfig:     tlsConfig,
		DisableKeepAlives:   true,
	}

	return &http.Client{
		Timeout:   20 * time.Second,
		Transport: transport,
	}
}
