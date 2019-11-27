package metricstore_test

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	shared_api "github.com/cloudfoundry/metric-store-release/src/internal/api"
	"github.com/cloudfoundry/metric-store-release/src/internal/logger"
	"github.com/cloudfoundry/metric-store-release/src/internal/metricstore"
	"github.com/cloudfoundry/metric-store-release/src/pkg/leanstreams"
	"github.com/cloudfoundry/metric-store-release/src/pkg/persistence"
	"github.com/cloudfoundry/metric-store-release/src/pkg/rpc"
	sharedtls "github.com/cloudfoundry/metric-store-release/src/pkg/tls"
	"github.com/influxdata/influxql"
	"github.com/niubaoshu/gotiny"
	prom_api_client "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/client_golang/prometheus"
	config_util "github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/pkg/labels"
	"github.com/prometheus/prometheus/storage"

	"github.com/cloudfoundry/metric-store-release/src/internal/testing"
	shared "github.com/cloudfoundry/metric-store-release/src/internal/testing"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

const (
	storagePathPrefix           = "metric-store"
	MAGIC_MEASUREMENT_NAME      = "cpu"
	MAGIC_MEASUREMENT_PEER_NAME = "memory"
)

type testContext struct {
	tlsConfig       *tls.Config
	egressTLSConfig *config_util.TLSConfig
	peer            *testing.SpyMetricStore
	store           *metricstore.MetricStore
	persistentStore storage.Storage
	apiClient       prom_api_client.API

	spyMetrics                *shared.SpyMetricRegistrar
	spyPersistentStoreMetrics *shared.SpyMetricRegistrar
	registry                  *prometheus.Registry

	minTimeInMilliseconds int64
	maxTimeInMilliseconds int64
}

var _ = Describe("MetricStore", func() {
	var setupWithPersistentStore = func(persistentStore storage.Storage, storagePath string) (tc *testContext, cleanup func()) {
		tc = &testContext{
			minTimeInMilliseconds: influxql.MinTime / int64(time.Millisecond),
			maxTimeInMilliseconds: influxql.MaxTime / int64(time.Millisecond),
		}

		var err error
		tc.tlsConfig, err = sharedtls.NewMutualTLSConfig(
			shared.Cert("metric-store-ca.crt"),
			shared.Cert("metric-store.crt"),
			shared.Cert("metric-store.key"),
			metricstore.COMMON_NAME,
		)
		Expect(err).ToNot(HaveOccurred())

		tc.egressTLSConfig = &config_util.TLSConfig{
			CAFile:     shared.Cert("metric-store-ca.crt"),
			CertFile:   shared.Cert("metric-store.crt"),
			KeyFile:    shared.Cert("metric-store.key"),
			ServerName: metricstore.COMMON_NAME,
		}

		tc.peer = testing.NewSpyMetricStore(tc.tlsConfig)
		peerAddrs := tc.peer.Start()
		tc.spyMetrics = shared.NewSpyMetricRegistrar()
		tc.persistentStore = persistentStore

		tc.store = metricstore.New(
			persistentStore,
			storagePath,
			tc.tlsConfig,
			tc.tlsConfig,
			tc.egressTLSConfig,
			metricstore.WithAddr("127.0.0.1:0"),
			metricstore.WithIngressAddr("127.0.0.1:0"),
			metricstore.WithInternodeAddr("127.0.0.1:0"),
			metricstore.WithClustered(
				0,
				[]string{"my-addr", peerAddrs.EgressAddr},
				[]string{"my-addr", peerAddrs.InternodeAddr},
			),
			metricstore.WithMetrics(tc.spyMetrics),
			metricstore.WithLogger(logger.NewTestLogger()),
		)
		tc.store.Start()

		return tc, func() {
			tc.store.Close()
			tc.peer.Stop()
		}
	}

	var createAPIClient = func(addr string, tlsConfig *tls.Config) prom_api_client.API {
		client, err := shared_api.NewPromHTTPClient(
			addr,
			"",
			tlsConfig,
		)
		Expect(err).NotTo(HaveOccurred())
		return client
	}

	var setup = func() (tc *testContext, cleanup func()) {
		storagePath, err := ioutil.TempDir("", storagePathPrefix)
		if err != nil {
			panic(err)
		}

		spyPersistentStoreMetrics := shared.NewSpyMetricRegistrar()
		persistentStore := persistence.NewStore(
			storagePath,
			spyPersistentStoreMetrics,
		)

		tc, innerCleanup := setupWithPersistentStore(persistentStore, storagePath)
		tc.spyPersistentStoreMetrics = spyPersistentStoreMetrics

		tc.apiClient = createAPIClient(tc.store.Addr(), tc.tlsConfig)

		return tc, func() {
			innerCleanup()
			os.RemoveAll(storagePath)
		}
	}

	It("queries data via PromQL Instant Queries", func() {
		tc, cleanup := setup()
		defer cleanup()

		now := time.Now()
		writePoints(tc, []*rpc.Point{
			{
				Timestamp: now.Add(-2 * time.Second).UnixNano(),
				Name:      MAGIC_MEASUREMENT_NAME,
				Value:     99,
				Labels:    map[string]string{"source_id": "source-id"},
			},
		})

		f := func() error {
			value, _, err := tc.apiClient.Query(
				context.Background(),
				fmt.Sprintf(`%s{source_id="%s"}`, MAGIC_MEASUREMENT_NAME, "source-id"),
				now,
			)
			Expect(err).NotTo(HaveOccurred())

			switch samples := value.(type) {
			case model.Vector:
				if len(samples) != 1 {
					return fmt.Errorf("expected 1 point, got %d", len(samples))
				}
			default:
				return errors.New("expected result to be a model.Vector")
			}

			return nil
		}
		Eventually(f).Should(BeNil())
	})

	It("queries data via PromQL Range Queries", func() {
		tc, cleanup := setup()
		defer cleanup()

		now := time.Now()
		writePoints(tc, []*rpc.Point{
			{
				Timestamp: now.Add(-2 * time.Second).UnixNano(),
				Name:      MAGIC_MEASUREMENT_NAME,
				Value:     99,
				Labels:    map[string]string{"source_id": "source-id"},
			},
		})

		f := func() error {
			value, _, err := tc.apiClient.QueryRange(
				context.Background(),
				fmt.Sprintf(`%s{source_id="%s"}`, MAGIC_MEASUREMENT_NAME, "source-id"),
				prom_api_client.Range{
					Start: now.Add(-time.Minute),
					End:   now,
					Step:  time.Minute,
				},
			)
			Expect(err).NotTo(HaveOccurred())

			switch serieses := value.(type) {
			case model.Matrix:
				if len(serieses) != 1 {
					return fmt.Errorf("expected 1 series, got %d", len(serieses))
				}

				if len(serieses[0].Values) != 1 {
					return fmt.Errorf("expected 1 sample, got %d", len(serieses[0].Values))
				}
			default:
				return errors.New("expected result to be a model.Vector")
			}

			return nil
		}
		Eventually(f).Should(BeNil())
	})

	It("provides a default resolution for sub-queries", func() {
		tc, cleanup := setup()
		defer cleanup()

		now := time.Now()
		writePoints(tc, []*rpc.Point{
			{
				Timestamp: now.Add(-2 * time.Minute).UnixNano(),
				Name:      MAGIC_MEASUREMENT_NAME,
				Value:     250,
				Labels:    map[string]string{"source_id": "source-id"},
			},
			{
				Timestamp: now.Add(-6 * time.Minute).UnixNano(),
				Name:      MAGIC_MEASUREMENT_NAME,
				Value:     99,
				Labels:    map[string]string{"source_id": "source-id"},
			},
		})

		f := func() error {
			value, _, err := tc.apiClient.Query(
				context.Background(),
				fmt.Sprintf(`%s[5m:]`, MAGIC_MEASUREMENT_NAME),
				now,
			)
			Expect(err).NotTo(HaveOccurred())

			switch serieses := value.(type) {
			case model.Matrix:
				if len(serieses) != 1 {
					return fmt.Errorf("expected 1 series, got %d", len(serieses))
				}

				if len(serieses[0].Values) != 5 {
					return fmt.Errorf("expected 5 samples, got %d", len(serieses[0].Values))
				}
			default:
				return errors.New("expected result to be a model.Matrix")
			}

			return nil
		}
		Eventually(f).Should(BeNil())
	})

	It("routes points to internode peers", func() {
		tc, cleanup := setup()
		defer cleanup()

		writePoints(tc, []*rpc.Point{
			{Timestamp: 1, Name: MAGIC_MEASUREMENT_NAME},
			{Timestamp: 2, Name: MAGIC_MEASUREMENT_PEER_NAME},
			{Timestamp: 3, Name: MAGIC_MEASUREMENT_PEER_NAME},
		})

		Eventually(tc.peer.GetInternodePoints).Should(HaveLen(2))
		Expect(tc.peer.GetInternodePoints()[0].Timestamp).To(Equal(int64(2)))
		Expect(tc.peer.GetInternodePoints()[1].Timestamp).To(Equal(int64(3)))
		Expect(tc.peer.GetLocalOnlyValues()).ToNot(ContainElement(false))
	})

	It("replays writes to internode connections when they come back online", func() {
		tc, cleanup := setup()
		defer cleanup()

		tc.peer.Stop()
		writePoints(tc, []*rpc.Point{
			{Timestamp: 1, Name: MAGIC_MEASUREMENT_NAME},
			{Timestamp: 2, Name: MAGIC_MEASUREMENT_PEER_NAME},
			{Timestamp: 3, Name: MAGIC_MEASUREMENT_PEER_NAME},
		})
		tc.peer.Resume()

		Eventually(tc.peer.GetInternodePoints).Should(HaveLen(2))
		Expect(tc.peer.GetInternodePoints()[0].Timestamp).To(Equal(int64(2)))
		Expect(tc.peer.GetInternodePoints()[1].Timestamp).To(Equal(int64(3)))
		Expect(tc.peer.GetLocalOnlyValues()).ToNot(ContainElement(false))
	})

	Describe("Rules API", func() {
		createRulesManagerPayload := []byte(`
			{
				"data": {
					"id": "rules-manager-id"
				}
			}
		`)

		Describe("/rules/manager endpoint", func() {
			It("Creates a rule manager with a generated ID", func() {
				tc, cleanup := setup()
				defer cleanup()
				noIdCreatePayload := []byte(`
					{
						"data": {}
					}
				`)

				apiClient := &http.Client{
					Transport: &http.Transport{TLSClientConfig: tc.tlsConfig},
				}
				resp, err := apiClient.Post("https://"+tc.store.Addr()+"/rules/manager",
					"application/json",
					bytes.NewReader(noIdCreatePayload),
				)
				Expect(err).ToNot(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(201))

				body, err := ioutil.ReadAll(resp.Body)
				Expect(err).ToNot(HaveOccurred())
				var response struct {
					Data struct {
						Id string
					}
				}

				err = json.Unmarshal(body, &response)
				Expect(err).ToNot(HaveOccurred())
				Expect(response.Data.Id).ToNot(Equal(""))

				resp, err = apiClient.Post("https://"+tc.store.Addr()+"/rules/manager",
					"application/json",
					bytes.NewReader(noIdCreatePayload),
				)
				Expect(err).ToNot(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(201))
			})

			It("Creates a rules manager with the provided ID", func() {
				tc, cleanup := setup()
				defer cleanup()
				apiClient := &http.Client{
					Transport: &http.Transport{TLSClientConfig: tc.tlsConfig},
				}
				resp, err := apiClient.Post("https://"+tc.store.Addr()+"/rules/manager",
					"application/json",
					bytes.NewReader(createRulesManagerPayload),
				)
				Expect(err).ToNot(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(201))

				body, err := ioutil.ReadAll(resp.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(body).To(MatchJSON(createRulesManagerPayload))
			})

			It("Returns an error when provided an existing rule manager id", func() {
				tc, cleanup := setup()
				defer cleanup()
				apiClient := &http.Client{
					Transport: &http.Transport{TLSClientConfig: tc.tlsConfig},
				}
				resp, err := apiClient.Post("https://"+tc.store.Addr()+"/rules/manager",
					"application/json",
					bytes.NewReader(createRulesManagerPayload),
				)
				Expect(err).ToNot(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(201))

				resp, err = apiClient.Post("https://"+tc.store.Addr()+"/rules/manager",
					"application/json",
					bytes.NewReader(createRulesManagerPayload),
				)
				Expect(err).ToNot(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(409))
			})

			It("Returns an error when given invalid json", func() {
				tc, cleanup := setup()
				defer cleanup()
				apiClient := &http.Client{
					Transport: &http.Transport{TLSClientConfig: tc.tlsConfig},
				}
				resp, err := apiClient.Post("https://"+tc.store.Addr()+"/rules/manager",
					"application/json",
					bytes.NewReader([]byte("invalid json goes here")),
				)
				Expect(err).ToNot(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(400))
			})
		})

		Describe("/rules/manager/:manager_id/group endpoint", func() {
			// recordedMetric := "job:http_total:sum"
			createRuleGroupPayload := []byte(`
			{
				"data": {
					"name": "my-example-group",
					"interval": "1s",
					"rules": [
						{
							"record": "job:http_total:sum",
							"expr": "sum(http_total) by (source_id)",
							"labels": {
								"foo": "bar"
							}
						}
					]
				}
			}`)

			Context("when a rule manager exists", func() {
				var tc *testContext
				var cleanup func()
				var apiClient *http.Client

				BeforeEach(func() {
					tc, cleanup = setup()

					apiClient = &http.Client{
						Transport: &http.Transport{TLSClientConfig: tc.tlsConfig},
					}

					resp, err := apiClient.Post("https://"+tc.store.Addr()+"/rules/manager",
						"application/json",
						bytes.NewReader(createRulesManagerPayload),
					)

					Expect(err).ToNot(HaveOccurred())
					Expect(resp.StatusCode).To(Equal(201))
				})

				AfterEach(func() {
					cleanup()
				})

				It("Creates a rule group", func() {
					resp, err := apiClient.Post(
						"https://"+tc.store.Addr()+"/rules/manager/rules-manager-id/group",
						"application/json",
						bytes.NewReader(createRuleGroupPayload),
					)
					Expect(err).ToNot(HaveOccurred())
					Expect(resp.StatusCode).To(Equal(201))

					body, err := ioutil.ReadAll(resp.Body)
					Expect(err).ToNot(HaveOccurred())
					Expect(body).To(MatchJSON(createRuleGroupPayload))

					f := func() error {
						rules, err := tc.apiClient.Rules(context.Background())
						Expect(err).ToNot(HaveOccurred())

						if len(rules.Groups) == 0 {
							return errors.New("no rule group")
						}
						return nil
					}
					Eventually(f, 5*time.Second).Should(BeNil())
				})

				It("Returns an error if no name is provided", func() {
					payload := []byte(`
					{
						"data": {
							"interval": "1m",
							"rules": [
								{
									"record": "job:http_total:sum",
									"expr": "sum(http_total) by (source_id)",
								}
							]
						}
					}`)
					resp, err := apiClient.Post(
						"https://"+tc.store.Addr()+"/rules/manager/rules-manager-id/group",
						"application/json",
						bytes.NewReader(payload),
					)
					Expect(err).ToNot(HaveOccurred())
					Expect(resp.StatusCode).To(Equal(400))
				})

				It("Uses the default if no interval is provided", func() {
					payload := []byte(`
					{
						"data": {
							"name": "my-example-group",
							"rules": [
								{
									"record": "job:http_total:sum",
									"expr": "sum(http_total) by (source_id)",
									"labels": {
										"foo": "bar"
									}
								}
							]
						}
					}`)
					resp, err := apiClient.Post(
						"https://"+tc.store.Addr()+"/rules/manager/rules-manager-id/group",
						"application/json",
						bytes.NewReader(payload),
					)
					Expect(err).ToNot(HaveOccurred())
					Expect(resp.StatusCode).To(Equal(201))
					responseBytes, err := ioutil.ReadAll(resp.Body)
					Expect(err).ToNot(HaveOccurred())
					Expect(responseBytes).To(MatchJSON([]byte(`
					{
						"data": {
							"name": "my-example-group",
							"interval": "1m0s",
							"rules": [
								{
									"record": "job:http_total:sum",
									"expr": "sum(http_total) by (source_id)",
									"labels": {
										"foo": "bar"
									}
								}
							]
						}
					}`)))

				})

				It("Returns an error if the provided interval is not a duration", func() {
					payload := []byte(`
					{
						"data": {
							"name": "my-example-group",
							"interval": "not a duration",
							"rules": [
								{
									"record": "job:http_total:sum",
									"expr": "sum(http_total) by (source_id)",
									"labels": {
										"foo": "bar"
									}
								}
							]
						}
					}`)
					resp, err := apiClient.Post(
						"https://"+tc.store.Addr()+"/rules/manager/rules-manager-id/group",
						"application/json",
						bytes.NewReader(payload),
					)
					Expect(err).ToNot(HaveOccurred())
					Expect(resp.StatusCode).To(Equal(400))
				})

				It("Returns an error if the rules array is not provided", func() {
					payload := []byte(`
					{
						"data": {
							"name": "my-example-group",
							"interval": "1m",
						}
					}`)
					resp, err := apiClient.Post(
						"https://"+tc.store.Addr()+"/rules/manager/rules-manager-id/group",
						"application/json",
						bytes.NewReader(payload),
					)
					Expect(err).ToNot(HaveOccurred())
					Expect(resp.StatusCode).To(Equal(400))
				})

				It("Returns an error if the rules array is empty", func() {
					payload := []byte(`
					{
						"data": {
							"name": "my-example-group",
							"interval": "1m",
							"rules": []
						}
					}`)
					resp, err := apiClient.Post(
						"https://"+tc.store.Addr()+"/rules/manager/rules-manager-id/group",
						"application/json",
						bytes.NewReader(payload),
					)
					Expect(err).ToNot(HaveOccurred())
					Expect(resp.StatusCode).To(Equal(400))
				})

				It("Returns an error if the resulting config is not valid", func() {
					payload := []byte(`
					{
						"data": {
							"name": "my-example-group",
							"interval": "1m",
							"rules": [
								{
									"record": "job:http_total:sum",
									"expr": "invalid promql {",
									"labels": {
										"foo": "bar"
									}
								}
							]
						}
					}`)
					resp, err := apiClient.Post(
						"https://"+tc.store.Addr()+"/rules/manager/rules-manager-id/group",
						"application/json",
						bytes.NewReader(payload),
					)
					Expect(err).ToNot(HaveOccurred())
					Expect(resp.StatusCode).To(Equal(400))
				})
			})

			It("Returns an error if the manager_id does not exist", func() {
				tc, cleanup := setup()
				defer cleanup()

				apiClient := &http.Client{
					Transport: &http.Transport{TLSClientConfig: tc.tlsConfig},
				}
				resp, err := apiClient.Post(
					"https://"+tc.store.Addr()+"/rules/manager/rules-manager-that-doesnt-exist/group",
					"application/json",
					bytes.NewReader(createRuleGroupPayload),
				)
				Expect(err).ToNot(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(400))
			})
		})
	})

	Describe("TLS security", func() {
		DescribeTable("allows only supported TLS versions", func(clientTLSVersion int, serverAllows bool) {
			tc, cleanup := setup()
			defer cleanup()

			clientTlsConfig := tc.tlsConfig.Clone()
			clientTlsConfig.MaxVersion = uint16(clientTLSVersion)
			clientTlsConfig.CipherSuites = []uint16{tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384}

			insecureApiClient := createAPIClient(tc.store.Addr(), clientTlsConfig)
			_, _, err := insecureApiClient.Query(context.Background(), "1+1", time.Now())

			if serverAllows {
				Expect(err).NotTo(HaveOccurred())
			} else {
				Expect(err).To(HaveOccurred())
			}
		},

			Entry("unsupported SSL 3.0", tls.VersionSSL30, false),
			Entry("unsupported TLS 1.0", tls.VersionTLS10, false),
			Entry("unsupported TLS 1.1", tls.VersionTLS11, false),
			Entry("supported TLS 1.2", tls.VersionTLS12, true),
		)

		DescribeTable("allows only supported cipher suites", func(clientCipherSuite uint16, serverAllows bool) {
			tc, cleanup := setup()
			defer cleanup()

			clientTlsConfig := tc.tlsConfig.Clone()
			clientTlsConfig.MaxVersion = tls.VersionTLS12
			clientTlsConfig.CipherSuites = []uint16{clientCipherSuite}

			insecureApiClient := createAPIClient(tc.store.Addr(), clientTlsConfig)
			_, _, err := insecureApiClient.Query(context.Background(), "1+1", time.Now())

			if serverAllows {
				Expect(err).NotTo(HaveOccurred())
			} else {
				Expect(err).To(HaveOccurred())
			}
		},

			Entry("unsupported cipher RSA_WITH_3DES_EDE_CBC_SHA", tls.TLS_RSA_WITH_3DES_EDE_CBC_SHA, false),
			Entry("unsupported cipher ECDHE_RSA_WITH_3DES_EDE_CBC_SHA", tls.TLS_ECDHE_RSA_WITH_3DES_EDE_CBC_SHA, false),
			Entry("unsupported cipher RSA_WITH_RC4_128_SHA", tls.TLS_RSA_WITH_RC4_128_SHA, false),
			Entry("unsupported cipher RSA_WITH_AES_128_CBC_SHA256", tls.TLS_RSA_WITH_AES_128_CBC_SHA256, false),
			Entry("unsupported cipher ECDHE_ECDSA_WITH_CHACHA20_POLY1305", tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305, false),
			Entry("unsupported cipher ECDHE_ECDSA_WITH_RC4_128_SHA", tls.TLS_ECDHE_ECDSA_WITH_RC4_128_SHA, false),
			Entry("unsupported cipher ECDHE_ECDSA_WITH_AES_128_CBC_SHA", tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA, false),
			Entry("unsupported cipher ECDHE_ECDSA_WITH_AES_256_CBC_SHA", tls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA, false),
			Entry("unsupported cipher ECDHE_ECDSA_WITH_AES_128_CBC_SHA256", tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256, false),
			Entry("unsupported cipher ECDHE_ECDSA_WITH_AES_128_GCM_SHA256", tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256, false),
			Entry("unsupported cipher ECDHE_ECDSA_WITH_AES_256_GCM_SHA384", tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384, false),
			Entry("unsupported cipher ECDHE_RSA_WITH_RC4_128_SHA", tls.TLS_ECDHE_RSA_WITH_RC4_128_SHA, false),
			Entry("unsupported cipher ECDHE_RSA_WITH_AES_128_CBC_SHA256", tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256, false),
			Entry("unsupported cipher ECDHE_RSA_WITH_AES_128_CBC_SHA", tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA, false),
			Entry("unsupported cipher ECDHE_RSA_WITH_AES_256_CBC_SHA", tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA, false),
			Entry("unsupported cipher ECDHE_RSA_WITH_CHACHA20_POLY1305", tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305, false),
			Entry("unsupported cipher RSA_WITH_AES_128_CBC_SHA", tls.TLS_RSA_WITH_AES_128_CBC_SHA, false),
			Entry("unsupported cipher RSA_WITH_AES_128_GCM_SHA256", tls.TLS_RSA_WITH_AES_128_GCM_SHA256, false),
			Entry("unsupported cipher RSA_WITH_AES_256_CBC_SHA", tls.TLS_RSA_WITH_AES_256_CBC_SHA, false),
			Entry("unsupported cipher RSA_WITH_AES_256_GCM_SHA384", tls.TLS_RSA_WITH_AES_256_GCM_SHA384, false),

			Entry("supported cipher ECDHE_RSA_WITH_AES_128_GCM_SHA256", tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256, true),
			Entry("supported cipher ECDHE_RSA_WITH_AES_256_GCM_SHA384", tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384, true),
		)
	})
})

func writePoints(tc *testContext, testPoints []*rpc.Point) {
	ingressAddr := tc.store.IngressAddr()
	localPointCount := 0

	for _, point := range testPoints {
		if point.Name == MAGIC_MEASUREMENT_NAME {
			localPointCount = localPointCount + 1
		}
	}

	cfg := &leanstreams.TCPClientConfig{
		MaxMessageSize: 65536,
		Address:        ingressAddr,
		TLSConfig:      tc.tlsConfig,
	}
	remoteConnection, err := leanstreams.DialTCP(cfg)
	Expect(err).ToNot(HaveOccurred())

	payload := gotiny.Marshal(&rpc.Batch{
		Points: testPoints,
	})

	_, err = remoteConnection.Write(payload)
	Expect(err).ToNot(HaveOccurred())

	querier, _ := tc.persistentStore.Querier(context.TODO(), 0, 0)
	if localPointCount > 0 {
		f := func() error {
			seriesSet, _, err := querier.Select(
				&storage.SelectParams{Start: tc.minTimeInMilliseconds, End: tc.maxTimeInMilliseconds},
				&labels.Matcher{Name: "__name__", Value: MAGIC_MEASUREMENT_NAME, Type: labels.MatchEqual},
			)
			if err != nil {
				return err
			}

			series := shared.ExplodeSeriesSet(seriesSet)
			if len(series) < 1 {
				return errors.New("expected at least 1 series")
			}

			if len(series[0].Points) < localPointCount {
				return errors.New(fmt.Sprintf("expected at least %d points", localPointCount))
			}

			return nil
		}
		Eventually(f).Should(BeNil())
	}
}

type mockPersistentStore struct {
}

func newMockPersistentStore() *mockPersistentStore {
	return &mockPersistentStore{}
}

func (m *mockPersistentStore) Querier(ctx context.Context, mint int64, maxt int64) (storage.Querier, error) {
	return nil, nil
}

func (m *mockPersistentStore) StartTime() (int64, error) {
	panic("not implemented")
}

func (m *mockPersistentStore) Appender() (storage.Appender, error) {
	return nil, nil
}

func (m *mockPersistentStore) Close() error {
	panic("not implemented")
}
