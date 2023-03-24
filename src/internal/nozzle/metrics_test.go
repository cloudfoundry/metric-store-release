package nozzle_test

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/metric-store-release/src/internal/metrics"
	. "github.com/cloudfoundry/metric-store-release/src/internal/nozzle"
	"github.com/cloudfoundry/metric-store-release/src/internal/testing"
	"github.com/cloudfoundry/metric-store-release/src/pkg/logger"
)

var _ = Describe("collect nozzle metrics", func() {
	It("writes Ingress, Egress and Err metrics", func() {
		tlsServerConfig, tlsClientConfig := buildTLSConfigs()

		streamConnector := newSpyStreamConnector()
		metricStore := testing.NewSpyMetricStore(tlsServerConfig)
		addrs := metricStore.Start()
		defer metricStore.Stop()

		metricRegistrar := testing.NewSpyMetricRegistrar()
		n := NewNozzle(streamConnector,
			addrs.IngressAddr,
			tlsClientConfig,
			"metric-store",
			0,
			false,
			[]string{},
			WithNozzleDebugRegistrar(metricRegistrar),
			WithNozzleTimerRollup(
				100*time.Millisecond,
				[]string{"tag1", "tag2", "status_code"},
				[]string{"tag1", "tag2"},
			),
			WithNozzleLogger(logger.NewTestLogger(GinkgoWriter)),
		)
		go n.Start()

		addEnvelope(1, "memory", "some-source-id", streamConnector)
		addEnvelope(2, "memory", "some-source-id", streamConnector)
		addEnvelope(3, "memory", "some-source-id", streamConnector)

		Eventually(metricRegistrar.Fetch(metrics.NozzleIngressEnvelopesTotal)).Should(Equal(float64(3)))
		Eventually(metricRegistrar.Fetch(metrics.NozzleEgressPointsTotal)).Should(Equal(float64(3)))
		Eventually(metricRegistrar.Fetch(metrics.NozzleEgressErrorsTotal)).Should(Equal(float64(0)))
	})

	It("writes duration seconds histogram metrics", func() {
		tlsServerConfig, tlsClientConfig := buildTLSConfigs()

		streamConnector := newSpyStreamConnector()
		metricStore := testing.NewSpyMetricStore(tlsServerConfig)
		addrs := metricStore.Start()
		defer metricStore.Stop()

		metricRegistrar := testing.NewSpyMetricRegistrar()
		n := NewNozzle(streamConnector,
			addrs.IngressAddr,
			tlsClientConfig,
			"metric-store",
			0,
			false,
			[]string{},
			WithNozzleDebugRegistrar(metricRegistrar),
			WithNozzleTimerRollup(
				100*time.Millisecond,
				[]string{"tag1", "tag2", "status_code"},
				[]string{"tag1", "tag2"},
			),
			WithNozzleLogger(logger.NewTestLogger(GinkgoWriter)),
		)
		go n.Start()

		addEnvelope(1, "memory", "some-source-id", streamConnector)
		Eventually(metricRegistrar.FetchHistogram(metrics.NozzleEgressDurationSeconds)).Should(HaveLen(1))
	})
})
