package testing

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"net/url"

	"go.uber.org/atomic"
)

type ScrapeTargetSpy struct {
	server          *httptest.Server
	scrapesReceived atomic.Int32
	tlsConfig       *tls.Config
}

func NewScrapeTargetSpy(tlsConfig *tls.Config) *ScrapeTargetSpy {
	return &ScrapeTargetSpy{
		tlsConfig: tlsConfig,
	}
}

func (spy *ScrapeTargetSpy) ScrapesReceived() int32 {
	return spy.scrapesReceived.Load()
}

func (spy *ScrapeTargetSpy) receive(w http.ResponseWriter, req *http.Request) {
	_ = req.Body.Close()

	w.WriteHeader(http.StatusOK)
	spy.scrapesReceived.Inc()
}

func (spy *ScrapeTargetSpy) Start() {
	spy.server = httptest.NewUnstartedServer(http.HandlerFunc(spy.receive))
	spy.server.TLS = spy.tlsConfig
	spy.server.StartTLS()
}

func (spy *ScrapeTargetSpy) Stop() {
	spy.server.Close()
}

func (spy *ScrapeTargetSpy) Addr() string {
	addr, _ := url.Parse(spy.server.URL)
	return addr.Host
}
