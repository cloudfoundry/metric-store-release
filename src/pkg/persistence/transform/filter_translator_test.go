package transform_test

import (
	"github.com/cloudfoundry/metric-store-release/src/pkg/persistence/transform"
	"github.com/influxdata/influxql"
	"github.com/prometheus/prometheus/pkg/labels"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
)

var _ = Describe("Filter Translator", func() {
	Context("ToInfluxFilter()", func() {
		It("maps the prometheus matcher to its InfluxQL counterpart", func() {
			matcher := labels.Matcher{
				Type:  labels.MatchEqual,
				Name:  "foo",
				Value: "bar",
			}

			influxFilter, err := transform.ToInfluxFilter(&matcher)
			Expect(err).ToNot(HaveOccurred())

			Expect(influxFilter).To(PointTo(Equal(influxql.BinaryExpr{
				LHS: &influxql.VarRef{Val: "foo"},
				RHS: &influxql.StringLiteral{Val: "bar"},
				Op:  influxql.EQ,
			})))
		})

		DescribeTable("maps the comparator properly", func(in labels.MatchType, out influxql.Token) {
			influxFilter, err := transform.ToInfluxFilter(&labels.Matcher{Type: in})
			Expect(err).ToNot(HaveOccurred())
			Expect(influxFilter.Op).To(Equal(out))
		},
			Entry("Equals", labels.MatchEqual, influxql.EQ),
			Entry("Not Equals", labels.MatchNotEqual, influxql.NEQ),
			Entry("Regular Expression", labels.MatchRegexp, influxql.EQREGEX),
			Entry("Not Regular Expression", labels.MatchNotRegexp, influxql.NEQREGEX),
		)

		It("returns an error when handed an invalid operator", func() {
			matcher := labels.Matcher{
				Type:  99,
				Name:  "foo",
				Value: "bar",
			}

			influxFilter, err := transform.ToInfluxFilter(&matcher)
			Expect(err).To(HaveOccurred())
			Expect(influxFilter).To(BeNil())
		})

		It("returns an error when regex is invalid", func() {
			matcher := labels.Matcher{
				Type:  labels.MatchRegexp,
				Name:  "foo",
				Value: "[",
			}

			influxFilter, err := transform.ToInfluxFilter(&matcher)
			Expect(err).To(HaveOccurred())
			Expect(influxFilter).To(BeNil())
		})
	})

	Context("ToInfluxFilters()", func() {
		It("maps a list of prometheus matchers to an InfluxQL filter tree", func() {
			matchers := []*labels.Matcher{{
				Type:  labels.MatchNotEqual,
				Name:  "foo",
				Value: "bar",
			}, {
				Type:  labels.MatchEqual,
				Name:  "baz",
				Value: "faz",
			}}

			influxFilters, err := transform.ToInfluxFilters(matchers)
			Expect(err).ToNot(HaveOccurred())

			Expect(influxFilters).To(PointTo(Equal(influxql.BinaryExpr{
				LHS: &influxql.BinaryExpr{
					LHS: &influxql.VarRef{Val: "baz"},
					RHS: &influxql.StringLiteral{Val: "faz"},
					Op:  influxql.EQ,
				},
				RHS: &influxql.BinaryExpr{
					LHS: &influxql.VarRef{Val: "foo"},
					RHS: &influxql.StringLiteral{Val: "bar"},
					Op:  influxql.NEQ,
				},
				Op: influxql.AND,
			})))
		})

		It("tolerates receiving empty filter list", func() {
			matchers := []*labels.Matcher{}
			influxFilters, err := transform.ToInfluxFilters(matchers)
			Expect(err).ToNot(HaveOccurred())
			Expect(influxFilters).To(BeNil())
		})

		It("returns an error if any of the matchers' operators are unsupported", func() {
			matchers := []*labels.Matcher{{
				Type:  99,
				Name:  "foo",
				Value: "bar",
			}, {
				Type:  labels.MatchEqual,
				Name:  "baz",
				Value: "faz",
			}}

			influxFilters, err := transform.ToInfluxFilters(matchers)
			Expect(err).To(HaveOccurred())
			Expect(influxFilters).To(BeNil())
		})
	})
})
