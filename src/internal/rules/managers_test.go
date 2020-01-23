package rules_test

import (
	"io/ioutil"
	"log"
	"os"
	"time"

	"github.com/cloudfoundry/metric-store-release/src/internal/logger"
	. "github.com/cloudfoundry/metric-store-release/src/internal/rules"
	"github.com/cloudfoundry/metric-store-release/src/internal/testing"
	"github.com/cloudfoundry/metric-store-release/src/pkg/persistence"
	"github.com/prometheus/prometheus/promql"

	shared "github.com/cloudfoundry/metric-store-release/src/internal/testing"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Rules", func() {
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

	It("creates a rule manager from a given file", func() {
		alertmanager := testing.NewAlertManagerSpy()
		alertmanager.Start()
		defer alertmanager.Stop()

		tmpfile, cleanup := createRuleFile(alertmanager.Addr(), "only")
		defer cleanup()

		storagePath, err := ioutil.TempDir("", "metric-store")
		if err != nil {
			panic(err)
		}
		defer os.RemoveAll(storagePath)

		spyMetrics := shared.NewSpyMetricRegistrar()
		persistentStore := persistence.NewStore(
			storagePath,
			spyMetrics,
		)

		engineOpts := promql.EngineOpts{
			MaxConcurrent: 10,
			MaxSamples:    20e6,
			Timeout:       time.Minute,
			Logger:        logger.NewTestLogger(GinkgoWriter),
		}
		queryEngine := promql.NewEngine(engineOpts)

		ruleManagers := NewRuleManagers(
			persistentStore,
			queryEngine,
			time.Duration(5*time.Second),
			logger.NewTestLogger(GinkgoWriter),
			spyMetrics,
		)
		ruleManagers.Create("rule_manager_yml", tmpfile.Name(), alertmanager.Addr())

		Expect(len(ruleManagers.RuleGroups())).To(Equal(1))
		Expect(len(ruleManagers.AlertingRules())).To(Equal(1))
		Eventually(func() int { return len(ruleManagers.Alertmanagers()) }, 10).Should(Equal(1))
	})

	It("Returns unique alertmanagers", func() {
		alertmanager := testing.NewAlertManagerSpy()
		alertmanager.Start()
		defer alertmanager.Stop()

		tmpfile1, cleanup1 := createRuleFile(alertmanager.Addr(), "first")
		defer cleanup1()

		tmpfile2, cleanup2 := createRuleFile(alertmanager.Addr(), "second")
		defer cleanup2()

		storagePath, err := ioutil.TempDir("", "metric-store")
		if err != nil {
			panic(err)
		}
		defer os.RemoveAll(storagePath)

		spyMetrics := shared.NewSpyMetricRegistrar()
		persistentStore := persistence.NewStore(
			storagePath,
			spyMetrics,
		)

		engineOpts := promql.EngineOpts{
			MaxConcurrent: 10,
			MaxSamples:    1e6,
			Timeout:       time.Minute,
			Logger:        logger.NewTestLogger(GinkgoWriter),
		}
		queryEngine := promql.NewEngine(engineOpts)
		ruleManagers := NewRuleManagers(
			persistentStore,
			queryEngine,
			time.Duration(5*time.Second),
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
