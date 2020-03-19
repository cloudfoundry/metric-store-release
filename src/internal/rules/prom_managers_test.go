package rules_test

import (
	"io/ioutil"
	"log"
	"os"
	"time"

	. "github.com/cloudfoundry/metric-store-release/src/internal/rules"
	"github.com/cloudfoundry/metric-store-release/src/internal/testing"
	sharedtls "github.com/cloudfoundry/metric-store-release/src/internal/tls"
	"github.com/cloudfoundry/metric-store-release/src/pkg/logger"
	"github.com/cloudfoundry/metric-store-release/src/pkg/persistence"
	"github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	prom_config "github.com/prometheus/prometheus/config"
	sd_config "github.com/prometheus/prometheus/discovery/config"
	"github.com/prometheus/prometheus/discovery/targetgroup"

	shared "github.com/cloudfoundry/metric-store-release/src/internal/testing"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("PromRuleManagers", func() {
	var spyMetrics *testing.SpyMetricRegistrar
	var alertmanager *testing.AlertManagerSpy
	var tempStorage testing.TempStorage
	var ruleManagers *PromRuleManagers
	var cleanups []func()
	var alertManagerConfigs *prom_config.AlertmanagerConfigs

	var createRuleFile = func(prefix string) string {
		yml := []byte(`
---
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

	BeforeEach(func() {
		cleanups = []func(){}

		caCert := shared.Cert("metric-store-ca.crt")
		cert := shared.Cert("metric-store.crt")
		key := shared.Cert("metric-store.key")
		tlsConfig, err := sharedtls.NewMutualTLSClientConfig(caCert, cert, key, "metric-store")
		Expect(err).ToNot(HaveOccurred())

		alertmanager = testing.NewAlertManagerSpy(tlsConfig)
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

		alertManagerConfigs = &prom_config.AlertmanagerConfigs{{
			ServiceDiscoveryConfig: sd_config.ServiceDiscoveryConfig{
				StaticConfigs: []*targetgroup.Group{
					{
						Targets: []model.LabelSet{
							{
								"__address__": model.LabelValue(alertmanager.Addr()),
							},
						},
					},
				},
			},
			Scheme:     "https",
			Timeout:    10000000000,
			APIVersion: prom_config.AlertmanagerAPIVersionV2,
			HTTPClientConfig: config.HTTPClientConfig{
				TLSConfig: config.TLSConfig{
					CAFile:     caCert,
					CertFile:   cert,
					KeyFile:    key,
					ServerName: "metric-store",
				},
			},
		}}
	})

	AfterEach(func() {
		for _, c := range cleanups {
			c()
		}
	})

	Describe("#Create", func() {
		It("creates a rule manager from a given file", func() {
			ruleManagers.Create("appmetrics", createRuleFile("appmetrics"), alertManagerConfigs)

			Expect(len(ruleManagers.RuleGroups())).To(Equal(1))
			Expect(len(ruleManagers.AlertingRules())).To(Equal(1))
			Eventually(func() int { return len(ruleManagers.Alertmanagers()) }, 10).Should(Equal(1))
		})
	})

	Describe("#Delete", func() {
		It("deletes a rule manager", func() {
			ruleManagers.Create("appmetrics", createRuleFile("appmetrics"), alertManagerConfigs)
			Expect(len(ruleManagers.RuleGroups())).To(Equal(1))
			Expect(len(ruleManagers.AlertingRules())).To(Equal(1))

			ruleManagers.Delete("appmetrics")

			Expect(len(ruleManagers.RuleGroups())).To(Equal(0))
			Expect(len(ruleManagers.AlertingRules())).To(Equal(0))
		})
	})

	Describe("#DeleteAll", func() {
		It("deletes all rule managers", func() {
			ruleManagers.Create("manager1", createRuleFile("manager1"), alertManagerConfigs)
			ruleManagers.Create("manager2", createRuleFile("manager2"), alertManagerConfigs)
			ruleManagers.Create("manager3", createRuleFile("manager3"), alertManagerConfigs)

			Expect(len(ruleManagers.RuleGroups())).To(Equal(3))
			Expect(len(ruleManagers.AlertingRules())).To(Equal(3))

			ruleManagers.DeleteAll()

			Expect(len(ruleManagers.RuleGroups())).To(Equal(0))
			Expect(len(ruleManagers.AlertingRules())).To(Equal(0))
		})
	})

	Describe("#Alertmanagers", func() {
		It("Returns unique alertmanagers", func() {
			ruleManagers.Create("manager1", createRuleFile("manager1"), alertManagerConfigs)
			ruleManagers.Create("manager2", createRuleFile("manager2"), alertManagerConfigs)

			Expect(len(ruleManagers.RuleGroups())).To(Equal(2))
			Expect(len(ruleManagers.AlertingRules())).To(Equal(2))
			Eventually(func() int { return len(ruleManagers.Alertmanagers()) }, 10).Should(Equal(1))
		})
	})
})
