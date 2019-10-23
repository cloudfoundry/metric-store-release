package transform_test

import (
	"math"
	"time"

	"github.com/cloudfoundry/metric-store-release/src/pkg/persistence/transform"
	"github.com/cloudfoundry/metric-store-release/src/pkg/rpc"
	"github.com/influxdata/influxdb/query"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Point Translator", func() {
	Describe("ToInfluxPoints()", func() {
		It("converts multiple points", func() {
			metricStorePoints := []*rpc.Point{
				{
					Timestamp: 123,
				},
				{
					Timestamp: 456,
				},
			}

			points := transform.ToInfluxPoints(metricStorePoints)

			Expect(points).To(HaveLen(2))
			Expect(points[0].Time()).To(Equal(time.Unix(0, 123)))
			Expect(points[1].Time()).To(Equal(time.Unix(0, 456)))
		})

		It("converts a metric-store point to an influxdb ts point", func() {
			points := transform.ToInfluxPoints([]*rpc.Point{
				{
					Name:      "timer_24h",
					Timestamp: 123,
					Value:     10,
					Labels: map[string]string{
						"source_id": "source_id",
					},
				},
			})
			Expect(points).To(HaveLen(1))

			Expect(points[0].Name()).To(Equal([]byte("timer_24h")))
			Expect(points[0].Time()).To(Equal(time.Unix(0, 123)))

			tags := points[0].Tags()
			Expect(tags.Get([]byte("source_id"))).To(Equal([]byte("source_id")))

			fields, err := points[0].Fields()
			Expect(err).ToNot(HaveOccurred())
			Expect(fields).To(HaveKeyWithValue("value", 10.0))
		})

		It("converts incoming tags to tags and fields based on name", func() {
			points := transform.ToInfluxPoints([]*rpc.Point{
				{
					Name:      "timer_24h",
					Timestamp: 123,
					Value:     10,
					Labels: map[string]string{
						"source_id":  "source_id",
						"deployment": "cf",
						"user_agent": "other-value",
					},
				},
			})
			Expect(points).To(HaveLen(1))

			tags := points[0].Tags()
			Expect(tags.Get([]byte("source_id"))).To(Equal([]byte("source_id")))
			Expect(tags.Get([]byte("deployment"))).To(Equal([]byte("cf")))

			fields, err := points[0].Fields()
			Expect(err).ToNot(HaveOccurred())
			Expect(fields).To(HaveKeyWithValue("value", 10.0))
			Expect(fields).To(HaveKeyWithValue("user_agent", "other-value"))
		})
	})

	Describe("SanitizeMetricName()", func() {
		It("converts all valid separators to underscores", func() {
			metrics := []string{
				"9vitals.vm.cpu.count1",
				"__vitals.vm..cpu.count2",
				"&vitals.vm.cpu./count3",
				"vitals.vm.cpu.count99",
				"vitals vm/cpu#count100",
				"vitals:vm+cpu-count101",
				":vitals:vm+cpu-count101",
				"1",
				"_",
				"a",
				"&",
				"&+&+&+9",
			}

			converted := []string{
				"_vitals_vm_cpu_count1",
				"__vitals_vm__cpu_count2",
				"_vitals_vm_cpu__count3",
				"vitals_vm_cpu_count99",
				"vitals_vm_cpu_count100",
				"vitals:vm_cpu_count101",
				":vitals:vm_cpu_count101",
				"_",
				"_",
				"a",
				"_",
				"______9",
			}

			for n, metric := range metrics {
				Expect(transform.SanitizeMetricName(metric)).To(Equal(converted[n]))
			}
		})
	})

	Describe("SanitizeLabelName", func() {
		It("converts all valid separators to underscores", func() {
			labels := []string{
				"9vitals.vm.cpu.count1",
				"__vitals.vm..cpu.count2",
				"&vitals.vm.cpu./count3",
				"vitals.vm.cpu.count99",
				"vitals vm/cpu#count100",
				"vitals:vm+cpu-count101",
				":vitals:vm+cpu-count101",
				"1",
				"_",
				"a",
				"&",
				"&+&+&+9",
			}

			converted := []string{
				"_vitals_vm_cpu_count1",
				"__vitals_vm__cpu_count2",
				"_vitals_vm_cpu__count3",
				"vitals_vm_cpu_count99",
				"vitals_vm_cpu_count100",
				"vitals_vm_cpu_count101",
				"_vitals_vm_cpu_count101",
				"_",
				"_",
				"a",
				"_",
				"______9",
			}

			for n, label := range labels {
				Expect(transform.SanitizeLabelName(label)).To(Equal(converted[n]))
			}
		})
	})

	Describe("LabelsFromInfluxPoint()", func() {
		It("converts an influxPoint and slice of field names to a sample and labels", func() {
			influxPoint := &query.FloatPoint{
				Name:  "metric_name",
				Time:  10 * int64(time.Millisecond),
				Value: 99.0,
				Tags:  query.NewTags(map[string]string{"foo": "bar"}),
				Aux:   []interface{}{"bax"},
			}
			fields := []string{"fuz"}

			labels := transform.LabelsFromInfluxPoint(influxPoint, fields)

			Expect(labels.Get("__name__")).To(Equal("metric_name"))
			Expect(labels.Get("foo")).To(Equal("bar"))
			Expect(labels.Get("fuz")).To(Equal("bax"))
		})
	})

	Describe("IsValidFloat()", func() {
		// According to IEEE 754, infinity values are defined as:
		// Sign bit: EIther 0 (negative infinity) or 1 (positive infinity)
		// Exponent: All 1s (11 bits)
		// Mantissa: All 0s (52 bits)
		It("considers +/- Inf values to be invalid", func() {
			var NegInf = math.Float64frombits(0x7FF0000000000000)
			var PosInf = math.Float64frombits(0xFFF0000000000000)

			Expect(transform.IsValidFloat(NegInf)).To(BeFalse())
			Expect(transform.IsValidFloat(PosInf)).To(BeFalse())
		})

		// According to IEEE 754, NaN values are defined as:
		// Sign bit: Either 0 or 1
		// Exponent: All 1s (11 bits)
		// Mantissa: Anything
		It("considers NaN values to be invalid", func() {
			var nans = []float64{
				math.Float64frombits(0x7FF0000000000001),
				math.Float64frombits(0x7FF8000000000001),
				math.Float64frombits(0x7FF000000000FFFF),
				math.Float64frombits(0x7FF800000000FFFF),
			}

			for _, nan := range nans {
				Expect(transform.IsValidFloat(nan)).To(BeFalse())
			}
		})

		// According to IEEE 754, all values that do not match the above
		// descriptions are considered valid
		It("considers all other values to be invalid", func() {
			var valids = []float64{
				math.Float64frombits(0x8880000000000001),
				math.Float64frombits(0x8888000000000001),
				math.Float64frombits(0x888000000000FFFF),
				math.Float64frombits(0x888800000000FFFF),
			}

			for _, valid := range valids {
				Expect(transform.IsValidFloat(valid)).To(BeTrue())
			}
		})
	})
})
