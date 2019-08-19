package cfauthproxy

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/cloudfoundry/metric-store-release/src/pkg/auth"
	sharedtls "github.com/cloudfoundry/metric-store-release/src/pkg/tls"
)

type CFAuthProxy struct {
	blockOnStart bool
	ln           net.Listener

	metricStoreURL  *url.URL
	addr            string
	certPath        string
	keyPath         string
	proxyCACertPool *x509.CertPool
	clientTLSConfig *tls.Config

	authMiddleware   func(http.Handler) http.Handler
	accessMiddleware func(http.Handler) *auth.AccessHandler
}

func NewCFAuthProxy(metricStoreAddr, addr, certPath, keyPath string, proxyCACertPool *x509.CertPool, opts ...CFAuthProxyOption) *CFAuthProxy {
	// Force communication with Metric Store to happen via HTTPS
	metricStoreURL, err := url.Parse(fmt.Sprintf("https://%s", metricStoreAddr))
	if err != nil {
		log.Fatalf("failed to parse metric-store address: %s", err)
	}

	p := &CFAuthProxy{
		metricStoreURL:  metricStoreURL,
		addr:            addr,
		certPath:        certPath,
		keyPath:         keyPath,
		proxyCACertPool: proxyCACertPool,
		authMiddleware: func(h http.Handler) http.Handler {
			return h
		},
		accessMiddleware: auth.NewNullAccessMiddleware(),
	}

	for _, o := range opts {
		o(p)
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
		p.clientTLSConfig, err = sharedtls.NewMutualTLSConfig(caCert, cert, key, cn)
		if err != nil {
			log.Fatalf("failed to create client TLS config: %s", err)
		}
	}
}

// Start starts the HTTP listener and serves the HTTP server. If the
// CFAuthProxy was initialized with the WithCFAuthProxyBlock option this
// method will block.
func (p *CFAuthProxy) Start() {
	ln, err := net.Listen("tcp", p.addr)
	if err != nil {
		log.Fatalf("failed to start listener: %s", err)
	}

	p.ln = ln

	server := http.Server{
		Handler:   p.accessMiddleware(p.authMiddleware(p.reverseProxy())),
		TLSConfig: sharedtls.NewBaseTLSConfig(),
	}

	if p.blockOnStart {
		log.Fatal(server.ServeTLS(ln, p.certPath, p.keyPath))
	}

	go func() {
		log.Fatal(server.ServeTLS(ln, p.certPath, p.keyPath))
	}()
}

// Addr returns the listener address. This must be called after calling Start.
func (p *CFAuthProxy) Addr() string {
	return p.ln.Addr().String()
}

func (p *CFAuthProxy) reverseProxy() *httputil.ReverseProxy {
	proxy := httputil.NewSingleHostReverseProxy(p.metricStoreURL)

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
		TLSClientConfig: &tls.Config{
			RootCAs: p.proxyCACertPool,
		},
	}
	if p.clientTLSConfig != nil {
		transport.TLSClientConfig = p.clientTLSConfig
	}
	proxy.Transport = transport

	return proxy
}
