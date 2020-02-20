package rules_test

import (
	"net/http"
	"net/url"

	. "github.com/cloudfoundry/metric-store-release/src/internal/rules"
	"github.com/cloudfoundry/metric-store-release/src/pkg/rulesclient"

	"github.com/cloudfoundry/metric-store-release/src/internal/testing"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Rules", func() {
	Describe("#CreateManager", func() {
		It("creates a manager on respective nodes, regardless if one fails", func() {
			localRuleManager := testing.NewRuleManagerSpy()
			localRuleManager.CreateManager("app-metrics", "")

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
			err = ruleManager.CreateManager("app-metrics", "")
			Expect(err).To(HaveOccurred())

			Expect(localRuleManager.MethodsCalled()).To(ContainElement("CreateManager"))
			Expect(spyRulesApi.LastRequestPath()).To(Equal("/private/rules/manager"))
		})

		It("happy path", func() {
			localRuleManager := testing.NewRuleManagerSpy()

			ruleManager := NewReplicatedRuleManager(localRuleManager, 0, []string{"localhost:6060"}, 1, nil)
			err := ruleManager.CreateManager("app-metrics", "")
			Expect(err).ToNot(HaveOccurred())
			Expect(localRuleManager.MethodsCalled()).To(ContainElement("CreateManager"))
		})
	})

	Describe("#DeleteManager", func() {
		It("happy path", func() {
			localRuleManager := testing.NewRuleManagerSpy()
			localRuleManager.CreateManager("app-metrics", "")

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
			localRuleManager.CreateManager("app-metrics", "")

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
		It("returns unique alertmanagers from all nodes", func() {
			localRuleManager := testing.NewRuleManagerSpy()
			localRuleManager.CreateManager("app-metrics", "localhost:8080")
			localRuleManager.CreateManager("healthwatch", "localhost:8080")

			ruleManager := NewReplicatedRuleManager(localRuleManager, 0, []string{"localhost:6060"}, 1, nil)
			alertmanagers := ruleManager.Alertmanagers()

			Expect(len(alertmanagers)).To(Equal(1))
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
