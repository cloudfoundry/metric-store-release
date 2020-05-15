package cluster_discovery_test

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"github.com/cloudfoundry/metric-store-release/src/cmd/cluster-discovery/app"
	"github.com/cloudfoundry/metric-store-release/src/pkg/logger"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/cloudfoundry/metric-store-release/src/internal/cluster-discovery"
	"github.com/cloudfoundry/metric-store-release/src/internal/testing"
	shared "github.com/cloudfoundry/metric-store-release/src/internal/testing"
	sharedtls "github.com/cloudfoundry/metric-store-release/src/internal/tls"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	prometheusConfig "github.com/prometheus/prometheus/config"
)

// Sentinel to detect build failures early
var __ *cluster_discovery.ClusterDiscovery

var _ = Describe("ClusterDiscovery", func() {
	type testContext struct {
		tlsConfig        *tls.Config
		caCert           string
		cert             string
		key              string
		healthPort       int
		scrapeConfigPath string
		storagePath      string

		metricStoreAPIAddress string

		uaaSpy     *testing.SpyUAA
		pksSpy     *testing.PKSSpy
		kubeAPISpy *testing.K8sSpy

		app     *app.ClusterDiscoveryApp
		MetricS interface{}
	}

	var startProcess = func(tc *testContext) {
		tmpDir, err := ioutil.TempDir("", "cluster-discovery")
		Expect(err).ToNot(HaveOccurred())
		tc.storagePath = tmpDir
		tc.scrapeConfigPath = filepath.Join(tmpDir, "scrape_config.yml")

		cfg := &app.Config{
			HealthPort:  tc.healthPort,
			StoragePath: tmpDir,
			LogLevel:    "DEBUG",
			MetricsTLS: app.ClusterDiscoveryMetricsTLS{
				CAPath:   tc.caCert,
				CertPath: tc.cert,
				KeyPath:  tc.key,
			},
			MetricStoreAPI: app.MetricStoreAPI{
				Address:    tc.metricStoreAPIAddress,
				CAPath:     shared.Cert("metric-store-ca.crt"),
				CertPath:   shared.Cert("metric-store.crt"),
				KeyPath:    shared.Cert("metric-store.key"),
				CommonName: "metric-store",
			},
			PKS: app.PKSConfig{
				API:                "https://localhost:" + tc.pksSpy.Port(),
				CAPath:             tc.caCert,
				CommonName:         "metric-store", // api.pks.manila.cf-app.com
				InsecureSkipVerify: false,
			},
			UAA: app.UAAConfig{
				Addr:         "https://localhost:" + tc.uaaSpy.Port(),
				CAPath:       tc.caCert,
				Client:       "some-client",
				ClientSecret: "some-secret",
			},
		}
		tc.app = app.NewClusterDiscoveryApp(cfg, logger.NewTestLogger(GinkgoWriter))
		go tc.app.Run()
		time.Sleep(5 * time.Second)
		shared.WaitForHealthCheck(strconv.Itoa(tc.healthPort), tc.tlsConfig)

	}

	var stopProcess = func(tc *testContext) {
		tc.app.Stop()
	}

	type WithTestContextOption func(*testContext)

	var setup = func(numNodes int, opts ...WithTestContextOption) (*testContext, func()) {
		tc := &testContext{
			caCert:     shared.Cert("metric-store-ca.crt"),
			cert:       shared.Cert("metric-store.crt"),
			key:        shared.Cert("metric-store.key"),
			healthPort: shared.GetFreePort(),
		}

		var err error
		tc.tlsConfig, err = sharedtls.NewMutualTLSClientConfig(tc.caCert, tc.cert, tc.key, "metric-store")
		if err != nil {
			fmt.Printf("ERROR: invalid mutal TLS config: %s\n", err)
		}
		k8sTlsConfig, err := sharedtls.NewTLSServerConfig(shared.Cert("localhost.crt"), shared.Cert("localhost.key"))
		if err != nil {
			panic(err)
		}
		tc.kubeAPISpy = testing.NewK8sSpy(k8sTlsConfig)
		tc.kubeAPISpy.Start()

		caCertificate, err := ioutil.ReadFile(shared.Cert("metric-store-ca.crt"))

		if err != nil {
			panic(err)
		}
		kubeAPIHost, kubeAPIPort := tc.kubeAPISpy.ConnectionInfo()
		tc.pksSpy = testing.NewPKSSpy(kubeAPIHost, kubeAPIPort, string(caCertificate), tc.tlsConfig)
		tc.pksSpy.Start()

		tc.uaaSpy = testing.NewSpyUAA(tc.tlsConfig)
		tc.uaaSpy.Start()
		startProcess(tc)

		apiTLSConfig, err := sharedtls.NewMutualTLSServerConfig(tc.caCert, tc.cert, tc.key)
		if err != nil {
			fmt.Printf("ERROR: invalid mutal TLS config: %s\n", err)
		}
		metricStoreAPI := testing.NewSpyMetricStore(apiTLSConfig)
		tc.metricStoreAPIAddress = metricStoreAPI.Start().EgressAddr

		return tc, func() {
			stopProcess(tc)
			os.RemoveAll(tc.storagePath)
			tc.kubeAPISpy.Stop()
			tc.pksSpy.Stop()
			tc.uaaSpy.Stop()
			metricStoreAPI.Stop()
		}
	}

	Context("cluster discovery", func() {
		It("creates scrape configs for available clusters", func() {
			tc, cleanup := setup(1)
			defer cleanup()

			// TODO just waiting for file creation
			println("scrape config path test", tc.scrapeConfigPath)

			getFileContents := func() string {
				println("Checking scape path ", tc.scrapeConfigPath)
				_, err := os.Stat(tc.scrapeConfigPath)
				if os.IsNotExist(err) {
					return ""
				}

				fileContents, err := ioutil.ReadFile(tc.scrapeConfigPath)
				Expect(err).ToNot(HaveOccurred())
				return string(fileContents)
			}
			Eventually(getFileContents, 3).Should(ContainSubstring("cluster1"))

			configsString, err := ioutil.ReadFile(tc.scrapeConfigPath)

			var configs []*prometheusConfig.ScrapeConfig
			err = yaml.NewDecoder(bytes.NewReader(configsString)).Decode(&configs)
			Expect(err).ToNot(HaveOccurred())

			Expect(configs).To(HaveLen(6))
			c := configs[0]
			println(fmt.Sprintf("config %+v", c))
			Expect(c.JobName).To(ContainSubstring("cluster1"))
		})
	})

})
