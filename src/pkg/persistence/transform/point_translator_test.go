package transform_test

import (
	"time"

	"github.com/cloudfoundry/metric-store-release/src/pkg/persistence/transform"
	rpc "github.com/cloudfoundry/metric-store-release/src/pkg/rpc/metricstore_v1"
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

	Describe("SeriesDataFromInfluxPoint()", func() {
		It("converts an influxPoint and slice of field names to a sample and labels", func() {
			influxPoint := &query.FloatPoint{
				Name:  "metric_name",
				Time:  10 * int64(time.Millisecond),
				Value: 99.0,
				Tags:  query.NewTags(map[string]string{"foo": "bar"}),
				Aux:   []interface{}{"bax"},
			}
			fields := []string{"fuz"}

			sample, labels := transform.SeriesDataFromInfluxPoint(influxPoint, fields)

			Expect(sample.TimeInMilliseconds).To(Equal(int64(10)))
			Expect(sample.Value).To(Equal(99.0))

			Expect(labels).To(HaveKeyWithValue("__name__", "metric_name"))
			Expect(labels).To(HaveKeyWithValue("foo", "bar"))
			Expect(labels).To(HaveKeyWithValue("fuz", "bax"))
		})
	})

	Describe("SeriesDataFromPromQLSample()", func() {
		It("converts a PromQL Sample to a sample and labels", func() {
			promQLSample := &rpc.PromQL_Sample{
				Metric: map[string]string{"__name__": "metric_name", "foo": "bar"},
				Point: &rpc.PromQL_Point{
					Time:  10,
					Value: 99.0,
				},
			}

			sample, labels := transform.SeriesDataFromPromQLSample(promQLSample)

			Expect(sample.TimeInMilliseconds).To(Equal(int64(10)))
			Expect(sample.Value).To(Equal(99.0))

			Expect(labels).To(HaveKeyWithValue("__name__", "metric_name"))
			Expect(labels).To(HaveKeyWithValue("foo", "bar"))
		})
	})

	Describe("SeriesDataFromPromQLSeries()", func() {
		It("converts a PromQL Series to a sample and labels", func() {
			promQLSeries := &rpc.PromQL_Series{
				Metric: map[string]string{"__name__": "metric_name", "foo": "bar"},
				Points: []*rpc.PromQL_Point{
					{
						Time:  10,
						Value: 99.0,
					},
				},
			}

			samples, labels := transform.SeriesDataFromPromQLSeries(promQLSeries)

			sample := samples[0]
			Expect(sample.TimeInMilliseconds).To(Equal(int64(10)))
			Expect(sample.Value).To(Equal(99.0))

			Expect(labels).To(HaveKeyWithValue("__name__", "metric_name"))
			Expect(labels).To(HaveKeyWithValue("foo", "bar"))
		})
	})
})
