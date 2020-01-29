package rulesclient_test

import (
	"time"

	. "github.com/cloudfoundry/metric-store-release/src/pkg/rulesclient"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/pkg/rulefmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("RuleGroup", func() {
	Describe("Validate()", func() {
		It("returns nil for valid rule groups", func() {
			group := RuleGroup{
				Name:     "foo",
				Interval: Duration(1 * time.Minute),
				Rules: []Rule{{
					Record: "cpuRule",
					Expr:   "cpu",
				}},
			}

			err := group.Validate()
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns error when name is missing", func() {
			group := RuleGroup{
				Name: "",
				Rules: []Rule{{
					Record: "cpuRule",
					Expr:   "cpu",
				}},
			}

			err := group.Validate()
			Expect(err).To(HaveOccurred())
		})

		It("returns error when there are no rules", func() {
			group := RuleGroup{
				Name:  "foo",
				Rules: []Rule{},
			}

			err := group.Validate()
			Expect(err).To(HaveOccurred())
		})

		It("returns error when any rule group is invalid", func() {
			group := RuleGroup{
				Name: "foo",
				Rules: []Rule{
					{},
				},
			}

			err := group.Validate()
			Expect(err).To(HaveOccurred())
		})

		It("returns an error when the interval is less than 1 minute", func() {
			group := RuleGroup{
				Name:     "foo",
				Interval: Duration(1 * time.Second),
				Rules: []Rule{{
					Record: "cpuRule",
					Expr:   "cpu",
				}},
			}

			err := group.Validate()
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("ConvertToPromRuleGroup()", func() {
		It("returns a prom RuleGroup", func() {
			clientRuleGroup := RuleGroup{
				Name:     "foo",
				Interval: Duration(1 * time.Minute),
				Rules: []Rule{{
					Record: "cpuRule",
					Expr:   "cpu",
				}},
			}

			promRuleGroup := rulefmt.RuleGroup{
				Name:     "foo",
				Interval: model.Duration(1 * time.Minute),
				Rules: []rulefmt.Rule{{
					Record: "cpuRule",
					Expr:   "cpu",
				}},
			}

			result, _ := clientRuleGroup.ConvertToPromRuleGroup()
			Expect(*result).To(Equal(promRuleGroup))
		})

		It("returns an error if a rule's for can't be parsed", func() {
			clientRuleGroup := RuleGroup{
				Name:     "foo",
				Interval: Duration(1 * time.Minute),
				Rules: []Rule{{
					Record: "cpuRule",
					Expr:   "cpu",
					For:    "not-a-duration",
				}},
			}

			_, err := clientRuleGroup.ConvertToPromRuleGroup()
			Expect(err).To(HaveOccurred())
		})
	})
})
