package rules_test

import (
	"net/http"
	"net/url"

	. "github.com/cloudfoundry/metric-store-release/src/internal/rules"
	shared "github.com/cloudfoundry/metric-store-release/src/internal/testing"
	sharedtls "github.com/cloudfoundry/metric-store-release/src/internal/tls"
	"github.com/cloudfoundry/metric-store-release/src/pkg/rulesclient"
	"github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	prom_config "github.com/prometheus/prometheus/config"
	sd_config "github.com/prometheus/prometheus/discovery/config"
	"github.com/prometheus/prometheus/discovery/targetgroup"

	"github.com/cloudfoundry/metric-store-release/src/internal/testing"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Rules", func() {
	Describe("#CreateManager", func() {
		It("happy path", func() {
			localRuleManager := testing.NewRuleManagerSpy()

			ruleManager := NewReplicatedRuleManager(localRuleManager, 0, []string{"localhost:6060"}, 1, nil)
			_, err := ruleManager.CreateManager("app-metrics", nil)
			Expect(err).ToNot(HaveOccurred())
			Expect(localRuleManager.MethodsCalled()).To(ContainElement("CreateManager"))
		})

		It("creates a manager on respective nodes, regardless if one fails", func() {
			localRuleManager := testing.NewRuleManagerSpy()
			localRuleManager.CreateManager("app-metrics", nil)

			tlsServerConfig := testing.MutualTLSServerConfig()
			tlsClientConfig := testing.MutualTLSClientConfig()

			spyRulesApi, err := testing.NewRulesApiSpy(tlsServerConfig)
			Expect(err).ToNot(HaveOccurred())
			spyRulesApi.Start()
			defer spyRulesApi.Stop()
			spyRulesApi.NextRequestError(&testing.RulesApiHttpError{
				Status: http.StatusInternalServerError,
				Title:  "error",
			})

			ruleManager := NewReplicatedRuleManager(localRuleManager, 0, []string{"localhost:6060", spyRulesApi.Addr(), "url-not-used"}, 2, tlsClientConfig)
			_, err = ruleManager.CreateManager("app-metrics", nil)
			Expect(err).To(HaveOccurred())

			Expect(localRuleManager.MethodsCalled()).To(ContainElement("CreateManager"))
			Expect(spyRulesApi.LastRequestPath()).To(Equal("/private/rules/manager"))
		})
	})

	Describe("#DeleteManager", func() {
		It("happy path", func() {
			localRuleManager := testing.NewRuleManagerSpy()
			localRuleManager.CreateManager("app-metrics", nil)

			ruleManager := NewReplicatedRuleManager(localRuleManager, 0, []string{"localhost:6060"}, 1, nil)
			err := ruleManager.DeleteManager("app-metrics")

			Expect(err).ToNot(HaveOccurred())
			Expect(localRuleManager.MethodsCalled()).To(ContainElement("DeleteManager"))
		})

		It("deletes a manager on respective nodes, regardless if one fails", func() {
			localRuleManager := testing.NewRuleManagerSpy()
			tlsServerConfig := testing.MutualTLSServerConfig()
			tlsClientConfig := testing.MutualTLSClientConfig()

			spyRulesApi, err := testing.NewRulesApiSpy(tlsServerConfig)
			Expect(err).ToNot(HaveOccurred())
			spyRulesApi.Start()
			defer spyRulesApi.Stop()
			spyRulesApi.NextRequestError(&testing.RulesApiHttpError{
				Status: http.StatusInternalServerError,
				Title:  "error",
			})

			ruleManager := NewReplicatedRuleManager(localRuleManager, 0, []string{"localhost:6060", spyRulesApi.Addr(), "url-not-used"}, 2, tlsClientConfig)

			err = ruleManager.DeleteManager("app-metrics")
			Expect(err).To(HaveOccurred())
			Expect(localRuleManager.MethodsCalled()).To(ContainElement("DeleteManager"))
			Expect(spyRulesApi.LastRequestPath()).To(Equal("/private/rules/manager/app-metrics"))
		})
	})

	Describe("#UpsertRuleGroup", func() {
		It("happy path", func() {
			localRuleManager := testing.NewRuleManagerSpy()
			localRuleManager.CreateManager("app-metrics", nil)

			ruleManager := NewReplicatedRuleManager(localRuleManager, 0, []string{"localhost:6060"}, 1, nil)
			err := ruleManager.UpsertRuleGroup("app-metrics", &rulesclient.RuleGroup{
				Name: "app-metrics-rule-group",
				Rules: []rulesclient.Rule{
					{
						Record: "cpu_count",
						Expr:   "cpu",
					},
				},
			})

			Expect(err).ToNot(HaveOccurred())
			Expect(localRuleManager.MethodsCalled()).To(ContainElement("UpsertRuleGroup"))
		})

		It("upserts a rule group on respective nodes, regardless if one fails", func() {
			localRuleManager := testing.NewRuleManagerSpy()
			tlsServerConfig := testing.MutualTLSServerConfig()
			tlsClientConfig := testing.MutualTLSClientConfig()

			spyRulesApi, err := testing.NewRulesApiSpy(tlsServerConfig)
			Expect(err).ToNot(HaveOccurred())
			spyRulesApi.Start()
			defer spyRulesApi.Stop()
			spyRulesApi.NextRequestError(&testing.RulesApiHttpError{
				Status: http.StatusInternalServerError,
				Title:  "error",
			})

			ruleManager := NewReplicatedRuleManager(localRuleManager, 0, []string{"localhost:6060", spyRulesApi.Addr(), "url-not-used"}, 2, tlsClientConfig)

			err = ruleManager.UpsertRuleGroup("app-metrics", &rulesclient.RuleGroup{
				Name: "app-metrics-rule-group",
				Rules: []rulesclient.Rule{
					{
						Record: "cpu_count",
						Expr:   "cpu",
					},
				},
			})
			Expect(err).To(HaveOccurred())
			Expect(localRuleManager.MethodsCalled()).To(ContainElement("UpsertRuleGroup"))
			Expect(spyRulesApi.LastRequestPath()).To(Equal("/private/rules/manager/app-metrics/group"))
		})
	})

	Describe("#AlertManagers", func() {
		It("returns unique alertmanagers from all tenants/rule managers", func() {
			caCert := shared.Cert("metric-store-ca.crt")
			cert := shared.Cert("metric-store.crt")
			key := shared.Cert("metric-store.key")
			tlsConfig, err := sharedtls.NewMutualTLSClientConfig(caCert, cert, key, "metric-store")
			Expect(err).ToNot(HaveOccurred())

			alertSpy := testing.NewAlertManagerSpy(tlsConfig)
			alertSpy.Start()
			defer alertSpy.Stop()

			alertManagerConfigs := &prom_config.AlertmanagerConfigs{{
				ServiceDiscoveryConfig: sd_config.ServiceDiscoveryConfig{
					StaticConfigs: []*targetgroup.Group{
						{
							Targets: []model.LabelSet{
								{
									"__address__": model.LabelValue(alertSpy.Addr()),
									"__scheme__":  "https",
								},
							},
						},
					},
				},
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
			localRuleManager := testing.NewRuleManagerSpy()
			_, err = localRuleManager.CreateManager("app-metrics", alertManagerConfigs)
			Expect(err).ToNot(HaveOccurred())
			_, err = localRuleManager.CreateManager("healthwatch", alertManagerConfigs)
			Expect(err).ToNot(HaveOccurred())

			ruleManager := NewReplicatedRuleManager(localRuleManager, 0, []string{"localhost:6060"}, 1, nil)
			Expect(ruleManager.Alertmanagers()).To(HaveLen(1))
		})
	})

	Describe("#DroppedAlertmanagers", func() {
		It("returns unique dropped alertmanagers from all nodes", func() {
			localRuleManager := testing.NewRuleManagerSpy()
			alertmanagerUrl, err := url.Parse("localhost:8080")
			Expect(err).ToNot(HaveOccurred())
			localRuleManager.AddDroppedAlertmanagers([]*url.URL{alertmanagerUrl, alertmanagerUrl})

			ruleManager := NewReplicatedRuleManager(localRuleManager, 0, []string{"localhost:6060"}, 1, nil)
			droppedAlertmanagers := ruleManager.DroppedAlertmanagers()

			Expect(len(droppedAlertmanagers)).To(Equal(1))
		})
	})
})
