package app

import (
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/cloudfoundry/metric-store-release/src/internal/debug"
	"github.com/cloudfoundry/metric-store-release/src/internal/logger"
	"github.com/cloudfoundry/metric-store-release/src/pkg/auth"
	. "github.com/cloudfoundry/metric-store-release/src/pkg/cfauthproxy"
	sharedtls "github.com/cloudfoundry/metric-store-release/src/pkg/tls"
	"github.com/prometheus/client_golang/prometheus"
)

type CFAuthProxyApp struct {
	cfg *Config
	log *logger.Logger

	debugMu        sync.Mutex
	debugLis       net.Listener
	debugRegistrar *debug.Registrar
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
	c.debugMu.Lock()
	defer c.debugMu.Unlock()

	if c.debugLis != nil {
		return c.debugLis.Addr().String()
	}

	return ""
}

// Run starts the CFAuthProxyApp, this is a blocking method call.
func (c *CFAuthProxyApp) Run() {
	c.startDebugServer()

	uaaClient := auth.NewUAAClient(
		c.cfg.UAA.Addr,
		buildUAAClient(c.cfg, c.log),
		c.debugRegistrar,
		c.log,
	)

	// try to get our first token key, but bail out if we can't talk to UAA
	err := uaaClient.RefreshTokenKeys()
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

	proxyCACertPool := loadCA(c.cfg.ProxyCAPath, c.log)

	proxy := NewCFAuthProxy(
		c.cfg.MetricStoreAddr,
		c.cfg.Addr,
		c.cfg.CertPath,
		c.cfg.KeyPath,
		proxyCACertPool,
		WithAuthMiddleware(middlewareProvider.Middleware),
		WithCFAuthProxyBlock(),
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
}

// Stop stops all the subprocesses for the application.
func (c *CFAuthProxyApp) Stop() {
	c.debugMu.Lock()
	defer c.debugMu.Unlock()

	c.debugLis.Close()
	c.debugLis = nil
}

func (c *CFAuthProxyApp) startDebugServer() {
	c.debugMu.Lock()
	defer c.debugMu.Unlock()

	c.debugRegistrar = debug.NewRegistrar(
		c.log,
		"metric_store_cf_auth_proxy",
		debug.WithDefaultRegistry(),
		debug.WithHistogram(debug.AuthProxyRequestDurationSeconds, prometheus.HistogramOpts{
			Help:    "Duration in seconds of requests made to the auth proxy",
			Buckets: []float64{.001, .01, .05, .1, .2, 1, 2, 5},
		}),
		debug.WithHistogram(debug.AuthProxyCAPIRequestDurationSeconds, prometheus.HistogramOpts{
			Help:    "Duration in seconds of external requests made to CAPI",
			Buckets: []float64{.001, .01, .05, .1, .2, 1},
		}),
	)

	debugAddr := fmt.Sprintf("localhost:%d", c.cfg.HealthPort)
	c.debugLis = debug.StartServer(
		debugAddr,
		c.debugRegistrar.Gatherer(),
		c.log,
	)
}

func buildUAAClient(cfg *Config, log *logger.Logger) *http.Client {
	tlsConfig := sharedtls.NewBaseTLSConfig()
	tlsConfig.InsecureSkipVerify = cfg.SkipCertVerify

	tlsConfig.RootCAs = loadCA(cfg.UAA.CAPath, log)

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
	tlsConfig := sharedtls.NewBaseTLSConfig()
	tlsConfig.ServerName = cfg.CAPI.CommonName

	tlsConfig.RootCAs = loadCA(cfg.CAPI.CAPath, log)

	tlsConfig.InsecureSkipVerify = cfg.SkipCertVerify
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

func loadCA(caCertPath string, log *logger.Logger) *x509.CertPool {
	caCert, err := ioutil.ReadFile(caCertPath)
	if err != nil {
		log.Fatal("failed to read CA certificate", err)
	}

	certPool := x509.NewCertPool()
	ok := certPool.AppendCertsFromPEM(caCert)
	if !ok {
		log.Fatal("failed to parse CA certificate.", nil)
	}

	return certPool
}
