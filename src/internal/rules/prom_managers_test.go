package rules_test

import (
	"io/ioutil"
	"log"
	"os"
	"time"

	. "github.com/cloudfoundry/metric-store-release/src/internal/rules"
	"github.com/cloudfoundry/metric-store-release/src/internal/testing"
	"github.com/cloudfoundry/metric-store-release/src/pkg/logger"
	"github.com/cloudfoundry/metric-store-release/src/pkg/persistence"

	shared "github.com/cloudfoundry/metric-store-release/src/internal/testing"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var createRuleFile = func(alertmanagerAddr, prefix string) string {
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

	cleanups = append(cleanups, func() { os.Remove(tmpfile.Name()) })
	return tmpfile.Name()
}

var spyMetrics *testing.SpyMetricRegistrar
var alertmanager *testing.AlertManagerSpy
var tempStorage testing.TempStorage
var ruleManagers *PromRuleManagers
var cleanups []func()

var _ = Describe("PromRuleManagers", func() {
	BeforeEach(func() {
		cleanups = []func(){}

		alertmanager = testing.NewAlertManagerSpy()
		alertmanager.Start()
		cleanups = append(cleanups, alertmanager.Stop)

		tempStorage = testing.NewTempStorage()
		cleanups = append(cleanups, tempStorage.Cleanup)

		spyMetrics = shared.NewSpyMetricRegistrar()
		persistentStore := persistence.NewStore(
			tempStorage.Path(),
			spyMetrics,
		)

		ruleManagers = NewRuleManagers(
			persistentStore,
			testing.NewQueryEngine(),
			5*time.Second,
			logger.NewTestLogger(GinkgoWriter),
			spyMetrics,
			2*time.Second,
		)

	})

	AfterEach(func() {
		for _, c := range cleanups {
			c()
		}
	})

	Describe("#Create", func() {
		It("creates a rule manager from a given file", func() {
			ruleManagers.Create("appmetrics", createRuleFile("", "appmetrics"), alertmanager.Addr())

			Expect(len(ruleManagers.RuleGroups())).To(Equal(1))
			Expect(len(ruleManagers.AlertingRules())).To(Equal(1))
			Eventually(func() int { return len(ruleManagers.Alertmanagers()) }, 10).Should(Equal(1))
		})
	})

	Describe("#Delete", func() {
		It("deletes a rule manager", func() {
			ruleManagers.Create("appmetrics", createRuleFile("", "appmetrics"), alertmanager.Addr())
			Expect(len(ruleManagers.RuleGroups())).To(Equal(1))
			Expect(len(ruleManagers.AlertingRules())).To(Equal(1))

			ruleManagers.Delete("appmetrics")

			Expect(len(ruleManagers.RuleGroups())).To(Equal(0))
			Expect(len(ruleManagers.AlertingRules())).To(Equal(0))
		})
	})

	Describe("#DeleteAll", func() {
		It("deletes all rule managers", func() {
			ruleManagers.Create("manager1", createRuleFile("", "manager1"), alertmanager.Addr())
			ruleManagers.Create("manager2", createRuleFile("", "manager2"), alertmanager.Addr())
			ruleManagers.Create("manager3", createRuleFile("", "manager3"), alertmanager.Addr())

			Expect(len(ruleManagers.RuleGroups())).To(Equal(3))
			Expect(len(ruleManagers.AlertingRules())).To(Equal(3))

			ruleManagers.DeleteAll()

			Expect(len(ruleManagers.RuleGroups())).To(Equal(0))
			Expect(len(ruleManagers.AlertingRules())).To(Equal(0))
		})
	})

	Describe("#Alertmanagers", func() {
		It("Returns unique alertmanagers", func() {
			ruleManagers.Create("manager1", createRuleFile("", "manager1"), alertmanager.Addr())
			ruleManagers.Create("manager2", createRuleFile("", "manager2"), alertmanager.Addr())

			Expect(len(ruleManagers.RuleGroups())).To(Equal(2))
			Expect(len(ruleManagers.AlertingRules())).To(Equal(2))
			Eventually(func() int { return len(ruleManagers.Alertmanagers()) }, 10).Should(Equal(1))
		})
	})
})
