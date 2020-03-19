package rules_test

import (
	. "github.com/cloudfoundry/metric-store-release/src/internal/rules"
	"github.com/cloudfoundry/metric-store-release/src/internal/testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("RuleManagerFile", func() {
	Describe("Delete", func() {
		It("deletes the directory for a manager", func() {
			tempStorage := testing.NewTempStorage()
			defer tempStorage.Cleanup()

			ruleManagerFiles := NewRuleManagerFiles(tempStorage.Path())

			_, err := ruleManagerFiles.Create("app-metrics", nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(tempStorage.Directories()).To(ConsistOf("app-metrics"))

			Expect(ruleManagerFiles.Delete("app-metrics")).To(Succeed())
			Expect(tempStorage.Directories()).To(BeEmpty())
		})

		It("errors if manager file doesn't exist", func() {
			tempStorage := testing.NewTempStorage()
			defer tempStorage.Cleanup()

			ruleManagerFiles := NewRuleManagerFiles(tempStorage.Path())

			Expect(tempStorage.Directories()).To(BeEmpty())

			err := ruleManagerFiles.Delete("app-metrics")
			Expect(err).To(MatchError(ManagerNotExistsError))
		})
	})
})
