package cfauthproxy_test

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/cloudfoundry/metric-store-release/src/pkg/auth"
	. "github.com/cloudfoundry/metric-store-release/src/pkg/cfauthproxy"
	"github.com/cloudfoundry/metric-store-release/src/pkg/testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("CFAuthProxy", func() {
	It("proxies requests to metric-store gateways", func() {
		var called bool
		testServer := httptest.NewServer(
			http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				called = true
			}))

		proxy := NewCFAuthProxy(
			testServer.URL,
			"127.0.0.1:0",
			testing.Cert("localhost.crt"),
			testing.Cert("localhost.key"),
		)
		proxy.Start()

		resp, err := makeTLSReq("https", proxy.Addr())
		Expect(err).ToNot(HaveOccurred())

		Expect(resp.StatusCode).To(Equal(http.StatusOK))
		Expect(called).To(BeTrue())
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

		_, err := makeTLSReq("http", proxy.Addr())
		Expect(err).To(HaveOccurred())
	})
})

func makeTLSReq(scheme, addr string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s://%s", scheme, addr), nil)
	Expect(err).ToNot(HaveOccurred())

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}

	return client.Do(req)
}
