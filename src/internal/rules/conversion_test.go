package rules_test

import (
	. "github.com/cloudfoundry/metric-store-release/src/internal/rules"
	prom_versioned_api_client "github.com/prometheus/client_golang/api/prometheus/v1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Conversion", func() {
	FDescribe("#ConvertAPIRuleGroupToGroup", func() {
		It("converts API RuleGroups to Groups", func() {
			apiRecordingRule := prom_versioned_api_client.RecordingRule{
				Name: "recording rule",
			}
			apiRuleGroup := prom_versioned_api_client.RuleGroup{
				Name:     "name",
				File:     "/foo/bar",
				Interval: 1,
				Rules: prom_versioned_api_client.Rules{
					apiRecordingRule,
				},
			}

			groups := ConvertAPIRuleGroupToGroup(
				[]prom_versioned_api_client.RuleGroup{apiRuleGroup},
			)

			Expect(len(groups)).To(Equal(1))

			group := groups[0]
			Expect(group.Name()).To(Equal("name"))
			Expect(group.File()).To(Equal("/foo/bar"))
			Expect(group.Interval().Seconds()).To(Equal(float64(1)))
			Expect(len(group.Rules())).To(Equal(1))

			rule := group.Rules()[0]
			Expect(rule.Name()).To(Equal("recording rule"))
		})
	})

	Describe("#ConvertAPIRulesToRule", func() {
		It("converts API Rules to Rules", func() {
			apiRecordingRule := prom_versioned_api_client.RecordingRule{
				Name: "recording rule",
			}

			rules := ConvertAPIRulesToRule(
				prom_versioned_api_client.Rules{apiRecordingRule},
			)

			Expect(len(rules)).To(Equal(1))

			rule := rules[0]
			Expect(rule.Name()).To(Equal("recording rule"))
		})
	})
})
