package rules_test

import (
	"io/ioutil"
	"log"
	"os"
	"time"

	"github.com/cloudfoundry/metric-store-release/src/pkg/logger"
	. "github.com/cloudfoundry/metric-store-release/src/internal/rules"
	"github.com/cloudfoundry/metric-store-release/src/internal/testing"
	"github.com/cloudfoundry/metric-store-release/src/pkg/persistence"

	shared "github.com/cloudfoundry/metric-store-release/src/internal/testing"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("PromRuleManagers", func() {
	var createRuleFile = func(alertmanagerAddr, prefix string) (*os.File, func()) {
		yml := []byte(`
# ALERTMANAGER_URL ` + alertmanagerAddr + `
groups:
- name: ` + prefix + `-group
  rules:
  - alert: ` + prefix + `Alert
    expr: metric_store_test_metric > 2
  - record: ` + prefix + `Record
    expr: metric_store_test_metric
`)
		tmpfile, err := ioutil.TempFile("", prefix)
		Expect(err).NotTo(HaveOccurred())
		if _, err := tmpfile.Write(yml); err != nil {
			log.Fatal(err)
		}
		if err := tmpfile.Close(); err != nil {
			log.Fatal(err)
		}
		return tmpfile, func() {
			os.Remove(tmpfile.Name())
		}
	}

	Describe("#Create", func() {
		It("creates a rule manager from a given file", func() {
			alertmanager := testing.NewAlertManagerSpy()
			alertmanager.Start()
			defer alertmanager.Stop()

			tmpfile, cleanup := createRuleFile(alertmanager.Addr(), "app_metrics")
			defer cleanup()

			tempStorage := testing.NewTempStorage()
			defer tempStorage.Cleanup()

			spyMetrics := shared.NewSpyMetricRegistrar()
			persistentStore := persistence.NewStore(
				tempStorage.Path(),
				spyMetrics,
			)

			ruleManagers := NewRuleManagers(
				persistentStore,
				testing.NewQueryEngine(),
				5*time.Second,
				logger.NewTestLogger(GinkgoWriter),
				spyMetrics,
			)
			ruleManagers.Create("app-metrics", tmpfile.Name(), alertmanager.Addr())

			Expect(len(ruleManagers.RuleGroups())).To(Equal(1))
			Expect(len(ruleManagers.AlertingRules())).To(Equal(1))
			Eventually(func() int { return len(ruleManagers.Alertmanagers()) }, 10).Should(Equal(1))
		})
	})

	Describe("#DeleteManager", func() {
		It("deletes a rule manager", func() {
			tmpfile, cleanup := createRuleFile("", "app_metrics")
			defer cleanup()

			tempStorage := testing.NewTempStorage()
			defer tempStorage.Cleanup()

			spyMetrics := shared.NewSpyMetricRegistrar()
			persistentStore := persistence.NewStore(
				tempStorage.Path(),
				spyMetrics,
			)

			ruleManagers := NewRuleManagers(
				persistentStore,
				testing.NewQueryEngine(),
				5*time.Second,
				logger.NewTestLogger(GinkgoWriter),
				spyMetrics,
			)
			ruleManagers.Create("app-metrics", tmpfile.Name(), "")

			Expect(len(ruleManagers.RuleGroups())).To(Equal(1))
			Expect(len(ruleManagers.AlertingRules())).To(Equal(1))

			ruleManagers.DeleteManager("app-metrics")

			Expect(len(ruleManagers.RuleGroups())).To(Equal(0))
			Expect(len(ruleManagers.AlertingRules())).To(Equal(0))
		})
	})

	Describe("#Alertmanagers", func() {
		It("Returns unique alertmanagers", func() {
			alertmanager := testing.NewAlertManagerSpy()
			alertmanager.Start()
			defer alertmanager.Stop()

			tmpfile1, cleanup1 := createRuleFile(alertmanager.Addr(), "first_manager")
			defer cleanup1()

			tmpfile2, cleanup2 := createRuleFile(alertmanager.Addr(), "second_manager")
			defer cleanup2()

			tempStorage := testing.NewTempStorage()
			defer tempStorage.Cleanup()

			spyMetrics := shared.NewSpyMetricRegistrar()
			persistentStore := persistence.NewStore(
				tempStorage.Path(),
				spyMetrics,
			)

			ruleManagers := NewRuleManagers(
				persistentStore,
				testing.NewQueryEngine(),
				5*time.Second,
				logger.NewTestLogger(GinkgoWriter),
				spyMetrics,
			)

			ruleManagers.Create("manager-1", tmpfile1.Name(), alertmanager.Addr())
			ruleManagers.Create("manager-2", tmpfile2.Name(), alertmanager.Addr())

			Expect(len(ruleManagers.RuleGroups())).To(Equal(2))
			Expect(len(ruleManagers.AlertingRules())).To(Equal(2))
			Eventually(func() int { return len(ruleManagers.Alertmanagers()) }, 10).Should(Equal(1))
		})
	})
})
