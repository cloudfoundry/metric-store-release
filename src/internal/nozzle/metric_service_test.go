package nozzle_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/cloudfoundry/metric-store-release/src/cmd/nozzle/app"
	. "github.com/cloudfoundry/metric-store-release/src/internal/nozzle"
)

var _ = Describe("Metric Service", func() {

	Describe("When Envelope Selector is enabled", func() {
		It("writes each envelope as a point when metric have a matched tag", func() {

			metricService := NewMetricService(nil,
				WithMetricFiltering([]string{AppId, ApplicationGuid}, true))
			Expect(metricService.AllowListedMetric(map[string]string{AppId: "some-source-id"})).To(BeTrue())
			Expect(metricService.AllowListedMetric(map[string]string{ApplicationGuid: "some-source-id"})).To(BeTrue())
			Expect(metricService.AllowListedMetric(map[string]string{AppId: "some-source-id", ApplicationGuid: "some-source-id"})).To(BeTrue())
			Expect(metricService.AllowListedMetric(map[string]string{"tag-1": "some", "tag-2": "thing"})).To(BeFalse())
		})
	})
})
