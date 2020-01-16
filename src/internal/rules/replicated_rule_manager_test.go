package rules_test

import (
	"net/url"

	. "github.com/cloudfoundry/metric-store-release/src/internal/rules"

	"github.com/cloudfoundry/metric-store-release/src/internal/testing"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Rules", func() {
	Describe("#AlertManagers", func() {
		It("returns unique alertmanagers from all nodes", func() {
			localRuleManager := testing.NewRuleManagerSpy()
			localRuleManager.Create("app-metrics", "localhost:8080")
			localRuleManager.Create("healthwatch", "localhost:8080")

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
