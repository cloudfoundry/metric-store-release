package metricstore_test

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"time"

	"github.com/cloudfoundry/metric-store-release/src/pkg/api"
	"github.com/cloudfoundry/metric-store-release/src/pkg/leanstreams"
	"github.com/cloudfoundry/metric-store-release/src/pkg/metricstore"
	"github.com/cloudfoundry/metric-store-release/src/pkg/persistence"
	rpc "github.com/cloudfoundry/metric-store-release/src/pkg/rpc/metricstore_v1"
	sharedtls "github.com/cloudfoundry/metric-store-release/src/pkg/tls"
	"github.com/gogo/protobuf/proto"
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

	spyMetrics                *testing.SpyMetrics
	spyPersistentStoreMetrics *testing.SpyMetrics
	registry                  *prometheus.Registry
}

func (tc *testContext) writePoints(testPoints []*rpc.Point) {
	cfg := &leanstreams.TCPClientConfig{
		MaxMessageSize: 65536,
		Address:        tc.store.IngressAddr(),
		TLSConfig:      tc.tlsConfig,
	}
	remoteConnection, err := leanstreams.DialTCP(cfg)
	Expect(err).ToNot(HaveOccurred())

	payload, err := proto.Marshal(&rpc.SendRequest{
		Batch: &rpc.Points{
			Points: testPoints,
		},
	})
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
		tc.spyMetrics = testing.NewSpyMetrics()

		tc.store = metricstore.New(
			persistentStore,
			tc.tlsConfig,
			tc.tlsConfig,
			metricstore.WithAddr("127.0.0.1:0"),
			metricstore.WithIngressAddr("127.0.0.1:0"),
			metricstore.WithMetrics(tc.spyMetrics),
			metricstore.WithLogger(log.New(GinkgoWriter, "", 0)),
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

		spyPersistentStoreMetrics := testing.NewSpyMetrics()

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

	Describe("instrumentation", func() {
		It("updates ingress metrics", func() {
			mockPersistentStore := newMockPersistentStore()
			tc, cleanup := setupWithPersistentStore(mockPersistentStore)
			defer cleanup()

			tc.writePoints([]*rpc.Point{
				{
					Timestamp: time.Now().UnixNano(),
					Name:      MEASUREMENT_NAME,
					Value:     99,
					Labels:    map[string]string{"source_id": "source-id"},
				},
			})

			Eventually(func() float64 {
				return tc.spyMetrics.Get("metric_store_ingress")
			}).Should(BeEquivalentTo(1))
		})
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
	), nil
}

func (m *mockPersistentStore) StartTime() (int64, error) {
	panic("not implemented")
}

func (m *mockPersistentStore) Close() error {
	panic("not implemented")
}
