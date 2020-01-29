package rulesclient_test

import (
	. "github.com/cloudfoundry/metric-store-release/src/pkg/rulesclient"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Rule", func() {
	Describe("Validate()", func() {
		It("returns nil for valid rule groups", func() {
			rule := Rule{
				Record: "cpuRule",
				Expr:   "cpu",
			}

			err := rule.Validate()
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns error when any rule is invalid", func() {
			rule := Rule{}

			err := rule.Validate()
			Expect(err).To(HaveOccurred())
		})
	})
})
