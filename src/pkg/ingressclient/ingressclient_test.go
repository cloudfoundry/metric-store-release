package ingressclient_test

import (
	"crypto/tls"

	. "github.com/cloudfoundry/metric-store-release/src/pkg/ingressclient"
	"github.com/cloudfoundry/metric-store-release/src/internal/logger"
	"github.com/cloudfoundry/metric-store-release/src/pkg/rpc"
	sharedtls "github.com/cloudfoundry/metric-store-release/src/pkg/tls"

	. "github.com/cloudfoundry/metric-store-release/src/internal/matchers"
	"github.com/cloudfoundry/metric-store-release/src/internal/testing"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("IngressClient", func() {
	var (
		metricStore      *testing.SpyMetricStore
		ingressTlsConfig *tls.Config
		ingressAddress   string
	)

	BeforeEach(func() {
		tlsConfig, err := sharedtls.NewMutualTLSConfig(
			testing.Cert("metric-store-ca.crt"),
			testing.Cert("metric-store.crt"),
			testing.Cert("metric-store.key"),
			"metric-store",
		)
		Expect(err).ToNot(HaveOccurred())
		metricStore = testing.NewSpyMetricStore(tlsConfig)
		addrs := metricStore.Start()

		ingressTlsConfig = tlsConfig
		ingressAddress = addrs.IngressAddr
	})

	It("writes points to metric store", func() {
		point := &rpc.Point{
			Name:      "input",
			Timestamp: 20,
			Value:     50.0,
			Labels: map[string]string{
				"unit":      "mb/s",
				"source_id": "source-id",
			},
		}

		ingressClient, err := NewIngressClient(ingressAddress, ingressTlsConfig, WithIngressClientLogger(logger.NewTestLogger()))
		Expect(err).ToNot(HaveOccurred())

		points := []*rpc.Point{point}
		Expect(ingressClient.Write(points)).To(Succeed())
		Eventually(metricStore.GetPoints).Should(ContainPoints(points))
	})

	It("errors if writing fails", func() {
		point := &rpc.Point{
			Name:      "input",
			Timestamp: 20,
			Value:     50.0,
			Labels: map[string]string{
				"unit":      "mb/s",
				"source_id": "source-id",
			},
		}

		ingressClient, err := NewIngressClient(ingressAddress, ingressTlsConfig, WithIngressClientLogger(logger.NewTestLogger()))
		Expect(err).ToNot(HaveOccurred())
		metricStore.Stop()

		points := []*rpc.Point{point}
		Expect(ingressClient.Write(points)).ToNot(Succeed())
	})

	It("errors if writing fails to to a payload size greater than the max", func() {
		point := &rpc.Point{
			Name:      "input",
			Timestamp: 20,
			Value:     50.0,
			Labels: map[string]string{
				"unit":      "mb/s",
				"source_id": "source-id",
			},
		}

		magicNumberOfPointsToSurpassMaxPayload := 100000
		points := make([]*rpc.Point, magicNumberOfPointsToSurpassMaxPayload, magicNumberOfPointsToSurpassMaxPayload)
		for n := range points {
			points[n] = point
		}

		ingressClient, err := NewIngressClient(ingressAddress, ingressTlsConfig, WithIngressClientLogger(logger.NewTestLogger()))
		Expect(err).ToNot(HaveOccurred())

		Expect(ingressClient.Write(points)).ToNot(Succeed())
	})
})
