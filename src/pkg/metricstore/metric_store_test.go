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

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/cloudfoundry/metric-store-release/src/pkg/leanstreams"
	"github.com/cloudfoundry/metric-store-release/src/pkg/metricstore"
	"github.com/cloudfoundry/metric-store-release/src/pkg/persistence"
	rpc "github.com/cloudfoundry/metric-store-release/src/pkg/rpc/metricstore_v1"
	metrictls "github.com/cloudfoundry/metric-store-release/src/pkg/tls"
	"github.com/gogo/protobuf/proto"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/prometheus/pkg/labels"
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
	tlsConfig        *tls.Config
	peer             *testing.SpyMetricStore
	store            *metricstore.MetricStore
	diskFreeReporter *spyDiskFreeReporter

	spyMetrics                *testing.SpyMetrics
	spyPersistentStoreMetrics *testing.SpyMetrics
	registry                  *prometheus.Registry
}

type testConfig struct {
	RetentionPeriod time.Duration
}
type withSetupOption func(*testConfig)

func withLongRetentionPolicy(config *testConfig) {
	config.RetentionPeriod = time.Hour * 24
}

var _ = Describe("MetricStore", func() {
	var setupWithPersistentStore = func(persistentStore metricstore.PersistentStore, opts ...withSetupOption) (tc *testContext, cleanup func()) {
		tc = &testContext{}
		config := &testConfig{
			RetentionPeriod: time.Hour,
		}
		for _, opt := range opts {
			opt(config)
		}

		var err error
		tc.tlsConfig, err = metrictls.NewMutualTLSConfig(
			testing.Cert("metric-store-ca.crt"),
			testing.Cert("metric-store.crt"),
			testing.Cert("metric-store.key"),
			"metric-store",
		)
		Expect(err).ToNot(HaveOccurred())

		tc.peer = testing.NewSpyMetricStore(tc.tlsConfig)
		tc.spyMetrics = testing.NewSpyMetrics()
		tc.diskFreeReporter = newSpyDiskFreeReporter()

		tc.store = metricstore.New(
			persistentStore,
			tc.diskFreeReporter.Get,
			metricstore.WithAddr("127.0.0.1:0"),
			metricstore.WithIngressAddr("127.0.0.1:0"),
			metricstore.WithMetrics(tc.spyMetrics),
			metricstore.WithServerOpts(
				grpc.Creds(credentials.NewTLS(tc.tlsConfig)),
			),
			metricstore.WithRetentionConfig(metricstore.RetentionConfig{
				RetentionPeriod:       config.RetentionPeriod,
				ExpiryFrequency:       250 * time.Millisecond,
				DiskFreePercentTarget: 50,
			}),
			metricstore.WithLogger(log.New(GinkgoWriter, "", 0)),
		)
		tc.store.Start()

		return tc, func() {
			tc.store.Close()
		}
	}

	var setup = func(opts ...withSetupOption) (tc *testContext, cleanup func()) {
		storagePath, err := ioutil.TempDir("", storagePathPrefix)
		if err != nil {
			panic(err)
		}

		tsStore, err := persistence.OpenTsStore(storagePath)
		Expect(err).ToNot(HaveOccurred())

		spyPersistentStoreMetrics := testing.NewSpyMetrics()
		persistentStore := persistence.NewStore(tsStore, spyPersistentStoreMetrics)

		tc, innerCleanup := setupWithPersistentStore(persistentStore, opts...)
		tc.spyPersistentStoreMetrics = spyPersistentStoreMetrics

		return tc, func() {
			innerCleanup()
			os.RemoveAll(storagePath)
		}
	}

	It("queries data via PromQL Instant Queries", func() {
		tc, cleanup := setup()
		defer cleanup()

		now := time.Now()
		writePoints(tc.store.IngressAddr(), []*rpc.Point{
			{
				Timestamp: now.Add(-2 * time.Second).UnixNano(),
				Name:      MEASUREMENT_NAME,
				Value:     99,
				Labels:    map[string]string{"source_id": "source-id"},
			},
		})

		conn, err := grpc.Dial(tc.store.Addr(),
			grpc.WithTransportCredentials(credentials.NewTLS(tc.tlsConfig)),
		)
		Expect(err).ToNot(HaveOccurred())
		defer conn.Close()
		client := rpc.NewPromQLAPIClient(conn)

		f := func() error {
			resp, err := client.InstantQuery(context.Background(), &rpc.PromQL_InstantQueryRequest{
				Query: fmt.Sprintf(`%s{source_id="%s"}`, MEASUREMENT_NAME, "source-id"),
				Time:  testing.FormatTimeWithDecimalMillis(now),
			})
			if err != nil {
				return err
			}

			if len(resp.GetVector().GetSamples()) != 1 {
				return errors.New("expected 1 samples")
			}

			return nil
		}
		Eventually(f).Should(BeNil())
	})

	It("queries data via PromQL Range Queries", func() {
		tc, cleanup := setup()
		defer cleanup()

		now := time.Now()
		writePoints(tc.store.IngressAddr(), []*rpc.Point{
			{
				Timestamp: now.Add(-2 * time.Second).UnixNano(),
				Name:      MEASUREMENT_NAME,
				Value:     99,
				Labels:    map[string]string{"source_id": "source-id"},
			},
		})

		conn, err := grpc.Dial(tc.store.Addr(),
			grpc.WithTransportCredentials(credentials.NewTLS(tc.tlsConfig)),
		)
		Expect(err).ToNot(HaveOccurred())
		defer conn.Close()
		client := rpc.NewPromQLAPIClient(conn)

		f := func() error {
			resp, err := client.RangeQuery(context.Background(), &rpc.PromQL_RangeQueryRequest{
				Query: fmt.Sprintf(`%s{source_id="%s"}`, MEASUREMENT_NAME, "source-id"),
				Start: testing.FormatTimeWithDecimalMillis(now.Add(-time.Minute)),
				End:   testing.FormatTimeWithDecimalMillis(now),
				Step:  "1m",
			})
			if err != nil {
				return err
			}

			Expect(len(resp.GetMatrix().GetSeries())).To(Equal(1))
			Expect(len(resp.GetMatrix().GetSeries()[0].GetPoints())).To(Equal(1))

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

			insecureConn, err := grpc.Dial(
				tc.store.Addr(),
				grpc.WithTransportCredentials(
					credentials.NewTLS(clientTlsConfig),
				),
			)
			Expect(err).NotTo(HaveOccurred())

			insecureClient := rpc.NewPromQLAPIClient(insecureConn)
			req := &rpc.PromQL_InstantQueryRequest{Query: "1+1"}
			_, err = insecureClient.InstantQuery(context.Background(), req)

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

			insecureConn, err := grpc.Dial(
				tc.store.Addr(),
				grpc.WithTransportCredentials(
					credentials.NewTLS(clientTlsConfig),
				),
			)
			Expect(err).NotTo(HaveOccurred())

			insecureClient := rpc.NewPromQLAPIClient(insecureConn)
			req := &rpc.PromQL_InstantQueryRequest{Query: "1+1"}
			_, err = insecureClient.InstantQuery(context.Background(), req)

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

	Context("periodic expiry", func() {
		It("expires old data on the configured schedule", func() {
			mockPersistentStore := newMockPersistentStore()
			_, cleanup := setupWithPersistentStore(mockPersistentStore)
			defer cleanup()

			Eventually(mockPersistentStore.deleteOlderThanCalls, .5).Should(
				Receive(BeTemporally("~", time.Now().Add(-time.Hour), time.Minute)),
			)

			Eventually(mockPersistentStore.deleteOlderThanCalls, 1.1).Should(Receive())
		})

		It("expires old data while the disk free space target is unmet", func() {
			mockPersistentStore := newMockPersistentStore()
			tc, cleanup := setupWithPersistentStore(mockPersistentStore)
			defer cleanup()

			Consistently(mockPersistentStore.deleteOldestCalls, 1.1).ShouldNot(Receive())

			tc.diskFreeReporter.Add(0)
			tc.diskFreeReporter.Add(0)
			Eventually(mockPersistentStore.deleteOldestCalls, 1.1).Should(HaveLen(2))
			Consistently(mockPersistentStore.deleteOldestCalls, 1.1).Should(HaveLen(2))
		})
	})
})

func writePoints(ingressAddr string, testPoints []*rpc.Point) {
	cfg := &leanstreams.TCPConnConfig{
		MaxMessageSize: 65536,
		Address:        ingressAddr,
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

type mockPersistentStore struct {
	deleteOlderThanCalls chan time.Time
	deleteOldestCalls    chan struct{}
}

func newMockPersistentStore() *mockPersistentStore {
	return &mockPersistentStore{
		deleteOlderThanCalls: make(chan time.Time, 10),
		deleteOldestCalls:    make(chan struct{}, 10),
	}
}

func (m *mockPersistentStore) Put(points []*rpc.Point) {
	panic("not implemented")
}

func (m *mockPersistentStore) Get(*storage.SelectParams, ...*labels.Matcher) (storage.SeriesSet, error) {
	panic("not implemented")
}

func (m *mockPersistentStore) DeleteOlderThan(cutoff time.Time) {
	m.deleteOlderThanCalls <- cutoff
}

func (m *mockPersistentStore) DeleteOldest() {
	m.deleteOldestCalls <- struct{}{}
}

func (m *mockPersistentStore) Close() {
	panic("not implemented")
}

func (m *mockPersistentStore) Labels() (*rpc.PromQL_LabelsQueryResult, error) {
	panic("not implemented")
}

func (m *mockPersistentStore) LabelValues(*rpc.PromQL_LabelValuesQueryRequest) (*rpc.PromQL_LabelValuesQueryResult, error) {
	panic("not implemented")
}

type spyDiskFreeReporter struct {
	diskFreePercentages chan float64
}

func newSpyDiskFreeReporter() *spyDiskFreeReporter {
	return &spyDiskFreeReporter{
		diskFreePercentages: make(chan float64, 10),
	}
}

func (s *spyDiskFreeReporter) Get() float64 {
	select {
	case v := <-s.diskFreePercentages:
		return v
	default:
		return 100
	}
}

func (s *spyDiskFreeReporter) Add(v float64) {
	s.diskFreePercentages <- v
}
