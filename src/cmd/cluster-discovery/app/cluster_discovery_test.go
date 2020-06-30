package app_test

import (
	. "github.com/onsi/ginkgo"
)

var _ = Describe("Cluster Discovery App", func() {
	var (
	//clusterDiscovery *app.ClusterDiscoveryApp
	)

	BeforeEach(func() {
		//metricStore = app.NewMetricStoreApp(&app.Config{
		//	TLS: tls.TLS{
		//		CAPath:   testing.Cert("metric-store-ca.crt"),
		//		CertPath: testing.Cert("metric-store.crt"),
		//		KeyPath:  testing.Cert("metric-store.key"),
		//	},
		//	MetricStoreServerTLS: app.MetricStoreServerTLS{
		//		CAPath:   testing.Cert("metric-store-ca.crt"),
		//		CertPath: testing.Cert("metric-store.crt"),
		//		KeyPath:  testing.Cert("metric-store.key"),
		//	},
		//	MetricStoreInternodeTLS: app.MetricStoreInternodeTLS{
		//		CAPath:   testing.Cert("metric-store-ca.crt"),

		//		CertPath: testing.Cert("metric-store.crt"),
		//		KeyPath:  testing.Cert("metric-store.key"),
		//	},
		//	MetricStoreMetricsTLS: app.MetricStoreMetricsTLS{
		//		CAPath:   testing.Cert("metric-store-ca.crt"),
		//		CertPath: testing.Cert("metric-store.crt"),
		//		KeyPath:  testing.Cert("metric-store.key"),
		//	},
		//	StoragePath: "/tmp/metric-store",
		//}, logger.NewTestLogger(GinkgoWriter))
		//go metricStore.Run()
		//
		//Eventually(metricStore.HealthAddr).ShouldNot(BeEmpty())
	})

	AfterEach(func() {
		//defer metricStore.Stop()
	})

	It("serves metrics on a metrics endpoint", func() {
		//var body string
		//
		//tlsConfig, err := tls.NewMutualTLSClientConfig(
		//	testing.Cert("metric-store-ca.crt"),
		//	testing.Cert("metric-store.crt"),
		//	testing.Cert("metric-store.key"),
		//	"metric-store",
		//)
		//Expect(err).ToNot(HaveOccurred())
		//
		//httpClient := &http.Client{
		//	Transport: &http.Transport{TLSClientConfig: tlsConfig},
		//}
		//
		//fn := func() string {
		//	resp, err := httpClient.get("https://" + metricStore.HealthAddr() + "/metrics")
		//	if err != nil {
		//		return ""
		//	}
		//	defer resp.Body.Close()
		//
		//	bytes, err := ioutil.ReadAll(resp.Body)
		//	if err != nil {
		//		return ""
		//	}
		//
		//	body = string(bytes)
		//
		//	return body
		//}
		//Eventually(fn).ShouldNot(BeEmpty())
		//Expect(body).To(ContainSubstring(debug.MetricStoreWrittenPointsTotal))
		//Expect(body).To(ContainSubstring("go_threads"))
	})
})
