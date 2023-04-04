package api_test

import (
	"context"
	"crypto/tls"
	"go.uber.org/atomic"
	"net"
	"net/http"

	. "github.com/cloudfoundry/metric-store-release/src/internal/api"
	"github.com/cloudfoundry/metric-store-release/src/internal/testing"
	sharedtls "github.com/cloudfoundry/metric-store-release/src/internal/tls"
	"github.com/cloudfoundry/metric-store-release/src/pkg/logger"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type reloadApiTestContext struct {
	addr       string
	httpClient *http.Client
}

func (tc *reloadApiTestContext) Reload() (resp *http.Response, err error) {
	return tc.httpClient.Post(
		"https://"+tc.addr+"/~/reload",
		"application/json",
		nil)
}

var reloadCalls atomic.Int32

var _ = Describe("Reload API", func() {
	var setup = func() (*reloadApiTestContext, func()) {
		tlsServerConfig, err := sharedtls.NewMutualTLSServerConfig(
			testing.Cert("metric-store-ca.crt"),
			testing.Cert("metric-store.crt"),
			testing.Cert("metric-store.key"),
		)
		Expect(err).ToNot(HaveOccurred())

		tlsClientConfig, err := sharedtls.NewMutualTLSClientConfig(
			testing.Cert("metric-store-ca.crt"),
			testing.Cert("metric-store.crt"),
			testing.Cert("metric-store.key"),
			"metric-store",
		)
		Expect(err).ToNot(HaveOccurred())

		insecureConnection, err := net.Listen("tcp", ":0")
		Expect(err).ToNot(HaveOccurred())

		secureConnection := tls.NewListener(insecureConnection, tlsServerConfig)
		mux := http.NewServeMux()

		reloadConfig := func() {
			reloadCalls.Inc()
		}

		reloadAPI := NewReloadAPI(reloadConfig, logger.NewTestLogger(GinkgoWriter))
		reloadAPIRouter := reloadAPI.Router()
		mux.Handle("/~/", http.StripPrefix("/~", reloadAPIRouter))
		server := &http.Server{Handler: mux}
		go server.Serve(secureConnection)

		httpClient := &http.Client{
			Transport: &http.Transport{TLSClientConfig: tlsClientConfig},
		}

		return &reloadApiTestContext{
				httpClient: httpClient,
				addr:       secureConnection.Addr().String(),
			}, func(server *http.Server) func() {
				return func() {
					server.Shutdown(context.Background())
				}
			}(server)
	}

	Describe("POST /~/reload", func() {
		It("reloads the scrape configs", func() {
			tc, teardown := setup()
			defer teardown()

			resp, err := tc.Reload()
			Expect(err).ToNot(HaveOccurred())
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(200))
			Expect(reloadCalls.Load()).To(Equal(int32(1)))
		})

	})
})
