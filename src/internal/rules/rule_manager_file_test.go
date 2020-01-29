package rules_test

import (
	. "github.com/cloudfoundry/metric-store-release/src/internal/rules"
	"github.com/cloudfoundry/metric-store-release/src/internal/testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("RuleManagerFile", func() {
	Describe("Delete", func() {
		It("deletes the file for a manager", func() {
			tempStorage := testing.NewTempStorage()
			defer tempStorage.Cleanup()

			ruleManagerFile := NewRuleManagerFile(tempStorage.Path())

			_, err := ruleManagerFile.Create("app-metrics", "")
			Expect(err).NotTo(HaveOccurred())
			Expect(tempStorage.FileNames()).To(ConsistOf("app-metrics"))

			Expect(ruleManagerFile.Delete("app-metrics")).To(Succeed())
			Expect(tempStorage.FileNames()).To(BeEmpty())
		})

		It("errors if manager file doesn't exist", func() {
			tempStorage := testing.NewTempStorage()
			defer tempStorage.Cleanup()

			ruleManagerFile := NewRuleManagerFile(tempStorage.Path())

			Expect(tempStorage.FileNames()).To(BeEmpty())

			err := ruleManagerFile.Delete("app-metrics")
			Expect(err).To(MatchError(ManagerNotExistsError))
		})
	})
})
