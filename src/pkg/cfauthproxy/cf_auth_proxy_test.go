package cfauthproxy_test

import (
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"

	"github.com/cloudfoundry/metric-store-release/src/internal/testing"
	sharedtls "github.com/cloudfoundry/metric-store-release/src/internal/tls"
	"github.com/cloudfoundry/metric-store-release/src/pkg/auth"
	. "github.com/cloudfoundry/metric-store-release/src/pkg/cfauthproxy"
	"github.com/cloudfoundry/metric-store-release/src/pkg/logger"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("CFAuthProxy", func() {
	Context("while serving TLS", func() {
		It("proxies requests to Metric Store", func() {
			metricstore := startMetricStore("Hello World!")
			defer metricstore.Close()

			proxy := NewCFAuthProxy(
				AddrFromURL(metricstore.URL),
				"127.0.0.1:0",
				testing.Cert("metric-store-ca.crt"),
				logger.NewTestLogger(GinkgoWriter),
				WithServerTLS(
					testing.Cert("metric-store.crt"),
					testing.Cert("metric-store.key"),
				),
				WithClientTLS(
					testing.Cert("metric-store-ca.crt"),
					testing.Cert("metric-store.crt"),
					testing.Cert("metric-store.key"),
					"metric-store",
				),
			)
			proxy.Start()

			resp, err := makeTLSReq("https", proxy.Addr())
			Expect(err).ToNot(HaveOccurred())

			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			body, _ := ioutil.ReadAll(resp.Body)
			Expect(body).To(Equal([]byte("Hello World!")))
		})

		It("returns an error when proxying requests to an insecure metric-store", func() {
			// suppress tls error in test
			log.SetOutput(ioutil.Discard)

			testServer := httptest.NewServer(
				http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
					w.Write([]byte("Insecure MetricStore not allowed, lol"))
				}))
			defer testServer.Close()

			proxy := NewCFAuthProxy(
				AddrFromURL(testServer.URL),
				"127.0.0.1:0",
				testing.Cert("metric-store-ca.crt"),
				logger.NewTestLogger(GinkgoWriter),
				WithServerTLS(
					testing.Cert("metric-store.crt"),
					testing.Cert("metric-store.key"),
				),
			)
			proxy.Start()

			resp, err := makeTLSReq("https", proxy.Addr())
			Expect(err).ToNot(HaveOccurred())

			Expect(resp.StatusCode).To(Equal(http.StatusBadGateway))
		})

		It("successfully connects via TLS to metric-store with mTLS disabled", func() {
			// suppress tls error in test
			log.SetOutput(ioutil.Discard)

			testServer := httptest.NewServer(
				http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
					w.Write([]byte("Hello World!"))
				}))
			defer testServer.Close()

			proxy := NewCFAuthProxy(
				AddrFromURL(testServer.URL),
				"127.0.0.1:0",
				"",
				logger.NewTestLogger(GinkgoWriter),
				WithServerTLS(
					testing.Cert("metric-store.crt"),
					testing.Cert("metric-store.key"),
				),
			)
			proxy.Start()

			resp, err := makeTLSReq("https", proxy.Addr())
			Expect(err).ToNot(HaveOccurred())

			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			body, _ := ioutil.ReadAll(resp.Body)
			Expect(body).To(Equal([]byte("Hello World!")))
		})

		It("delegates to the auth middleware", func() {
			var middlewareCalled bool
			middleware := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				middlewareCalled = true
				w.WriteHeader(http.StatusNotFound)
			})

			proxy := NewCFAuthProxy(
				"127.0.0.1",
				"127.0.0.1:0",
				testing.Cert("metric-store-ca.crt"),
				logger.NewTestLogger(GinkgoWriter),
				WithServerTLS(
					testing.Cert("metric-store.crt"),
					testing.Cert("metric-store.key"),
				),
				WithAuthMiddleware(func(http.Handler) http.Handler {
					return middleware
				}),
			)
			proxy.Start()

			resp, err := makeTLSReq("https", proxy.Addr())
			Expect(err).ToNot(HaveOccurred())

			Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
			Expect(middlewareCalled).To(BeTrue())
		})

		It("delegates to the access middleware", func() {
			var middlewareCalled bool
			middleware := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				middlewareCalled = true
				w.WriteHeader(http.StatusNotFound)
			})

			proxy := NewCFAuthProxy(
				"127.0.0.1",
				"127.0.0.1:0",
				testing.Cert("metric-store-ca.crt"),
				logger.NewTestLogger(GinkgoWriter),
				WithServerTLS(
					testing.Cert("metric-store.crt"),
					testing.Cert("metric-store.key"),
				),
				WithAccessMiddleware(func(http.Handler) *auth.AccessHandler {
					return auth.NewAccessHandler(middleware, auth.NewNullAccessLogger(), "0.0.0.0", "1234", logger.NewTestLogger(GinkgoWriter))
				}),
			)
			proxy.Start()

			resp, err := makeTLSReq("https", proxy.Addr())
			Expect(err).ToNot(HaveOccurred())

			Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
			Expect(middlewareCalled).To(BeTrue())
		})

		It("does not accept unencrypted connections", func() {
			testServer := httptest.NewServer(
				http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {}),
			)
			proxy := NewCFAuthProxy(
				AddrFromURL(testServer.URL),
				"localhost:0",
				testing.Cert("metric-store-ca.crt"),
				logger.NewTestLogger(GinkgoWriter),
				WithServerTLS(
					testing.Cert("metric-store.crt"),
					testing.Cert("metric-store.key"),
				),
			)
			proxy.Start()

			resp, err := makeTLSReq("http", proxy.Addr())
			Expect(err).NotTo(HaveOccurred())

			Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))
		})
	})

	Context("while not serving TLS", func() {
		It("proxies requests to Metric Store", func() {
			metricstore := startMetricStore("Hello World!")
			defer metricstore.Close()

			proxy := NewCFAuthProxy(
				AddrFromURL(metricstore.URL),
				"127.0.0.1:0",
				testing.Cert("metric-store-ca.crt"),
				logger.NewTestLogger(GinkgoWriter),
				WithClientTLS(
					testing.Cert("metric-store-ca.crt"),
					testing.Cert("metric-store.crt"),
					testing.Cert("metric-store.key"),
					"metric-store",
				),
			)
			proxy.Start()

			resp, err := makeTLSReq("http", proxy.Addr())
			Expect(err).ToNot(HaveOccurred())

			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			body, _ := ioutil.ReadAll(resp.Body)
			Expect(body).To(Equal([]byte("Hello World!")))
		})

		It("proxies requests to and insecure Metric Store", func() {
			testServer := httptest.NewServer(
				http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
					w.Write([]byte("Hello World!"))
				}))
			defer testServer.Close()

			proxy := NewCFAuthProxy(
				AddrFromURL(testServer.URL),
				"127.0.0.1:0",
				"",
				logger.NewTestLogger(GinkgoWriter),
			)
			proxy.Start()

			resp, err := makeTLSReq("http", proxy.Addr())
			Expect(err).ToNot(HaveOccurred())

			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			body, _ := ioutil.ReadAll(resp.Body)
			Expect(body).To(Equal([]byte("Hello World!")))
		})
	})
})

func startMetricStore(responseBody string) *httptest.Server {
	testMetricStore := httptest.NewUnstartedServer(
		http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			w.Write([]byte(responseBody))
		}),
	)

	tlsConfig, err := sharedtls.NewMutualTLSServerConfig(
		testing.Cert("metric-store-ca.crt"),
		testing.Cert("metric-store.crt"),
		testing.Cert("metric-store.key"),
	)
	Expect(err).ToNot(HaveOccurred())

	tlsConfig.ClientAuth = tls.RequireAnyClientCert
	testMetricStore.TLS = tlsConfig
	testMetricStore.StartTLS()

	return testMetricStore
}

func makeTLSReq(scheme, addr string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s://%s", scheme, addr), nil)
	Expect(err).ToNot(HaveOccurred())

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}

	return client.Do(req)
}

func AddrFromURL(u string) string {
	url, _ := url.Parse(u)
	return fmt.Sprintf("%s:%s", url.Hostname(), url.Port())
}
