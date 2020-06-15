package app_test

import (
	"io/ioutil"
	"net/http"

	"github.com/cloudfoundry/metric-store-release/src/cmd/cf-auth-proxy/app"
	"github.com/cloudfoundry/metric-store-release/src/internal/metrics"
	"github.com/cloudfoundry/metric-store-release/src/internal/testing"
	"github.com/cloudfoundry/metric-store-release/src/internal/tls"
	"github.com/cloudfoundry/metric-store-release/src/pkg/logger"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("CF Auth Proxy App", func() {
	var (
		cfAuthProxy *app.CFAuthProxyApp
	)

	BeforeEach(func() {
		uaaTLSConfig, _ := tls.NewMutualTLSClientConfig(
			testing.Cert("metric-store-ca.crt"),
			testing.Cert("localhost.crt"),
			testing.Cert("localhost.key"),
			"localhost",
		)
		spyUAA := testing.NewSpyUAA(uaaTLSConfig)
		Expect(spyUAA.Start()).To(Succeed())

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
			MetricStoreClientTLS: app.MetricStoreClientTLS{
				CertPath: testing.Cert("metric-store.crt"),
				KeyPath:  testing.Cert("metric-store.key"),
			},
			MetricStoreMetricsTLS: app.MetricStoreMetricsTLS{
				CAPath:   testing.Cert("metric-store-ca.crt"),
				CertPath: testing.Cert("metric-store.crt"),
				KeyPath:  testing.Cert("metric-store.key"),
			},
		}, logger.NewTestLogger(GinkgoWriter))
		go cfAuthProxy.Run()

		Eventually(cfAuthProxy.MetricsAddr).ShouldNot(BeEmpty())
	})

	AfterEach(func() {
		defer func() {
			cfAuthProxy.Stop()
		}()
	})

	It("serves metrics on a metrics endpoint", func() {
		var body string

		tlsConfig, err := tls.NewMutualTLSClientConfig(
			testing.Cert("metric-store-ca.crt"),
			testing.Cert("metric-store.crt"),
			testing.Cert("metric-store.key"),
			"metric-store",
		)
		Expect(err).ToNot(HaveOccurred())

		httpClient := &http.Client{
			Transport: &http.Transport{TLSClientConfig: tlsConfig},
		}

		fn := func() string {
			resp, err := httpClient.Get("https://" + cfAuthProxy.MetricsAddr() + "/metrics")
			if err != nil {
				return ""
			}
			defer func() { _ = resp.Body.Close() }()

			bytes, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				return ""
			}

			body = string(bytes)

			return body
		}
		Eventually(fn).ShouldNot(BeEmpty())
		Expect(body).To(ContainSubstring(metrics.AuthProxyRequestDurationSeconds))
		Expect(body).To(ContainSubstring("go_threads"))
	})
})
