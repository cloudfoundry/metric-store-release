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
	It("creates a rule manager from a given file", func() {
		alertmanager := testing.NewAlertManagerSpy()
		alertmanager.Start()
		defer alertmanager.Stop()

		rule_manager_yml := []byte(`
# ALERTMANAGER_URL ` + alertmanager.Addr() + `
groups:
- name: example
  rules:
  - alert: HighNumberOfTestMetrics
    expr: metric_store_test_metric > 2
  - record: SomeNumberOfTestMetrics
    expr: metric_store_test_metric
`)
		tmpfile, err := ioutil.TempFile("", "rule_manager_yml")
		Expect(err).NotTo(HaveOccurred())
		defer os.Remove(tmpfile.Name())
		if _, err := tmpfile.Write(rule_manager_yml); err != nil {
			log.Fatal(err)
		}
		if err := tmpfile.Close(); err != nil {
			log.Fatal(err)
		}

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
			Logger:        logger.NewTestLogger(),
		}
		queryEngine := promql.NewEngine(engineOpts)

		ruleManagers := NewRuleManagers(
			persistentStore,
			queryEngine,
			logger.NewTestLogger(),
			spyMetrics,
		)
		ruleManagers.Create(tmpfile.Name(), alertmanager.Addr())

		Expect(len(ruleManagers.RuleGroups())).To(Equal(1))
		Expect(len(ruleManagers.AlertingRules())).To(Equal(1))
		Eventually(func() int { return len(ruleManagers.Alertmanagers()) }, 10).Should(Equal(1))
	})
})
