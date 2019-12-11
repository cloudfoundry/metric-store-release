package blackbox_test

import (
	"crypto/tls"
	"sync"
	"time"

	"github.com/cloudfoundry/metric-store-release/src/internal/blackbox"
	"github.com/cloudfoundry/metric-store-release/src/internal/logger"
	"github.com/cloudfoundry/metric-store-release/src/internal/metricstore"
	"github.com/cloudfoundry/metric-store-release/src/pkg/ingressclient"
	"github.com/cloudfoundry/metric-store-release/src/pkg/rpc"
	sharedtls "github.com/cloudfoundry/metric-store-release/src/pkg/tls"

	"github.com/cloudfoundry/metric-store-release/src/internal/testing"
	shared "github.com/cloudfoundry/metric-store-release/src/internal/testing"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
)

var _ = Describe("Metric Store© Blackbox™", func() {
	It("emits one test metric per emission interval per MagicMetricName", func() {
		tc := setup()
		defer tc.teardown()

		tc.waitGroup.Add(1)
		emissionInterval := 10 * time.Millisecond
		numberOfMagicMetricNames := len(blackbox.MagicMetricNames())
		expectedEmissionCount := int(tc.testDuration/emissionInterval) * numberOfMagicMetricNames

		startTime := time.Now().UnixNano()
		go func() {
			tc.blackbox.StartEmittingReliabilityMetrics(
				"source-1",
				emissionInterval,
				tc.client,
				tc.stop,
			)
			tc.waitGroup.Done()
		}()

		var points []*rpc.Point
		Eventually(func() int {
			points = tc.metricStore.GetPoints()
			return len(points)
		}, tc.testDuration+tc.timingFudgeFactor).Should(BeNumerically(">=", expectedEmissionCount))

		for i, expected_metric_name := range blackbox.MagicMetricNames() {
			Expect(points[i]).To(PointTo(MatchFields(IgnoreExtras, Fields{
				"Timestamp": BeNumerically("~", startTime, int64(time.Second)),
				"Name":      Equal(expected_metric_name),
				"Value":     Equal(10.0),
				"Labels":    HaveKeyWithValue("source_id", "source-1"),
			})))
		}

		close(tc.stop)
		tc.waitGroup.Wait()
	})

	It("emits performance test metric at the configured interval", func() {
		tc := setup()
		defer tc.teardown()

		emissionInterval := 10 * time.Millisecond
		expectedEmissionCount := int(tc.testDuration / emissionInterval)

		labels := map[string][]string{"onelabel": {"onevalue"}}

		points := tc.collectPerformanceMetrics(emissionInterval, expectedEmissionCount, labels)

		Expect(points[0]).To(PointTo(MatchFields(IgnoreExtras, Fields{
			"Name":  Equal("blackbox_performance_canary"),
			"Value": Equal(10.0),
			"Labels": MatchAllKeys(Keys{
				"source_id": Equal("source-1"),
				"onelabel":  Equal("onevalue"),
			}),
		})))
	})

	// TODO: this test is a flake. we should probably mock the client instead.
	It("round robin assigns label values", func() {
		tc := setup()
		defer tc.teardown()

		emissionInterval := 10 * time.Millisecond
		expectedEmissionCount := 3
		tc.testDuration = time.Duration(expectedEmissionCount) * emissionInterval

		labels := map[string][]string{"onelabel": {"one", "two"}}

		points := tc.collectPerformanceMetrics(emissionInterval, expectedEmissionCount, labels)

		Expect(points[0]).To(PointTo(MatchFields(IgnoreExtras, Fields{
			"Name": Equal("blackbox_performance_canary"),
			"Labels": MatchAllKeys(Keys{
				"source_id": Equal("source-1"),
				"onelabel":  Equal("one"),
			}),
		})))
		Expect(points[1]).To(PointTo(MatchFields(IgnoreExtras, Fields{
			"Name": Equal("blackbox_performance_canary"),
			"Labels": MatchAllKeys(Keys{
				"source_id": Equal("source-1"),
				"onelabel":  Equal("two"),
			}),
		})))
		Expect(points[2]).To(PointTo(MatchFields(IgnoreExtras, Fields{
			"Name": Equal("blackbox_performance_canary"),
			"Labels": MatchAllKeys(Keys{
				"source_id": Equal("source-1"),
				"onelabel":  Equal("one"),
			}),
		})))
	})
})

func setup() testContext {
	tlsConfig, _ := sharedtls.NewMutualTLSConfig(
		shared.Cert("metric-store-ca.crt"),
		shared.Cert("metric-store.crt"),
		shared.Cert("metric-store.key"),
		metricstore.COMMON_NAME,
	)
	ms := testing.NewSpyMetricStore(
		tlsConfig,
	)

	addrs := ms.Start()
	client, err := ingressclient.NewIngressClient(addrs.IngressAddr, tlsConfig)
	Expect(err).ToNot(HaveOccurred())

	time.Sleep(10 * time.Millisecond)

	return testContext{
		blackbox:          blackbox.NewBlackbox(logger.NewNop()),
		client:            client,
		testDuration:      100 * time.Millisecond,
		timingFudgeFactor: 1 * time.Second,
		metricStore:       ms,
		msAddrs:           addrs,
		tlsConfig:         tlsConfig,

		stop:      make(chan bool),
		waitGroup: &sync.WaitGroup{},
	}
}

func (tc testContext) teardown() {
	tc.metricStore.Stop()
}

type testContext struct {
	blackbox          *blackbox.Blackbox
	client            *ingressclient.IngressClient
	testDuration      time.Duration
	timingFudgeFactor time.Duration

	metricStore *testing.SpyMetricStore
	msAddrs     testing.SpyMetricStoreAddrs
	tlsConfig   *tls.Config

	stop      chan bool
	waitGroup *sync.WaitGroup
}

func (t *testContext) collectPerformanceMetrics(emissionInterval time.Duration, expectedEmissionCount int, labels map[string][]string) []*rpc.Point {
	t.waitGroup.Add(1)

	go func() {
		t.blackbox.StartEmittingPerformanceTestMetrics(
			"source-1",
			emissionInterval,
			t.client,
			t.stop,
			labels,
		)
		t.waitGroup.Done()
	}()

	var points []*rpc.Point
	Eventually(func() int {
		points = t.metricStore.GetPoints()
		return len(points)
	}, t.testDuration+t.timingFudgeFactor).Should(BeNumerically(">=", expectedEmissionCount))

	close(t.stop)
	t.waitGroup.Wait()

	return points
}
