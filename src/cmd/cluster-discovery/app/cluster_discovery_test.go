package app_test

import (
	"fmt"
	"io/ioutil"
	"net/http"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/metric-store-release/src/cmd/cluster-discovery/app"
	"github.com/cloudfoundry/metric-store-release/src/internal/metrics"
	"github.com/cloudfoundry/metric-store-release/src/internal/testing"
	"github.com/cloudfoundry/metric-store-release/src/internal/tls"
	"github.com/cloudfoundry/metric-store-release/src/pkg/logger"
)

var _ = Describe("Cluster Discovery App", func() {
	var (
		clusterDiscovery *app.ClusterDiscoveryApp
	)

	BeforeEach(func() {
		clusterDiscovery = app.NewClusterDiscoveryApp(&app.Config{
			MetricsAddr:     "",
			ProfilingAddr:   "",
			StoragePath:     "/tmp/metric-store",
			LogLevel:        "",
			MetricsTLS:      app.ClusterDiscoveryMetricsTLS{},
			MetricStoreAPI:  app.MetricStoreAPI{},
			PKS:             app.PKSConfig{},
			UAA:             app.UAAConfig{},
			RefreshInterval: 0,
		}, logger.NewTestLogger(GinkgoWriter))
		go clusterDiscovery.Run()

		Eventually(clusterDiscovery.MetricsAddr).ShouldNot(BeEmpty())
	})

	AfterEach(func() {
		defer clusterDiscovery.Stop()
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
			resp, err := httpClient.Get("https:" + clusterDiscovery.MetricsAddr() + "/metrics")
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
		Expect(body).To(ContainSubstring(metrics.MetricStoreWrittenPointsTotal))
		Expect(body).To(ContainSubstring("go_threads"))
	})

	It("listens with pprof", func() {
		callPprof := func() int {

			resp, err := http.Get("http://" + clusterDiscovery.ProfilingAddr() + "/debug/pprof")
			if err != nil {
				fmt.Printf("calling pprof: %s\n", err)
				return -1
			}
			defer func() { _ = resp.Body.Close() }()

			return resp.StatusCode
		}
		Eventually(callPprof).Should(Equal(200))
	})
})
