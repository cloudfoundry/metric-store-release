package rules_test

import (
	"net/url"
	"time"

	. "github.com/cloudfoundry/metric-store-release/src/internal/rules"
	"github.com/cloudfoundry/metric-store-release/src/internal/testing"
	"github.com/cloudfoundry/metric-store-release/src/pkg/logger"
	"github.com/prometheus/common/model"
	prom_config "github.com/prometheus/prometheus/config"
	sd_config "github.com/prometheus/prometheus/discovery/config"
	"github.com/prometheus/prometheus/discovery/targetgroup"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("LocalRuleManager", func() {
	Describe("CreateManager", func() {
		It("creates a populated rule manager", func() {
			tempStorage := testing.NewTempStorage()
			defer tempStorage.Cleanup()

			spyPromRuleManagers := NewRuleManagers(
				nil,
				nil,
				time.Millisecond,
				logger.NewTestLogger(GinkgoWriter),
				testing.NewSpyMetricRegistrar(),
				time.Millisecond,
			)
			localRuleManager := NewLocalRuleManager(tempStorage.Path(), spyPromRuleManagers)

			alertManagers := &prom_config.AlertmanagerConfigs{{
				ServiceDiscoveryConfig: sd_config.ServiceDiscoveryConfig{
					StaticConfigs: []*targetgroup.Group{
						{
							Targets: []model.LabelSet{
								{
									"__address__": "127.0.0.1:1234",
									"__scheme__":  "https",
								},
							},
						},
					},
				},
				Timeout:    10000000000,
				APIVersion: prom_config.AlertmanagerAPIVersionV2,
			}}
			Expect(localRuleManager.CreateManager("createTest", alertManagers)).To(Succeed())

			alertUrl, _ := url.Parse("https://127.0.0.1:1234/api/v2/alerts")
			Eventually(localRuleManager.Alertmanagers, 10).Should(ConsistOf([]*url.URL{alertUrl}))
			Expect(tempStorage.Directories()).To(ConsistOf("createTest"))
		})
	})

	Describe("DeleteManager", func() {
		It("deletes an existing rule manager", func() {
			tempStorage := testing.NewTempStorage()
			defer tempStorage.Cleanup()

			spyPromRuleManagers := testing.NewPromRuleManagersSpy()
			localRuleManager := NewLocalRuleManager(tempStorage.Path(), spyPromRuleManagers)

			Expect(localRuleManager.CreateManager("app-metrics", nil)).To(Succeed())
			Expect(spyPromRuleManagers.ManagerIds()).To(ConsistOf("app-metrics"))
			Expect(tempStorage.Directories()).To(ConsistOf("app-metrics"))

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

			Expect(localRuleManager.CreateManager("app-metrics", nil)).To(Succeed())
			Expect(spyPromRuleManagers.ManagerIds()).To(ConsistOf("app-metrics"))

			spyPromRuleManagers.Delete("app-metrics")
			Expect(localRuleManager.DeleteManager("app-metrics")).ToNot(Succeed())
		})
	})
})
