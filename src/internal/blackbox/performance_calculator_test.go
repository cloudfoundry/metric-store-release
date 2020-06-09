package blackbox_test

import (
	"context"
	"fmt"
	"sync"
	"time"

	prom_api_client "github.com/prometheus/client_golang/api"
	prom_http_client "github.com/prometheus/client_golang/api"
	"github.com/prometheus/common/model"

	"github.com/cloudfoundry/metric-store-release/src/internal/blackbox"
	"github.com/cloudfoundry/metric-store-release/src/internal/metric-store"
	"github.com/cloudfoundry/metric-store-release/src/internal/testing"
	shared "github.com/cloudfoundry/metric-store-release/src/internal/testing"
	"github.com/cloudfoundry/metric-store-release/src/pkg/ingressclient"
	"github.com/cloudfoundry/metric-store-release/src/pkg/logger"
	"github.com/cloudfoundry/metric-store-release/src/pkg/rpc"
	sharedtls "github.com/cloudfoundry/metric-store-release/src/internal/tls"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
)

var _ = Describe("Performance Calculator", func() {
	Describe("Calculate()", func() {
		It("calculates query benchmark metrics", func() {
			client := &mockPerfClient{}
			sourceID := "someSourceId"
			calc := blackbox.NewPerformanceCalculator(&blackbox.Config{SourceId: sourceID}, nil, nil)
			pMetrics, err := calc.Calculate(client)

			Expect(err).ToNot(HaveOccurred())

			Expect(client.query).To(Equal(fmt.Sprintf(`sum(count_over_time(%s{source_id="%s"}[1w]))`, blackbox.BlackboxPerformanceTestCanary, sourceID)))
			Expect(int64(pMetrics.Latency)).Should(BeNumerically(">=", int64(10*time.Millisecond))) // TODO best way to validate latency?
			Expect(pMetrics.Magnitude).To(Equal(750000))

		})

		It("reports query errors", func() {
			client := &mockUnresponsiveClient{}
			sourceID := "someSourceId"
			calc := blackbox.NewPerformanceCalculator(&blackbox.Config{SourceId: sourceID}, nil, nil)
			_, err := calc.Calculate(client)

			Expect(err).To(HaveOccurred())
		})
	})

	It("emits performance test metric at the configured interval", func() {
		tc := setup()
		defer tc.teardown()

		calc := blackbox.NewPerformanceCalculator(nil, logger.NewNop(), nil)
		emissionInterval := 10 * time.Millisecond
		expectedEmissionCount := int(tc.testDuration / emissionInterval)

		tc.waitGroup.Add(1)

		go func() {
			calc.EmitPerformanceTestMetrics("source-1", emissionInterval, tc.client, tc.stop)
			tc.waitGroup.Done()
		}()

		var points []*rpc.Point
		Eventually(func() int {
			points = tc.metricStore.GetPoints()
			return len(points)
		}, tc.testDuration+tc.timingFudgeFactor).Should(BeNumerically(">=", expectedEmissionCount))

		close(tc.stop)
		tc.waitGroup.Wait()

		Expect(points[0]).To(PointTo(MatchFields(IgnoreExtras, Fields{
			"Name":  Equal("blackbox_performance_canary"),
			"Value": Equal(10.0),
			"Labels": MatchKeys(IgnoreExtras, Keys{
				"source_id": Equal("source-1"),
			}),
		})))
	})
})

type mockPerfClient struct {
	query string
}

func (c *mockPerfClient) Query(ctx context.Context, query string, ts time.Time) (model.Value, prom_api_client.Warnings, error) {
	c.query = query

	value := model.Vector{
		&model.Sample{
			Metric:    model.Metric{},
			Value:     750000,
			Timestamp: model.Now(),
		}}

	time.Sleep(10 * time.Millisecond)
	return value, nil, nil
}

func (c *mockPerfClient) LabelValues(context.Context, string) (model.LabelValues, prom_http_client.Warnings, error) {
	return nil, nil, fmt.Errorf("unexpected status code 500")
}

func setup() testContext {
	tlsServerConfig, _ := sharedtls.NewMutualTLSServerConfig(
		shared.Cert("metric-store-ca.crt"),
		shared.Cert("metric-store.crt"),
		shared.Cert("metric-store.key"),
	)
	ms := testing.NewSpyMetricStore(
		tlsServerConfig,
	)

	tlsClientConfig, _ := sharedtls.NewMutualTLSClientConfig(
		shared.Cert("metric-store-ca.crt"),
		shared.Cert("metric-store.crt"),
		shared.Cert("metric-store.key"),
		metric_store.COMMON_NAME,
	)
	addrs := ms.Start()
	client, err := ingressclient.NewIngressClient(addrs.IngressAddr, tlsClientConfig)
	Expect(err).ToNot(HaveOccurred())

	time.Sleep(10 * time.Millisecond)

	return testContext{
		client:            client,
		testDuration:      100 * time.Millisecond,
		timingFudgeFactor: 1 * time.Second,
		metricStore:       ms,

		stop:      make(chan bool),
		waitGroup: &sync.WaitGroup{},
	}
}

func (tc testContext) teardown() {
	tc.metricStore.Stop()
}

type testContext struct {
	client            *ingressclient.IngressClient
	testDuration      time.Duration
	timingFudgeFactor time.Duration

	metricStore *testing.SpyMetricStore

	stop      chan bool
	waitGroup *sync.WaitGroup
}
