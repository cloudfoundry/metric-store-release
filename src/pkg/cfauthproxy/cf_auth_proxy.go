package cfauthproxy

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	sharedtls "github.com/cloudfoundry/metric-store-release/src/internal/tls"
	"github.com/cloudfoundry/metric-store-release/src/pkg/auth"
	"github.com/cloudfoundry/metric-store-release/src/pkg/logger"
)

type CFAuthProxy struct {
	blockOnStart bool
	ln           net.Listener

	metricStoreURL  *url.URL
	addr            string
	serveTLS        bool
	certPath        string
	keyPath         string
	caPath          string
	clientTLSConfig *tls.Config

	authMiddleware   func(http.Handler) http.Handler
	accessMiddleware func(http.Handler) *auth.AccessHandler

	log *logger.Logger
}

func NewCFAuthProxy(metricStoreAddr, addr, caPath string, log *logger.Logger, opts ...CFAuthProxyOption) *CFAuthProxy {
	p := &CFAuthProxy{
		addr:   addr,
		caPath: caPath,
		authMiddleware: func(h http.Handler) http.Handler {
			return h
		},
		accessMiddleware: auth.NewNullAccessMiddleware(),
		log:              log,
	}

	for _, o := range opts {
		o(p)
	}

	// Force communication with Metric Store to happen via HTTPS
	var err error
	p.metricStoreURL, err = url.Parse(fmt.Sprintf("https://%s", metricStoreAddr))
	if err != nil {
		p.log.Fatal("failed to parse metric-store address", err)
	}

	return p
}

// CFAuthProxyOption configures a CFAuthProxy
type CFAuthProxyOption func(*CFAuthProxy)

// WithCFAuthProxyBlock returns a CFAuthProxyOption that determines if Start
// launches a go-routine or not. It defaults to launching a go-routine. If
// this is set, start will block on serving the HTTP endpoint.
func WithCFAuthProxyBlock() CFAuthProxyOption {
	return func(p *CFAuthProxy) {
		p.blockOnStart = true
	}
}

// WithAuthMiddleware returns a CFAuthProxyOption that sets the CFAuthProxy's
// authentication and authorization middleware.
func WithAuthMiddleware(authMiddleware func(http.Handler) http.Handler) CFAuthProxyOption {
	return func(p *CFAuthProxy) {
		p.authMiddleware = authMiddleware
	}
}

func WithAccessMiddleware(accessMiddleware func(http.Handler) *auth.AccessHandler) CFAuthProxyOption {
	return func(p *CFAuthProxy) {
		p.accessMiddleware = accessMiddleware
	}
}

// WithClientTLS will use client TLS cert and key for communication to the
// proxy destination.
func WithClientTLS(caCert, cert, key, cn string) CFAuthProxyOption {
	return func(p *CFAuthProxy) {
		var err error
		p.clientTLSConfig, err = sharedtls.NewMutualTLSClientConfig(caCert, cert, key, cn)
		if err != nil {
			p.log.Fatal("failed to create client TLS config", err)
		}
	}
}

func WithServerTLS(cert, key string) CFAuthProxyOption {
	return func(p *CFAuthProxy) {
		p.serveTLS = true
		p.certPath = cert
		p.keyPath = key
	}
}

// Start starts the HTTP listener and serves the HTTP server. If the
// CFAuthProxy was initialized with the WithCFAuthProxyBlock option this
// method will block.
func (p *CFAuthProxy) Start() {
	ln, err := net.Listen("tcp", p.addr)
	if err != nil {
		p.log.Fatal("failed to start listener", err)
	}

	p.ln = ln

	if p.serveTLS {
		p.startTLS(ln)
		return
	}

	p.start(ln)
}

func (p *CFAuthProxy) startTLS(ln net.Listener) {
	tlsConfig, err := sharedtls.NewGenericTLSConfig()
	if err != nil {
		p.log.Fatal("failed to create TLS config", err)
	}

	server := http.Server{
		Handler:   p.accessMiddleware(p.authMiddleware(p.reverseProxy())),
		TLSConfig: tlsConfig,
	}

	if p.blockOnStart {
		err = server.ServeTLS(ln, p.certPath, p.keyPath)
		p.log.Fatal("server exiting", err)
	}

	go func() {
		err = server.ServeTLS(ln, p.certPath, p.keyPath)
		p.log.Fatal("server exiting", err)
	}()
}

func (p *CFAuthProxy) start(ln net.Listener) {
	server := http.Server{
		Handler: p.accessMiddleware(p.authMiddleware(p.reverseProxy())),
	}

	if p.blockOnStart {
		err := server.Serve(ln)
		p.log.Fatal("server exiting", err)
	}

	go func() {
		err := server.Serve(ln)
		p.log.Fatal("server exiting", err)
	}()
}

// Addr returns the listener address. This must be called after calling Start.
func (p *CFAuthProxy) Addr() string {
	return p.ln.Addr().String()
}

func (p *CFAuthProxy) reverseProxy() *httputil.ReverseProxy {
	proxy := httputil.NewSingleHostReverseProxy(p.metricStoreURL)

	defaultTLSConfig, err := sharedtls.NewTLSClientConfig(
		p.caPath,
		"metric-store",
	)
	if err != nil {
		p.log.Fatal("failed to create reverse proxy TLS config", err)
	}

	// Aside from the Root CA for the gateway, these values are defaults
	// from Golang's http.DefaultTransport
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
			DualStack: true,
		}).DialContext,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		TLSClientConfig:       defaultTLSConfig,
	}
	if p.clientTLSConfig != nil {
		transport.TLSClientConfig = p.clientTLSConfig
	}
	proxy.Transport = transport

	return proxy
}
