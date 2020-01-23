package app_test

import (
	"io/ioutil"
	"net/http"

	"github.com/cloudfoundry/metric-store-release/src/cmd/cf-auth-proxy/app"
	"github.com/cloudfoundry/metric-store-release/src/internal/debug"
	"github.com/cloudfoundry/metric-store-release/src/internal/logger"
	"github.com/cloudfoundry/metric-store-release/src/internal/testing"
	"github.com/cloudfoundry/metric-store-release/src/pkg/tls"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("CF Auth Proxy App", func() {
	var (
		cfAuthProxy *app.CFAuthProxyApp
	)

	BeforeEach(func() {
		uaaTLSConfig, _ := tls.NewMutualTLSConfig(
			testing.Cert("metric-store-ca.crt"),
			testing.Cert("localhost.crt"),
			testing.Cert("localhost.key"),
			"localhost",
		)
		spyUAA := testing.NewSpyUAA(uaaTLSConfig)
		spyUAA.Start()

		cfAuthProxy = app.NewCFAuthProxyApp(&app.Config{
			CAPI: app.CAPI{
				ExternalAddr: "",
				CAPath:       testing.Cert("metric-store-ca.crt"),
				CommonName:   "metric-store",
			},
			UAA: app.UAA{
				ClientID:     "metric-store",
				ClientSecret: "secret",
				Addr:         spyUAA.Url(),
				CAPath:       testing.Cert("metric-store-ca.crt"),
			},
			CertPath:    testing.Cert("metric-store.crt"),
			KeyPath:     testing.Cert("metric-store.key"),
			ProxyCAPath: testing.Cert("metric-store-ca.crt"),
		}, logger.NewTestLogger(GinkgoWriter))
		go cfAuthProxy.Run()

		Eventually(cfAuthProxy.DebugAddr).ShouldNot(BeEmpty())
	})

	AfterEach(func() {
		defer func() {
			cfAuthProxy.Stop()
		}()
	})

	It("serves metrics on a metrics endpoint", func() {
		var body string
		fn := func() string {
			resp, err := http.Get("http://" + cfAuthProxy.DebugAddr() + "/metrics")
			if err != nil {
				return ""
			}
			defer resp.Body.Close()

			bytes, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				return ""
			}

			body = string(bytes)

			return body
		}
		Eventually(fn).ShouldNot(BeEmpty())
		Expect(body).To(ContainSubstring(debug.AuthProxyRequestDurationSeconds))
		Expect(body).To(ContainSubstring("go_threads"))
	})
})
