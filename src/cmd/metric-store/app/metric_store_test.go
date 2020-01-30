package app_test

import (
	"io/ioutil"
	"net/http"

	"github.com/cloudfoundry/metric-store-release/src/cmd/metric-store/app"
	"github.com/cloudfoundry/metric-store-release/src/internal/debug"
	"github.com/cloudfoundry/metric-store-release/src/pkg/logger"
	"github.com/cloudfoundry/metric-store-release/src/internal/testing"
	"github.com/cloudfoundry/metric-store-release/src/internal/tls"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Metric Store App", func() {
	var (
		metricStore *app.MetricStoreApp
	)

	BeforeEach(func() {
		metricStore = app.NewMetricStoreApp(&app.Config{
			TLS: tls.TLS{
				CAPath:   testing.Cert("metric-store-ca.crt"),
				CertPath: testing.Cert("metric-store.crt"),
				KeyPath:  testing.Cert("metric-store.key"),
			},
			MetricStoreServerTLS: app.MetricStoreServerTLS{
				CAPath:   testing.Cert("metric-store-ca.crt"),
				CertPath: testing.Cert("metric-store.crt"),
				KeyPath:  testing.Cert("metric-store.key"),
			},
			MetricStoreInternodeTLS: app.MetricStoreInternodeTLS{
				CAPath:   testing.Cert("metric-store-ca.crt"),
				CertPath: testing.Cert("metric-store.crt"),
				KeyPath:  testing.Cert("metric-store.key"),
			},
			StoragePath: "/tmp/metric-store",
		}, logger.NewTestLogger(GinkgoWriter))
		go metricStore.Run()

		Eventually(metricStore.DebugAddr).ShouldNot(BeEmpty())
	})

	AfterEach(func() {
		defer metricStore.Stop()
	})

	It("serves metrics on a metrics endpoint", func() {
		var body string
		fn := func() string {
			resp, err := http.Get("http://" + metricStore.DebugAddr() + "/metrics")
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
		Expect(body).To(ContainSubstring(debug.MetricStoreWrittenPointsTotal))
		Expect(body).To(ContainSubstring("go_threads"))
	})
})
