package rules_test

import (
	. "github.com/cloudfoundry/metric-store-release/src/internal/rules"
	prom_versioned_api_client "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/pkg/labels"
	prom_rules "github.com/prometheus/prometheus/rules"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = FDescribe("Conversion", func() {
	Describe("#ConvertAPIRuleGroupToGroup", func() {
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
		It("converts API Recording Rules to Rules", func() {
			apiRecordingRule := prom_versioned_api_client.RecordingRule{
				Name:      "recording rule",
				Query:     "1+2",
				Labels:    map[model.LabelName]model.LabelValue{"labelKey": "labelValue"},
				Health:    prom_versioned_api_client.RuleHealthGood,
				LastError: "oops",
			}

			rules := ConvertAPIRulesToRule(
				prom_versioned_api_client.Rules{apiRecordingRule},
			)

			Expect(len(rules)).To(Equal(1))

			rule := rules[0]
			Expect(rule).To(BeAssignableToTypeOf(&prom_rules.RecordingRule{}))
			Expect(rule.Name()).To(Equal("recording rule"))
			Expect(rule.(*prom_rules.RecordingRule).Query().String()).To(Equal("1 + 2"))
			Expect(rule.Labels()).To(Equal(labels.FromMap(map[string]string{"labelKey": "labelValue"})))
			Expect(rule.Health()).To(Equal(prom_rules.HealthGood))
			Expect(rule.LastError()).To(MatchError("oops"))
		})
		It("converts API Alerting Rules to Rules", func() {
			apiAlertingRule := prom_versioned_api_client.AlertingRule{
				Name: "alerting rule",
			}

			rules := ConvertAPIRulesToRule(
				prom_versioned_api_client.Rules{apiAlertingRule},
			)

			Expect(len(rules)).To(Equal(1))

			rule := rules[0]
			Expect(rule).To(BeAssignableToTypeOf(&prom_rules.AlertingRule{}))
			Expect(rule.Name()).To(Equal("alerting rule"))
		})
	})
})
