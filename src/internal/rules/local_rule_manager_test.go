package rules_test

import (
	. "github.com/cloudfoundry/metric-store-release/src/internal/rules"
	"github.com/cloudfoundry/metric-store-release/src/internal/testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("LocalRuleManager", func() {
	Describe("DeleteManager", func() {
		It("deletes an existing rule manager", func() {
			tempStorage := testing.NewTempStorage()
			defer tempStorage.Cleanup()

			spyPromRuleManagers := testing.NewPromRuleManagersSpy()
			localRuleManager := NewLocalRuleManager(tempStorage.Path(), spyPromRuleManagers)

			Expect(localRuleManager.CreateManager("app-metrics", "")).To(Succeed())
			Expect(spyPromRuleManagers.ManagerIds()).To(ConsistOf("app-metrics"))
			Expect(tempStorage.FileNames()).To(ConsistOf("app-metrics"))

			Expect(localRuleManager.DeleteManager("app-metrics")).To(Succeed())
			Expect(spyPromRuleManagers.ManagerIds()).To(BeEmpty())
			Expect(tempStorage.FileNames()).To(BeEmpty())
		})

		It("errors if manager file doesn't exist", func() {
			tempStorage := testing.NewTempStorage()
			defer tempStorage.Cleanup()

			spyPromRuleManagers := testing.NewPromRuleManagersSpy()
			localRuleManager := NewLocalRuleManager(tempStorage.Path(), spyPromRuleManagers)

			Expect(tempStorage.FileNames()).To(BeEmpty())
			Expect(localRuleManager.DeleteManager("app-metrics")).ToNot(Succeed())
		})

		It("errors if manager id doesn't exist", func() {
			tempStorage := testing.NewTempStorage()
			defer tempStorage.Cleanup()

			spyPromRuleManagers := testing.NewPromRuleManagersSpy()
			localRuleManager := NewLocalRuleManager(tempStorage.Path(), spyPromRuleManagers)

			Expect(localRuleManager.CreateManager("app-metrics", "")).To(Succeed())
			Expect(spyPromRuleManagers.ManagerIds()).To(ConsistOf("app-metrics"))

			spyPromRuleManagers.Delete("app-metrics")
			Expect(localRuleManager.DeleteManager("app-metrics")).ToNot(Succeed())
		})
	})
})
