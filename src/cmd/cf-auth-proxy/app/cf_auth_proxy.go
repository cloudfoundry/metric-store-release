package app

import (
	"crypto/tls"
	"net"
	"net/http"
	"os"
	"os/signal"
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

	metrics        *metrics.Server
	debugRegistrar metrics.Registrar // TODO rename or remove
}

func NewCFAuthProxyApp(cfg *Config, log *logger.Logger) *CFAuthProxyApp {
	return &CFAuthProxyApp{
		cfg: cfg,
		log: log,
	}
}

// DebugAddr returns the address (host and port) that the debug server is bound
// to. If the debug server has not been started an empty string will be returned.
func (c *CFAuthProxyApp) DebugAddr() string {
	if c.metrics != nil {
		return c.metrics.Addr()
	}

	return ""
}

// Run starts the CFAuthProxyApp, this is a blocking method call.
func (c *CFAuthProxyApp) Run() {
	tlsMetricsConfig, err := sharedtls.NewMutualTLSServerConfig(
		c.cfg.MetricStoreMetricsTLS.CAPath,
		c.cfg.MetricStoreMetricsTLS.CertPath,
		c.cfg.MetricStoreMetricsTLS.KeyPath,
	)
	if err != nil {
		c.log.Fatal("unable to create metrics TLS config", err)
	}

	c.startMetricsServer(tlsMetricsConfig)

	uaaClient := auth.NewUAAClient(
		c.cfg.UAA.Addr,
		buildUAAClient(c.cfg, c.log),
		c.debugRegistrar,
		c.log,
	)

	// try to get our first token key, but bail out if we can't talk to UAA
	err = uaaClient.RefreshTokenKeys()
	if err != nil {
		c.log.Fatal("failed to fetch token from UAA", err)
	}

	capiClient := auth.NewCAPIClient(
		c.cfg.CAPI.ExternalAddr,
		buildCAPIClient(c.cfg, c.log),
		c.debugRegistrar,
		c.log,
	)

	queryParser := &QueryParser{}

	middlewareProvider := auth.NewCFAuthMiddlewareProvider(
		uaaClient,
		capiClient,
		queryParser,
		c.debugRegistrar,
		c.log,
	)

	proxy := NewCFAuthProxy(
		c.cfg.MetricStoreAddr,
		c.cfg.Addr,
		c.cfg.ProxyCAPath,
		c.log,
		WithAuthMiddleware(middlewareProvider.Middleware),
		WithClientTLS(
			c.cfg.ProxyCAPath,
			c.cfg.MetricStoreClientTLS.CertPath,
			c.cfg.MetricStoreClientTLS.KeyPath,
			metric_store.COMMON_NAME,
		),
		WithServerTLS(
			c.cfg.CertPath,
			c.cfg.KeyPath,
		),
	)

	if c.cfg.SecurityEventLog != "" {
		accessLog, err := os.OpenFile(c.cfg.SecurityEventLog, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0666)
		if err != nil {
			c.log.Panic("unable to open access log", logger.Error(err))
		}
		defer func() {
			accessLog.Sync()
			accessLog.Close()
		}()

		_, localPort, err := net.SplitHostPort(c.cfg.Addr)
		if err != nil {
			c.log.Panic("unable to determine local port", logger.Error(err))
		}

		accessLogger := auth.NewAccessLogger(accessLog, c.log)
		accessMiddleware := auth.NewAccessMiddleware(accessLogger, c.cfg.InternalIP, localPort, c.log)
		WithAccessMiddleware(accessMiddleware)(proxy)
	}

	proxy.Start()

	sigs := make(chan os.Signal, 1)
	done := make(chan bool, 1)

	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigs
		c.log.Info("received signal", logger.String("signal", sig.String()))
		c.Stop()
		close(done)
	}()

	<-done
}

// Stop stops all the subprocesses for the application.
func (c *CFAuthProxyApp) Stop() {
	c.metrics.Close()
	c.metrics = nil
}

func (c *CFAuthProxyApp) startMetricsServer(tlsConfig *tls.Config) {
	c.debugRegistrar = metrics.NewRegistrar(
		c.log,
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

	c.metrics = metrics.StartMetricsServer(
		c.cfg.MetricsAddr,
		tlsConfig,
		c.log,
		c.debugRegistrar,
	)
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
