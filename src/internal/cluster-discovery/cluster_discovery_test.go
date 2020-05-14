package cluster_discovery_test

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"fmt"
	cluster_discovery "github.com/cloudfoundry/metric-store-release/src/internal/cluster-discovery"
	"github.com/cloudfoundry/metric-store-release/src/internal/testing"
	sharedtls "github.com/cloudfoundry/metric-store-release/src/internal/tls"
	"github.com/cloudfoundry/metric-store-release/src/pkg/logger"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"github.com/onsi/gomega/types"
	prometheusConfig "github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/pkg/relabel"
	"gopkg.in/yaml.v2"
	"net/http"
	"regexp"
	"time"
)

var _ = Describe("Cluster Discovery", func() {
	type testContext struct {
		certificateStore     testing.ScrapeStoreSpy
		certificateClient    testing.MockCSRClient
		metricStoreAPI       *testing.SpyMetricStore
		metricStoreAPIClient *http.Client
	}

	var setup = func() *testContext {
		privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		Expect(err).To(Not(HaveOccurred()))
		tlsConfig, err := sharedtls.NewMutualTLSServerConfig(
			testing.Cert("metric-store-ca.crt"),
			testing.Cert("metric-store.crt"),
			testing.Cert("metric-store.key"),
		)
		if err != nil {
			panic(err)
		}
		tlsClientConfig, err := sharedtls.NewMutualTLSClientConfig(
			testing.Cert("metric-store-ca.crt"),
			testing.Cert("metric-store.crt"),
			testing.Cert("metric-store.key"),
			"metric-store",
		)
		if err != nil {
			panic(err)
		}

		metricStoreAPIClient := &http.Client{
			Transport: &http.Transport{TLSClientConfig: tlsClientConfig},
			Timeout:   10 * time.Second,
		}
		tc := &testContext{
			certificateClient: testing.MockCSRClient{
				Key: privateKey,
			},
			metricStoreAPIClient: metricStoreAPIClient,
			metricStoreAPI:       testing.NewSpyMetricStore(tlsConfig),
		}
		return tc
	}

	var runScrape = func(tc *testContext) []*prometheusConfig.ScrapeConfig {
		mockAuth := &testing.MockAuthClient{}
		discovery := cluster_discovery.New(&tc.certificateStore,
			&testing.MockClusterProvider{CertClient: &tc.certificateClient},
			mockAuth,
			"localhost:8080",
			tc.metricStoreAPIClient,
			cluster_discovery.WithLogger(logger.NewTestLogger(GinkgoWriter)))
		discovery.UpdateScrapeConfig()

		var expected prometheusConfig.Config
		yaml.NewDecoder(bytes.NewReader(tc.certificateStore.ScrapeConfig)).Decode(&expected)
		return expected.ScrapeConfigs
	}

	Describe("Start", func() {
		It("runs repeatedly", func() {
			tc := setup()
			mockAuth := &testing.MockAuthClient{}
			certificateStore := &testing.ScrapeStoreSpy{}
			discovery := cluster_discovery.New(certificateStore,
				&testing.MockClusterProvider{
					CertClient: testing.NewMockCSRClient(),
				},
				mockAuth,
				"localhost:8080",
				tc.metricStoreAPIClient,
				cluster_discovery.WithRefreshInterval(time.Millisecond),
			)

			discovery.Start()

			Eventually(mockAuth.Calls.Load).Should(BeNumerically(">", 1))
		})

		It("stops", func() {
			mockAuth := &testing.MockAuthClient{}
			tc := setup()
			certificateStore := &testing.ScrapeStoreSpy{}
			discovery := cluster_discovery.New(certificateStore,
				&testing.MockClusterProvider{
					CertClient: testing.NewMockCSRClient(),
				},
				mockAuth,
				"localhost:8080",
				tc.metricStoreAPIClient,
				cluster_discovery.WithRefreshInterval(time.Millisecond),
			)

			discovery.Start()
			Eventually(mockAuth.Calls.Load).Should(BeNumerically(">", 1))
			discovery.Stop()
			runsAtStop := mockAuth.Calls.Load()
			Consistently(mockAuth.Calls.Load).Should(BeNumerically("<=", runsAtStop+1))
		})

		It("reloads metric store's configuration", func() {
			tc := setup()
			mockAuth := &testing.MockAuthClient{}
			certificateStore := &testing.ScrapeStoreSpy{}

			discovery := cluster_discovery.New(certificateStore,
				&testing.MockClusterProvider{
					CertClient: testing.NewMockCSRClient(),
				},
				mockAuth,
				tc.metricStoreAPI.Start().EgressAddr,
				tc.metricStoreAPIClient,
				cluster_discovery.WithRefreshInterval(time.Second),
				cluster_discovery.WithLogger(logger.NewTestLogger(GinkgoWriter)),
			)

			discovery.Start()

			Eventually(mockAuth.Calls.Load, 5).Should(BeNumerically(">", 1))
			Expect(tc.metricStoreAPI.ReloadRequestsCount.Load()).To(BeNumerically(">", 0))
		})
	})

	Describe("ScrapeConfig for node in a cluster", func() {
		var yamlValues = func(doc []byte, name string) []string {
			// name: matches the characters value: literally (case sensitive)
			// \s* matches any whitespace character (equal to [\r\n\t\f\v ])
			//   * Quantifier — Matches between zero and unlimited times, as many times as possible, giving back as needed (greedy)
			// "? matches the character " literally (case sensitive)
			//   ? Quantifier — Matches between zero and one times, as many times as possible, giving back as needed (greedy)
			// 1st Capturing Group ([\w\.\/]+)
			//   Match a single character present in the list below [-\w\.\/]+
			//     - matches the character - literally (case sensitive)
			//     : matches the character : literally (case sensitive)
			//     \w matches any word character (equal to [a-zA-Z0-9_])
			//     \. matches the character . literally (case sensitive)
			//     \/ matches the character / literally (case sensitive)
			//     + Quantifier — Matches between one and unlimited times, as many times as possible, giving back as needed (greedy)
			// "? matches the character " literally (case sensitive)
			//   ? Quantifier — Matches between zero and one times, as many times as possible, giving back as needed (greedy)
			pattern := fmt.Sprintf(`%s:\s*"?([-:\w\.\/]+)"?`, name)
			regex, err := regexp.Compile(pattern)
			Expect(err).ToNot(HaveOccurred())

			matches := regex.FindAllSubmatch(doc, -1)
			Expect(len(matches)).To(BeNumerically(">", 0),
				fmt.Sprintf("attribute %s had no occurrences", name))

			var values []string
			for _, match := range matches {
				values = append(values, string(match[1]))
			}
			return values
		}

		var matchAllValues = func(doc []byte, name, expectedValue string) {
			values := yamlValues(doc, name)
			for occurrence, value := range values {
				Expect(value).To(Equal(expectedValue),
					fmt.Sprintf("%s occurrance #%d", name, occurrence))
			}
		}

		Describe("creates a scrape config", func() {
			It("creates a valid config", func() {
				tc := setup()
				unmarshalled := runScrape(tc)
				Expect(unmarshalled).ToNot(BeNil())
			})

			It("populates shared values", func() {
				tc := setup()
				runScrape(tc)

				matchAllValues(tc.certificateStore.ScrapeConfig, "ca_file", "/tmp/scraper/cluster1/ca.pem")
				matchAllValues(tc.certificateStore.ScrapeConfig, "cert_file", "/tmp/scraper/cluster1/cert.pem")
				matchAllValues(tc.certificateStore.ScrapeConfig, "key_file", "/tmp/scraper/private.key")
				matchAllValues(tc.certificateStore.ScrapeConfig, "insecure_skip_verify", "true")
				matchAllValues(tc.certificateStore.ScrapeConfig, "api_server", "https://somehost:12345")
			})

			var findJob = func(jobName string, configs []*prometheusConfig.ScrapeConfig) *prometheusConfig.ScrapeConfig {
				for _, job := range configs {
					if job.JobName == jobName {
						return job
					}
				}
				return nil
			}

			var matchRelabel = func() types.GomegaMatcher {
				return MatchElements(
					func(element interface{}) string {
						relabelConfig := element.(*relabel.Config)
						return string(relabelConfig.TargetLabel) + ":" + string(relabelConfig.Action)
					},
					IgnoreExtras,
					Elements{
						"__address__:replace": PointTo(MatchFields(IgnoreExtras,
							Fields{
								"Replacement": Equal("somehost:12345"),
							})),
					})
			}

			It("creates cluster job", func() {
				tc := setup()
				unmarshalled := runScrape(tc)

				serverJob := findJob("cluster1", unmarshalled)
				Expect(serverJob).ToNot(BeNil())
				Expect(string(serverJob.ServiceDiscoveryConfig.StaticConfigs[0].Targets[0]["__address__"])).
					To(Equal("somehost:12345"))
			})

			It("creates an apiserver job", func() {
				tc := setup()
				jobs := runScrape(tc)
				Expect(findJob("cluster1-kubernetes-apiservers", jobs)).ToNot(BeNil())
			})

			It("creates kubernetes-nodes job", func() {
				tc := setup()
				unmarshalled := runScrape(tc)

				serverJob := findJob("cluster1-kubernetes-nodes", unmarshalled)
				Expect(serverJob).ToNot(BeNil(), "cluster1-kubernetes-nodes cluster1 job doesn't exist")
				Expect(serverJob.RelabelConfigs).To(matchRelabel())
			})

			It("creates cadvisor job", func() {
				tc := setup()
				unmarshalled := runScrape(tc)

				serverJob := findJob("cluster1-kubernetes-cadvisor", unmarshalled)
				Expect(serverJob).ToNot(BeNil())
				Expect(serverJob.RelabelConfigs).To(matchRelabel())
			})

			It("creates kube-state-metrics job", func() {
				tc := setup()
				unmarshalled := runScrape(tc)

				serverJob := findJob("cluster1-kube-state-metrics", unmarshalled)
				Expect(serverJob).ToNot(BeNil())
				Expect(serverJob.RelabelConfigs).To(matchRelabel())
			})

			It("creates kubernetes-coredns job", func() {
				tc := setup()
				unmarshalled := runScrape(tc)

				serverJob := findJob("cluster1-kubernetes-coredns", unmarshalled)
				Expect(serverJob).ToNot(BeNil())
				Expect(serverJob.RelabelConfigs).To(matchRelabel())
			})
		})

		Describe("ScrapeConfig handles errors gracefully", func() {
			It("checks errors when saving the CA", func() {
				tc := setup()
				tc.certificateStore.NextSaveCAIsError = true
				Expect(runScrape(tc)).To(BeEmpty())
			})

			It("checks errors when generating the certificate", func() {
				tc := setup()
				tc.certificateClient.NextGetApprovalIsError = true
				Expect(runScrape(tc)).To(BeEmpty())
			})

			It("checks errors when saving the certificate", func() {
				tc := setup()
				tc.certificateStore.NextSaveCertIsError = true
				Expect(runScrape(tc)).To(BeEmpty())
			})

			It("checks errors when saving the key", func() {
				tc := setup()
				tc.certificateStore.NextSaveKeyIsError = true
				Expect(runScrape(tc)).To(BeEmpty())
			})
		})
	})
})
