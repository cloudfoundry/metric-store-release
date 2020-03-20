package metricstore_test

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/gob"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/cloudfoundry/metric-store-release/src/pkg/rulesclient"

	shared_api "github.com/cloudfoundry/metric-store-release/src/internal/api"
	"github.com/cloudfoundry/metric-store-release/src/internal/metricstore"
	sharedtls "github.com/cloudfoundry/metric-store-release/src/internal/tls"
	"github.com/cloudfoundry/metric-store-release/src/pkg/leanstreams"
	"github.com/cloudfoundry/metric-store-release/src/pkg/logger"
	"github.com/cloudfoundry/metric-store-release/src/pkg/persistence"
	"github.com/cloudfoundry/metric-store-release/src/pkg/rpc"
	"github.com/influxdata/influxql"
	prom_api_client "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/client_golang/prometheus"
	config_util "github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	prom_config "github.com/prometheus/prometheus/config"
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
	rulesClient     *rulesclient.RulesClient
	rulesApiClient  *http.Client

	spyMetrics                *shared.SpyMetricRegistrar
	spyPersistentStoreMetrics *shared.SpyMetricRegistrar
	registry                  *prometheus.Registry

	alertManager1         *testing.AlertManagerSpy
	alertManager2         *testing.AlertManagerSpy
	minTimeInMilliseconds int64
	maxTimeInMilliseconds int64
}

func (tc *testContext) CreateRuleManager(managerId string, alertManagers *prom_config.AlertmanagerConfigs) {
	_, err := tc.rulesClient.CreateManager(managerId, alertManagers)
	Expect(err).ToNot(HaveOccurred())
}

func (tc *testContext) CreateRuleGroup(managerId, alertName, alertExpr string) (*rulesclient.RuleGroup, error) {
	group := rulesclient.RuleGroup{
		Name:     "my-example-group",
		Interval: rulesclient.Duration(time.Minute),
		Rules: []rulesclient.Rule{
			{
				Record: alertName,
				Expr:   alertExpr,
			},
		},
	}
	return tc.rulesClient.UpsertRuleGroup(managerId, group)
}

func (tc *testContext) CreateAlertGroup(managerId, alertName, alertExpr string) {
	group := rulesclient.RuleGroup{
		Name:     "my-example-group",
		Interval: rulesclient.Duration(time.Second),
		Rules: []rulesclient.Rule{
			{
				Alert: alertName,
				Expr:  alertExpr,
			},
		},
	}

	_, err := tc.rulesClient.UpsertRuleGroup(managerId, group)
	Expect(err).ToNot(HaveOccurred())
}

var _ = Describe("MetricStore", func() {
	var setupWithPersistentStore = func(persistentStore storage.Storage, storagePath string) (tc *testContext, cleanup func()) {
		tc = &testContext{
			minTimeInMilliseconds: influxql.MinTime / int64(time.Millisecond),
			maxTimeInMilliseconds: influxql.MaxTime / int64(time.Millisecond),
		}

		var err error
		tlsServerConfig, err := sharedtls.NewMutualTLSServerConfig(
			shared.Cert("metric-store-ca.crt"),
			shared.Cert("metric-store.crt"),
			shared.Cert("metric-store.key"),
		)
		Expect(err).ToNot(HaveOccurred())

		tc.tlsConfig, err = sharedtls.NewMutualTLSClientConfig(
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

		tc.peer = testing.NewSpyMetricStore(tlsServerConfig)
		peerAddrs := tc.peer.Start()
		tc.spyMetrics = shared.NewSpyMetricRegistrar()
		tc.persistentStore = persistentStore

		tc.store = metricstore.New(
			persistentStore,
			storagePath,
			tlsServerConfig,
			tlsServerConfig,
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
			metricstore.WithLogger(logger.NewTestLogger(GinkgoWriter)),
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

		tc.alertManager1 = testing.NewAlertManagerSpy(tc.tlsConfig)
		tc.alertManager2 = testing.NewAlertManagerSpy(tc.tlsConfig)
		tc.alertManager1.Start()
		tc.alertManager2.Start()
		tc.apiClient = createAPIClient(tc.store.Addr(), tc.tlsConfig)

		tc.rulesApiClient = &http.Client{
			Transport: &http.Transport{TLSClientConfig: tc.tlsConfig},
		}

		tc.rulesClient = rulesclient.NewRulesClient(tc.store.Addr(), tc.tlsConfig,
			rulesclient.WithRulesClientLogger(logger.NewTestLogger(GinkgoWriter)))

		return tc, func() {
			innerCleanup()
			os.RemoveAll(storagePath)
			tc.alertManager1.Stop()
			tc.alertManager2.Stop()
		}
	}

	var tc *testContext
	var cleanup func()

	BeforeEach(func() {
		tc, cleanup = setup()
	})

	AfterEach(func() { cleanup() })

	It("queries data via PromQL Instant Queries", func() {
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
		now := time.Now()
		writePoints(tc, []*rpc.Point{
			{Timestamp: now.UnixNano(), Name: MAGIC_MEASUREMENT_NAME},
			{Timestamp: now.Add(time.Second).UnixNano(), Name: MAGIC_MEASUREMENT_PEER_NAME},
			{Timestamp: now.Add(2 * time.Second).UnixNano(), Name: MAGIC_MEASUREMENT_PEER_NAME},
		})

		Eventually(tc.peer.GetInternodePoints).Should(HaveLen(2))
		Expect(tc.peer.GetInternodePoints()[0].Timestamp).To(Equal(now.Add(time.Second).UnixNano()))
		Expect(tc.peer.GetInternodePoints()[1].Timestamp).To(Equal(now.Add(2 * time.Second).UnixNano()))
		Expect(tc.peer.GetLocalOnlyValues()).ToNot(ContainElement(false))
	})

	It("replays writes to internode connections when they come back online", func() {
		if runtime.GOOS == "darwin" {
			Skip("doesn't work on Mac OS")
		}

		tc.peer.Stop()
		now := time.Now()
		writePoints(tc, []*rpc.Point{
			{Timestamp: now.UnixNano(), Name: MAGIC_MEASUREMENT_NAME},
			{Timestamp: now.Add(time.Second).UnixNano(), Name: MAGIC_MEASUREMENT_PEER_NAME},
			{Timestamp: now.Add(2 * time.Second).UnixNano(), Name: MAGIC_MEASUREMENT_PEER_NAME},
		})
		tc.peer.Resume()

		Eventually(tc.peer.GetInternodePoints).Should(HaveLen(2))
		Expect(tc.peer.GetInternodePoints()[0].Timestamp).To(Equal(now.Add(time.Second).UnixNano()))
		Expect(tc.peer.GetInternodePoints()[1].Timestamp).To(Equal(now.Add(2 * time.Second).UnixNano()))
		Expect(tc.peer.GetLocalOnlyValues()).ToNot(ContainElement(false))
	})

	Describe("Rules API", func() {
		Describe("/rules/manager endpoint", func() {
			It("Creates a rules manager with the provided ID", func() {
				managerConfig, err := tc.rulesClient.CreateManager(MAGIC_MEASUREMENT_NAME, nil)
				Expect(err).ToNot(HaveOccurred())

				Expect(managerConfig.Id()).To(Equal(MAGIC_MEASUREMENT_NAME))
			})
		})

		Describe("/rules/manager/:manager_id/group endpoint", func() {
			createRuleGroupPayload := []byte(`
			{
				"data": {
					"name": "my-example-group",
					"interval": "1m",
					"rules": [
						{
							"record": "job:http_total:sum",
							"expr": "sum(http_total) by (source_id)"
						}
					]
				}
			}`)

			Context("when a rule manager exists", func() {
				BeforeEach(func() {
					tc.CreateRuleManager(MAGIC_MEASUREMENT_NAME, nil)
				})

				It("Creates a rule group", func() {
					group, err := tc.CreateRuleGroup(MAGIC_MEASUREMENT_NAME, "job:http_total:sum", "sum(http_total) by (source_id)")
					Expect(err).ToNot(HaveOccurred())

					Expect(group).To(Equal(&rulesclient.RuleGroup{
						Name:     "my-example-group",
						Interval: rulesclient.Duration(time.Minute),
						Rules: []rulesclient.Rule{
							{Record: "job:http_total:sum", Expr: "sum(http_total) by (source_id)"},
						},
					}))

					Eventually(countRuleGroups(tc), 5*time.Second).Should(BeNumerically(">", 0))
				})

				It("Correctly serializes the duration from the `for` field", func() {
					payload := []byte(`
					{
						"data": {
							"name": "example-group",
							"interval": "1m",
							"rules": [
								{
									"alert": "job:http_total:sum",
									"expr": "sum(http_total) > 1",
									"for": "10s"
								}
							]
						}
					}`)
					resp, err := tc.rulesApiClient.Post(
						"https://"+tc.store.Addr()+"/rules/manager/"+MAGIC_MEASUREMENT_NAME+"/group",
						"application/json",
						bytes.NewReader(payload),
					)
					Expect(err).ToNot(HaveOccurred())
					Expect(resp.StatusCode).To(Equal(201))
				})

				It("Returns an error if the rules array is not provided", func() {
					_, err := tc.rulesClient.UpsertRuleGroup(MAGIC_MEASUREMENT_NAME, rulesclient.RuleGroup{
						Name:     "tragic",
						Interval: rulesclient.Duration(time.Minute),
					})
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("rule"))
				})

				It("Returns an error if the resulting config is not valid", func() {
					_, err := tc.CreateRuleGroup(MAGIC_MEASUREMENT_NAME, "job:http_total:sum", "invalid promql {")
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("parse"))
				})
			})

			It("Returns an error if the manager_id does not exist", func() {
				resp, err := tc.rulesApiClient.Post(
					"https://"+tc.store.Addr()+"/rules/manager/rules-manager-that-isnt/group",
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

	var payload bytes.Buffer
	enc := gob.NewEncoder(&payload)
	err = enc.Encode(rpc.Batch{Points: testPoints})
	if err != nil {
		log.Fatal("gob encode error:", err)
	}

	_, err = remoteConnection.Write(payload.Bytes())
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

func countRuleGroups(tc *testContext) func() int {
	return func() int {
		rules, err := tc.apiClient.Rules(context.Background())
		Expect(err).ToNot(HaveOccurred())

		return len(rules.Groups)
	}
}

func countManagersActive(tc *testContext) func() int {
	return func() int {
		managers, err := tc.apiClient.AlertManagers(context.Background())
		Expect(err).ToNot(HaveOccurred())

		return len(managers.Active)
	}
}

func countFiringAlerts(tc *testContext) func() int {
	return func() int {
		alerts, err := tc.apiClient.Alerts(context.Background())
		Expect(err).ToNot(HaveOccurred())

		count := 0
		for _, alert := range alerts.Alerts {
			if alert.State == prom_api_client.AlertStateFiring {
				count += 1
			}
		}
		return count
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
