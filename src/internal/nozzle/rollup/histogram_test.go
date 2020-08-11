package rollup_test

import (
	"fmt"
	"time"

	. "github.com/cloudfoundry/metric-store-release/src/internal/nozzle/rollup"
	"github.com/cloudfoundry/metric-store-release/src/pkg/logger"
	"github.com/cloudfoundry/metric-store-release/src/pkg/rpc"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type histogram struct {
	points []*rpc.Point
}

func (h *histogram) Count() int {
	for _, p := range h.points {
		if p.Name == "http_duration_seconds_count" {
			return int(p.Value)
		}
	}
	Fail("No count point found in histogram")
	return 0
}

func (h *histogram) Sum() int {
	for _, p := range h.points {
		if p.Name == "http_duration_seconds_sum" {
			return int(p.Value)
		}
	}
	Fail("No sum point found in histogram")
	return 0
}

func (h *histogram) Points() []*rpc.Point {
	return h.points
}

func (h *histogram) Bucket(le string) *rpc.Point {
	for _, p := range h.points {
		if p.Name == "http_duration_seconds_bucket" && p.Labels["le"] == le {
			return p
		}
	}
	Fail(fmt.Sprintf("No bucket point found in histogram for le = '%s'", le))
	return nil
}

var _ = Describe("Histogram Rollup", func() {
	extract := func(batches []*PointsBatch) []*histogram {
		var histograms []*histogram

		for _, b := range batches {
			h := &histogram{}
			for _, p := range b.Points {
				h.points = append(h.points, p)
			}
			histograms = append(histograms, h)
		}

		return histograms
	}

	It("returns aggregate information for rolled up events", func() {
		rollup := NewHistogramRollup(
			logger.NewTestLogger(GinkgoWriter),
			"0",
			nil,
		)

		rollup.Record(
			"source-id",
			nil,
			10*int64(time.Second),
		)
		rollup.Record(
			"source-id",
			nil,
			5*int64(time.Second),
		)

		histograms := extract(rollup.Rollup(0))
		Expect(len(histograms)).To(Equal(1))
		Expect(histograms[0].Count()).To(Equal(2))
		Expect(histograms[0].Sum()).To(Equal(15))
	})

	It("returns batches which each includes a size estimate", func() {
		rollup := NewHistogramRollup(
			logger.NewTestLogger(GinkgoWriter),
			"0",
			nil,
		)

		rollup.Record(
			"source-id",
			nil,
			10*int64(time.Second),
		)

		pointsBatches := rollup.Rollup(0)
		Expect(len(pointsBatches)).To(Equal(1))
		Expect(pointsBatches[0].Size).To(BeNumerically(">", 0))
	})

	It("returns points for each bucket in the histogram", func() {
		rollup := NewHistogramRollup(
			logger.NewTestLogger(GinkgoWriter),
			"0",
			nil,
		)

		rollup.Record(
			"source-id",
			nil,
			2*int64(time.Second),
		)
		rollup.Record(
			"source-id",
			nil,
			7*int64(time.Second),
		)
		rollup.Record(
			"source-id",
			nil,
			8*int64(time.Second),
		)

		histograms := extract(rollup.Rollup(0))
		Expect(len(histograms)).To(Equal(1))
		Expect(histograms[0].Bucket("1").Value).To(BeNumerically("==", 0))
		Expect(histograms[0].Bucket("2.5").Value).To(BeNumerically("==", 1))
		Expect(histograms[0].Bucket("5").Value).To(BeNumerically("==", 1))
		Expect(histograms[0].Bucket("10").Value).To(BeNumerically("==", 3))
		Expect(histograms[0].Bucket("+Inf").Value).To(BeNumerically("==", 3))
	})

	It("returns points with the timestamp given to Rollup", func() {
		rollup := NewHistogramRollup(
			logger.NewTestLogger(GinkgoWriter),
			"node-index",
			nil,
		)

		rollup.Record(
			"source-id",
			nil,
			1,
		)

		histograms := extract(rollup.Rollup(88))
		Expect(len(histograms)).To(Equal(1))
		for _, p := range histograms[0].Points() {
			Expect(p.Timestamp).To(Equal(int64(88)), fmt.Sprintf("%#v", p))
		}
	})

	It("returns histograms with labels based on tags", func() {
		rollup := NewHistogramRollup(
			logger.NewTestLogger(GinkgoWriter),
			"node-index",
			[]string{"included-tag"},
		)

		rollup.Record(
			"source-id",
			map[string]string{"included-tag": "foo", "excluded-tag": "bar"},
			1,
		)

		histograms := extract(rollup.Rollup(0))
		Expect(len(histograms)).To(Equal(1))
		for _, p := range histograms[0].Points() {
			Expect(p.Labels).To(And(
				HaveKeyWithValue("included-tag", "foo"),
				HaveKeyWithValue("source_id", "source-id"),
				HaveKeyWithValue("node_index", "node-index"),
			))
		}
	})

	It("returns points that track a running total of rolled up events", func() {
		rollup := NewHistogramRollup(
			logger.NewTestLogger(GinkgoWriter),
			"0",
			[]string{"included-tag"},
		)

		rollup.Record(
			"source-id",
			map[string]string{"included-tag": "foo", "excluded-tag": "bar"},
			1,
		)

		histograms := extract(rollup.Rollup(0))
		Expect(len(histograms)).To(Equal(1))
		Expect(histograms[0].Count()).To(Equal(1))

		rollup.Record(
			"source-id",
			map[string]string{"included-tag": "foo", "excluded-tag": "bar"},
			1,
		)

		histograms = extract(rollup.Rollup(1))
		Expect(len(histograms)).To(Equal(1))
		Expect(histograms[0].Count()).To(Equal(2))
	})

	It("returns separate histograms for distinct source IDs", func() {
		rollup := NewHistogramRollup(
			logger.NewTestLogger(GinkgoWriter),
			"0",
			[]string{"included-tag"},
		)

		rollup.Record(
			"source-id",
			map[string]string{"included-tag": "foo"},
			1,
		)
		rollup.Record(
			"other-source-id",
			map[string]string{"included-tag": "foo"},
			1,
		)

		histograms := extract(rollup.Rollup(0))
		Expect(len(histograms)).To(Equal(2))
		Expect(histograms[0].Count()).To(Equal(1))
		Expect(histograms[1].Count()).To(Equal(1))
	})

	It("returns separate histograms for different included tags", func() {
		rollup := NewHistogramRollup(
			logger.NewTestLogger(GinkgoWriter),
			"0",
			[]string{"included-tag"},
		)

		rollup.Record(
			"source-id",
			map[string]string{"included-tag": "foo"},
			1,
		)
		rollup.Record(
			"source-id",
			map[string]string{"included-tag": "other-foo"},
			1,
		)

		histograms := extract(rollup.Rollup(0))
		Expect(len(histograms)).To(Equal(2))
		Expect(histograms[0].Count()).To(Equal(1))
		Expect(histograms[1].Count()).To(Equal(1))
	})

	It("does not return separate histograms for different excluded tags", func() {
		rollup := NewHistogramRollup(
			logger.NewTestLogger(GinkgoWriter),
			"0",
			[]string{"included-tag"},
		)

		rollup.Record(
			"source-id",
			map[string]string{"excluded-tag": "bar"},
			1,
		)
		rollup.Record(
			"source-id",
			map[string]string{"excluded-tag": "other-bar"},
			1,
		)

		histograms := extract(rollup.Rollup(0))
		Expect(len(histograms)).To(Equal(1))
		Expect(histograms[0].Count()).To(Equal(2))
		Expect(histograms[0].Points()[0].Labels).ToNot(HaveKey("excluded-tag"))
	})
})
