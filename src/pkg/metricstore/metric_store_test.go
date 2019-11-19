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

	"github.com/cloudfoundry/metric-store-release/src/pkg/api"
	"github.com/cloudfoundry/metric-store-release/src/pkg/leanstreams"
	"github.com/cloudfoundry/metric-store-release/src/pkg/logger"
	"github.com/cloudfoundry/metric-store-release/src/pkg/metricstore"
	"github.com/cloudfoundry/metric-store-release/src/pkg/persistence"
	"github.com/cloudfoundry/metric-store-release/src/pkg/rpc"
	sharedtls "github.com/cloudfoundry/metric-store-release/src/pkg/tls"
	"github.com/niubaoshu/gotiny"
	prom_api_client "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/storage"

	"github.com/cloudfoundry/metric-store-release/src/pkg/testing"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

const (
	storagePathPrefix = "metric-store"
	MEASUREMENT_NAME  = "cpu"
)

type testContext struct {
	tlsConfig *tls.Config
	peer      *testing.SpyMetricStore
	store     *metricstore.MetricStore
	apiClient prom_api_client.API

	spyMetrics                *testing.SpyMetricRegistrar
	spyPersistentStoreMetrics *testing.SpyMetricRegistrar
	registry                  *prometheus.Registry
}

func (tc *testContext) writePoints(points []*rpc.Point) {
	cfg := &leanstreams.TCPClientConfig{
		MaxMessageSize: 65536,
		Address:        tc.store.IngressAddr(),
		TLSConfig:      tc.tlsConfig,
	}
	remoteConnection, err := leanstreams.DialTCP(cfg)
	Expect(err).ToNot(HaveOccurred())

	payload := gotiny.Marshal(&rpc.Batch{Points: points})
	Expect(err).ToNot(HaveOccurred())

	_, err = remoteConnection.Write(payload)
	Expect(err).ToNot(HaveOccurred())
}

var _ = Describe("MetricStore", func() {
	var setupWithPersistentStore = func(persistentStore storage.Storage) (tc *testContext, cleanup func()) {
		tc = &testContext{}

		var err error
		tc.tlsConfig, err = sharedtls.NewMutualTLSConfig(
			testing.Cert("metric-store-ca.crt"),
			testing.Cert("metric-store.crt"),
			testing.Cert("metric-store.key"),
			"metric-store",
		)
		Expect(err).ToNot(HaveOccurred())

		tc.peer = testing.NewSpyMetricStore(tc.tlsConfig)
		tc.spyMetrics = testing.NewSpyMetricRegistrar()

		storagePath, err := ioutil.TempDir("", "storage")

		tc.store = metricstore.New(
			persistentStore,
			tc.tlsConfig,
			tc.tlsConfig,
			storagePath,
			metricstore.WithAddr("127.0.0.1:0"),
			metricstore.WithIngressAddr("127.0.0.1:0"),
			metricstore.WithMetrics(tc.spyMetrics),
			metricstore.WithLogger(logger.NewTestLogger()),
		)
		tc.store.Start()

		return tc, func() {
			tc.store.Close()
		}
	}

	var createAPIClient = func(addr string, tlsConfig *tls.Config) prom_api_client.API {
		client, _ := api.NewPromHTTPClient(addr, "", tlsConfig)
		return client
	}

	var setup = func() (tc *testContext, cleanup func()) {
		storagePath, err := ioutil.TempDir("", storagePathPrefix)
		if err != nil {
			panic(err)
		}

		spyPersistentStoreMetrics := testing.NewSpyMetricRegistrar()

		persistentStore := persistence.NewStore(
			storagePath,
			spyPersistentStoreMetrics,
		)

		tc, innerCleanup := setupWithPersistentStore(persistentStore)
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
		tc.writePoints([]*rpc.Point{
			{
				Timestamp: now.Add(-2 * time.Second).UnixNano(),
				Name:      MEASUREMENT_NAME,
				Value:     99,
				Labels:    map[string]string{"source_id": "source-id"},
			},
		})

		f := func() error {
			value, _, err := tc.apiClient.Query(
				context.Background(),
				fmt.Sprintf(`%s{source_id="%s"}`, MEASUREMENT_NAME, "source-id"),
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
		tc.writePoints([]*rpc.Point{
			{
				Timestamp: now.Add(-2 * time.Second).UnixNano(),
				Name:      MEASUREMENT_NAME,
				Value:     99,
				Labels:    map[string]string{"source_id": "source-id"},
			},
		})

		f := func() error {
			value, _, err := tc.apiClient.QueryRange(
				context.Background(),
				fmt.Sprintf(`%s{source_id="%s"}`, MEASUREMENT_NAME, "source-id"),
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
		tc.writePoints([]*rpc.Point{
			{
				Timestamp: now.Add(-2 * time.Minute).UnixNano(),
				Name:      MEASUREMENT_NAME,
				Value:     250,
				Labels:    map[string]string{"source_id": "source-id"},
			},
			{
				Timestamp: now.Add(-6 * time.Minute).UnixNano(),
				Name:      MEASUREMENT_NAME,
				Value:     99,
				Labels:    map[string]string{"source_id": "source-id"},
			},
		})

		f := func() error {
			value, _, err := tc.apiClient.Query(
				context.Background(),
				fmt.Sprintf(`%s[5m:]`, MEASUREMENT_NAME),
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

	Describe("Rules API", func() {
		Describe("/rules/manager endpoint", func() {

			createRulesPayload := []byte(`
				{
					"data": {
						"id": "rules-manager-id"
					}
				}
			`)

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
					bytes.NewReader(createRulesPayload),
				)
				Expect(err).ToNot(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(201))

				body, err := ioutil.ReadAll(resp.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(body).To(MatchJSON(createRulesPayload))
			})

			It("Returns an error when provided an existing rule manager id", func() {
				tc, cleanup := setup()
				defer cleanup()
				apiClient := &http.Client{
					Transport: &http.Transport{TLSClientConfig: tc.tlsConfig},
				}
				resp, err := apiClient.Post("https://"+tc.store.Addr()+"/rules/manager",
					"application/json",
					bytes.NewReader(createRulesPayload),
				)
				Expect(err).ToNot(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(201))

				resp, err = apiClient.Post("https://"+tc.store.Addr()+"/rules/manager",
					"application/json",
					bytes.NewReader(createRulesPayload),
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

type mockPersistentStore struct {
}

func newMockPersistentStore() *mockPersistentStore {
	return &mockPersistentStore{}
}

func (m *mockPersistentStore) Querier(ctx context.Context, mint, maxt int64) (storage.Querier, error) {
	return nil, nil
}

func (m *mockPersistentStore) Appender() (storage.Appender, error) {
	return persistence.NewAppender(
		testing.NewSpyAdapter(),
		testing.NewSpyMetricRegistrar(),
	), nil
}

func (m *mockPersistentStore) StartTime() (int64, error) {
	panic("not implemented")
}

func (m *mockPersistentStore) Close() error {
	panic("not implemented")
}
