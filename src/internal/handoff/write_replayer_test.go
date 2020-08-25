package handoff_test

import (
	"errors"
	"io/ioutil"
	"time"

	"github.com/cloudfoundry/metric-store-release/src/internal/handoff"
	"github.com/cloudfoundry/metric-store-release/src/internal/metrics"
	"github.com/cloudfoundry/metric-store-release/src/pkg/rpc"

	"github.com/cloudfoundry/metric-store-release/src/internal/testing"
	shared "github.com/cloudfoundry/metric-store-release/src/internal/testing"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
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

		queue := handoff.NewDiskBackedQueue(dir)

		spyMetrics := shared.NewSpyMetricRegistrar()
		spyClient := testing.NewSpyTCPClient()

		writeReplayer := handoff.NewWriteReplayer(queue, spyClient, spyMetrics, "0")

		done := make(chan struct{})
		err = writeReplayer.Open(done)
		Expect(err).ToNot(HaveOccurred())

		err = writeReplayer.Write(batch)
		Expect(err).ToNot(HaveOccurred())
		Expect(spyMetrics.Fetch(metrics.MetricStoreReplayerQueuedBytesTotal)()).To(BeNumerically(">", 0))

		By("sending writes while the ingress client is available", func() {
			Eventually(spyMetrics.Fetch(metrics.MetricStoreReplayerReplayedBytesTotal)).Should(BeNumerically(">", 0))
			Eventually(spyMetrics.Fetch(metrics.MetricStoreReplayerReplayErrorsTotal)).Should(BeEquivalentTo(0))
		})

		By("queuing writes while the ingress client is failing", func() {
			spyClient.SetErr(errors.New("metric_store_some_error"))
			err = writeReplayer.Write(batch)
			Expect(err).ToNot(HaveOccurred())

			Eventually(spyMetrics.Fetch(metrics.MetricStoreReplayerDiskUsageBytes)).Should(BeNumerically(">", 0))
			Eventually(spyMetrics.Fetch(metrics.MetricStoreReplayerReplayErrorsTotal)).Should(BeNumerically(">", 0))
		})

		By("sending queued writes once the ingress client starts succeeding", func() {
			spyClient.SetErr(nil)

			Eventually(spyMetrics.Fetch(metrics.MetricStoreReplayerReplayedBytesTotal)).Should(BeNumerically(">", 0))
		})
	})

	It("purges the queue", func() {
		dir, err := ioutil.TempDir("", "node_processor_test")
		Expect(err).ToNot(HaveOccurred())

		spyMetrics := &metrics.NullRegistrar{}
		spyClient := testing.NewSpyTCPClient()
		spyQueue := testing.NewSpyQueue(dir)

		writeReplayer := handoff.NewWriteReplayer(spyQueue, spyClient, spyMetrics, "0")
		writeReplayer.RetryInterval = time.Hour
		writeReplayer.PurgeInterval = time.Millisecond
		writeReplayer.MaxAge = time.Nanosecond

		done := make(chan struct{})
		err = writeReplayer.Open(done)
		Expect(err).ToNot(HaveOccurred())

		Eventually(func() int { return spyQueue.PurgeCalls }).Should(BeNumerically(">", 1))
	})

	It("closes when the done channel is closed", func() {
		dir, err := ioutil.TempDir("", "node_processor_test")
		Expect(err).ToNot(HaveOccurred())

		queue := handoff.NewDiskBackedQueue(dir)

		spyMetrics := shared.NewSpyMetricRegistrar()
		spyClient := testing.NewSpyTCPClient()

		n := handoff.NewWriteReplayer(queue, spyClient, spyMetrics, "0")

		done := make(chan struct{})
		err = n.Open(done)
		Expect(err).ToNot(HaveOccurred())

		close(done)
		Eventually(func() error { return n.Write(nil) }).Should(HaveOccurred())
	})
})
