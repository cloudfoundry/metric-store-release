package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	_ "net/http/pprof"
	"os"
	"time"

	"crypto/x509"

	envstruct "code.cloudfoundry.org/go-envstruct"
	"github.com/cloudfoundry/metric-store-release/src/pkg/auth"
	. "github.com/cloudfoundry/metric-store-release/src/pkg/cfauthproxy"
	"github.com/cloudfoundry/metric-store-release/src/pkg/metrics"
	logtls "github.com/cloudfoundry/metric-store-release/src/pkg/tls"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	log := log.New(os.Stderr, "", log.LstdFlags)
	log.Print("Starting Metric Store CF Auth Reverse Proxy...")
	defer log.Print("Closing Metric Store CF Auth Reverse Proxy.")

	cfg, err := LoadConfig()
	if err != nil {
		log.Fatalf("failed to load config: %s", err)
	}
	envstruct.WriteReport(cfg)

	metrics := metrics.New("metric-store-cf-auth-proxy")

	uaaClient := auth.NewUAAClient(
		cfg.UAA.Addr,
		buildUAAClient(cfg),
		metrics,
		log,
	)

	// try to get our first token key, but bail out if we can't talk to UAA
	err = uaaClient.RefreshTokenKeys()
	if err != nil {
		log.Fatalf("failed to fetch token from UAA: %s", err)
	}

	capiClient := auth.NewCAPIClient(
		cfg.CAPI.ExternalAddr,
		buildCAPIClient(cfg),
		metrics,
		log,
	)

	queryParser := &QueryParser{}

	middlewareProvider := auth.NewCFAuthMiddlewareProvider(
		uaaClient,
		capiClient,
		queryParser,
		metrics,
	)

	proxyCACertPool := loadCA(cfg.ProxyCAPath)

	proxy := NewCFAuthProxy(
		cfg.MetricStoreGatewayAddr,
		cfg.Addr,
		cfg.CertPath,
		cfg.KeyPath,
		proxyCACertPool,
		WithAuthMiddleware(middlewareProvider.Middleware),
	)

	if cfg.SecurityEventLog != "" {
		accessLog, err := os.OpenFile(cfg.SecurityEventLog, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0666)
		if err != nil {
			log.Panicf("Unable to open access log: %s", err)
		}
		defer func() {
			accessLog.Sync()
			accessLog.Close()
		}()

		_, localPort, err := net.SplitHostPort(cfg.Addr)
		if err != nil {
			log.Panicf("Unable to determine local port: %s", err)
		}

		accessLogger := auth.NewAccessLogger(accessLog)
		accessMiddleware := auth.NewAccessMiddleware(accessLogger, cfg.InternalIP, localPort)
		WithAccessMiddleware(accessMiddleware)(proxy)
	}

	proxy.Start()

	// Register prometheus-compatible metric endpoint
	http.Handle("/metrics", metrics)

	// Start listening on metrics/health endpoint and block forever
	http.ListenAndServe(fmt.Sprintf("localhost:%d", cfg.HealthPort), nil)
}

func buildUAAClient(cfg *Config) *http.Client {
	tlsConfig := logtls.NewBaseTLSConfig()
	tlsConfig.InsecureSkipVerify = cfg.SkipCertVerify

	tlsConfig.RootCAs = loadCA(cfg.UAA.CAPath)

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

func buildCAPIClient(cfg *Config) *http.Client {
	tlsConfig := logtls.NewBaseTLSConfig()
	tlsConfig.ServerName = cfg.CAPI.CommonName

	tlsConfig.RootCAs = loadCA(cfg.CAPI.CAPath)

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

func loadCA(caCertPath string) *x509.CertPool {
	caCert, err := ioutil.ReadFile(caCertPath)
	if err != nil {
		log.Fatalf("failed to read CA certificate: %s", err)
	}

	certPool := x509.NewCertPool()
	ok := certPool.AppendCertsFromPEM(caCert)
	if !ok {
		log.Fatal("failed to parse CA certificate.")
	}

	return certPool
}
