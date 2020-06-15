package app

import (
	"io/ioutil"
	"net/http"

	"github.com/cloudfoundry/metric-store-release/src/internal/blackbox"
	"github.com/cloudfoundry/metric-store-release/src/internal/testing"
	"github.com/cloudfoundry/metric-store-release/src/internal/tls"
	"github.com/cloudfoundry/metric-store-release/src/pkg/logger"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Blackbox App", func() {
	var (
		bb *BlackboxApp
	)

	BeforeEach(func() {
		bb = NewBlackboxApp(&blackbox.Config{
			TLS: tls.TLS{
				CAPath:   testing.Cert("metric-store-ca.crt"),
				CertPath: testing.Cert("metric-store.crt"),
				KeyPath:  testing.Cert("metric-store.key"),
			},
			MetricStoreMetricsTLS: blackbox.MetricStoreMetricsTLS{
				CAPath:   testing.Cert("metric-store-ca.crt"),
				CertPath: testing.Cert("metric-store.crt"),
				KeyPath:  testing.Cert("metric-store.key"),
			},
		}, logger.NewTestLogger(GinkgoWriter))
		go bb.Run()
		Eventually(bb.MetricsAddr).ShouldNot(BeEmpty())
	})

	AfterEach(func() {
		bb.Stop()
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
			resp, err := httpClient.Get("https://" + bb.MetricsAddr() + "/metrics")
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
		Expect(body).To(ContainSubstring("blackbox_http_reliability"))
		Expect(body).To(ContainSubstring("go_threads"))
	})
})
