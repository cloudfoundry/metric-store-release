package ingressclient_test

import (
	"crypto/tls"
	"log"

	. "github.com/cloudfoundry/metric-store-release/src/pkg/ingressclient"
	rpc "github.com/cloudfoundry/metric-store-release/src/pkg/rpc/metricstore_v1"
	metricstoretls "github.com/cloudfoundry/metric-store-release/src/pkg/tls"

	. "github.com/cloudfoundry/metric-store-release/src/pkg/matchers"
	"github.com/cloudfoundry/metric-store-release/src/pkg/testing"
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
		tlsConfig, err := metricstoretls.NewMutualTLSConfig(
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
		points := []*rpc.Point{point}

		ingressClient, err := NewIngressClient(ingressAddress, ingressTlsConfig, WithIngressClientLogger(log.New(GinkgoWriter, "", 0)))
		Expect(err).ToNot(HaveOccurred())

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
		points := []*rpc.Point{point}

		ingressClient, err := NewIngressClient(ingressAddress, ingressTlsConfig, WithIngressClientLogger(log.New(GinkgoWriter, "", 0)))
		Expect(err).ToNot(HaveOccurred())
		metricStore.Stop()

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

		magicNumberOfPointsToSurpassMaxPayload := 50000
		points := make([]*rpc.Point, magicNumberOfPointsToSurpassMaxPayload, magicNumberOfPointsToSurpassMaxPayload)
		for n, _ := range points {
			points[n] = point
		}

		ingressClient, err := NewIngressClient(ingressAddress, ingressTlsConfig, WithIngressClientLogger(log.New(GinkgoWriter, "", 0)))
		Expect(err).ToNot(HaveOccurred())

		Expect(ingressClient.Write(points)).ToNot(Succeed())
	})
})
