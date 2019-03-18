package transform

import (
	"fmt"
	"regexp"

	"github.com/influxdata/influxql"
	"github.com/prometheus/prometheus/pkg/labels"
)

var operatorMapping = map[labels.MatchType]influxql.Token{
	labels.MatchEqual:     influxql.EQ,
	labels.MatchNotEqual:  influxql.NEQ,
	labels.MatchRegexp:    influxql.EQREGEX,
	labels.MatchNotRegexp: influxql.NEQREGEX,
}

func ToInfluxFilter(matcher *labels.Matcher) (*influxql.BinaryExpr, error) {
	operator, ok := operatorMapping[matcher.Type]
	if !ok {
		return nil, fmt.Errorf("Unsupported operation: %d", matcher.Type)
	}

	var expected influxql.Expr

	switch operator {
	case influxql.EQ, influxql.NEQ:
		expected = &influxql.StringLiteral{Val: matcher.Value}
	case influxql.EQREGEX, influxql.NEQREGEX:
		regex, err := regexp.Compile(matcher.Value)
		if err != nil {
			return nil, fmt.Errorf("Invalid regular expression: %s", err)
		}
		expected = &influxql.RegexLiteral{Val: regex}
	default:
		return nil, fmt.Errorf("Unsupported operation: %d", matcher.Type)
	}

	influxFilter := influxql.BinaryExpr{
		LHS: &influxql.VarRef{Val: matcher.Name},
		RHS: expected,
		Op:  operator,
	}

	return &influxFilter, nil
}

func ToInfluxFilters(matchers []*labels.Matcher) (influxql.Expr, error) {
	var filterCondition influxql.Expr

	for _, matcher := range matchers {
		newFilterCondition, err := ToInfluxFilter(matcher)
		if err != nil {
			return nil, err
		}

		if filterCondition == nil {
			filterCondition = newFilterCondition
			continue
		}

		filterCondition = &influxql.BinaryExpr{
			LHS: newFilterCondition,
			RHS: filterCondition,
			Op:  influxql.AND,
		}
	}

	return filterCondition, nil
}
