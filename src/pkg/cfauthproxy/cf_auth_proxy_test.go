package cfauthproxy_test

import (
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"

	"github.com/cloudfoundry/metric-store-release/src/pkg/auth"
	. "github.com/cloudfoundry/metric-store-release/src/pkg/cfauthproxy"
	"github.com/cloudfoundry/metric-store-release/src/pkg/testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("CFAuthProxy", func() {
	It("only proxies requests to a secure log cache gateway", func() {
		gateway := startSecureGateway("Hello World!")
		defer gateway.Close()

		proxy := NewCFAuthProxy(
			gateway.URL,
			"127.0.0.1:0",
			testing.Cert("localhost.crt"),
			testing.Cert("localhost.key"),
		)
		http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{
			InsecureSkipVerify: true,
		}
		proxy.Start()

		resp, err := makeTLSReq("https", proxy.Addr())
		Expect(err).ToNot(HaveOccurred())

		Expect(resp.StatusCode).To(Equal(http.StatusOK))
		body, _ := ioutil.ReadAll(resp.Body)
		Expect(body).To(Equal([]byte("Hello World!")))
	})

	It("upgrades insecure gateway scheme to HTTPS", func() {
		gateway := startSecureGateway("Hello World!")
		defer gateway.Close()

		insecureGatewayURL, err := url.Parse(gateway.URL)
		insecureGatewayURL.Scheme = "http"

		proxy := NewCFAuthProxy(
			insecureGatewayURL.String(),
			"127.0.0.1:0",
			testing.Cert("localhost.crt"),
			testing.Cert("localhost.key"),
		)
		http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{
			InsecureSkipVerify: true,
		}
		proxy.Start()

		resp, err := makeTLSReq("https", proxy.Addr())
		Expect(err).ToNot(HaveOccurred())

		Expect(resp.StatusCode).To(Equal(http.StatusOK))
		body, _ := ioutil.ReadAll(resp.Body)
		Expect(body).To(Equal([]byte("Hello World!")))
	})

	It("returns an error when proxying requests to an insecure log cache gateway", func() {
		// suppress tls error in test
		log.SetOutput(ioutil.Discard)

		testServer := httptest.NewServer(
			http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				w.Write([]byte("Insecure Gateway not allowed, lol"))
			}))
		defer testServer.Close()

		proxy := NewCFAuthProxy(
			testServer.URL,
			"127.0.0.1:0",
			testing.Cert("localhost.crt"),
			testing.Cert("localhost.key"),
		)
		http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{
			InsecureSkipVerify: true,
		}
		proxy.Start()

		resp, err := makeTLSReq("https", proxy.Addr())
		Expect(err).ToNot(HaveOccurred())

		Expect(resp.StatusCode).To(Equal(http.StatusBadGateway))
	})

	It("delegates to the auth middleware", func() {
		var middlewareCalled bool
		middleware := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			middlewareCalled = true
			w.WriteHeader(http.StatusNotFound)
		})

		proxy := NewCFAuthProxy(
			"https://127.0.0.1",
			"127.0.0.1:0",
			testing.Cert("localhost.crt"),
			testing.Cert("localhost.key"),
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
			"https://127.0.0.1",
			"127.0.0.1:0",
			testing.Cert("localhost.crt"),
			testing.Cert("localhost.key"),
			WithAccessMiddleware(func(http.Handler) *auth.AccessHandler {
				return auth.NewAccessHandler(middleware, auth.NewNullAccessLogger(), "0.0.0.0", "1234")
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
			testServer.URL,
			"localhost:0",
			testing.Cert("localhost.crt"),
			testing.Cert("localhost.key"),
		)
		proxy.Start()

		resp, err := makeTLSReq("http", proxy.Addr())
		Expect(err).NotTo(HaveOccurred())

		Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))
	})
})

func startSecureGateway(responseBody string) *httptest.Server {
	testGateway := httptest.NewTLSServer(
		http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			w.Write([]byte(responseBody))
		}),
	)
	return testGateway
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
