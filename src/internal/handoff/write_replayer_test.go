package handoff_test

import (
	"errors"
	"io/ioutil"

	"github.com/cloudfoundry/metric-store-release/src/pkg/rpc"
	"github.com/cloudfoundry/metric-store-release/src/internal/handoff"
	"github.com/cloudfoundry/metric-store-release/src/internal/metrics"

	shared "github.com/cloudfoundry/metric-store-release/src/internal/testing"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/cloudfoundry/metric-store-release/src/internal/testing"
)

var _ = Describe("WriteReplayer", func() {
	It("writes batches", func() {
		batch := []*rpc.Point{
			{
				Timestamp: 1,
			},
			{
				Timestamp: 2,
			},
		}

		dir, err := ioutil.TempDir("", "node_processor_test")
		Expect(err).ToNot(HaveOccurred())

		spyMetrics := shared.NewSpyMetricRegistrar()
		spyClient := testing.NewSpyTCPClient()

		n := handoff.NewWriteReplayer(dir, spyClient, spyMetrics, "0")

		err = n.Open()
		Expect(err).ToNot(HaveOccurred())

		err = n.Write(batch)
		Expect(err).ToNot(HaveOccurred())
		Expect(spyMetrics.Fetch(metrics.MetricStoreReplayerQueuedBytesTotal)()).To(BeNumerically(">", 0))

		By("sending writes while the ingress client is available", func() {
			Eventually(spyMetrics.Fetch(metrics.MetricStoreReplayerReplayedBytesTotal)).Should(BeNumerically(">", 0))
			Eventually(spyMetrics.Fetch(metrics.MetricStoreReplayerReplayErrorsTotal)).Should(BeEquivalentTo(0))
		})

		By("queuing writes while the ingress client is failing", func() {
			spyClient.SetErr(errors.New("metric_store_some_error"))
			err = n.Write(batch)

			Eventually(spyMetrics.Fetch(metrics.MetricStoreReplayerDiskUsageBytes)).Should(BeNumerically(">", 0))
			Eventually(spyMetrics.Fetch(metrics.MetricStoreReplayerReplayErrorsTotal)).Should(BeNumerically(">", 0))
		})

		By("sending queued writes once the ingress client starts succeeding", func() {
			spyClient.SetErr(nil)

			Eventually(spyMetrics.Fetch(metrics.MetricStoreReplayerReplayedBytesTotal)).Should(BeNumerically(">", 0))
		})

		By("shutting down the write replayer", func() {
			err = n.Close()
			Expect(err).ToNot(HaveOccurred())

			// Confirm that purging works ok.
			err = n.Purge()
			Expect(err).ToNot(HaveOccurred())
			Expect(dir).ToNot(BeADirectory())
		})
	})
})
