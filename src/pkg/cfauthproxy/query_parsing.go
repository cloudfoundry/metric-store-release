package cfauthproxy

import (
	"errors"

	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/promql/parser"
)

type QueryParser struct{}

func (q *QueryParser) ExtractSourceIds(query string) ([]string, error) {
	expr, err := parser.ParseExpr(query)
	if err != nil {
		return nil, err
	}

	visitor := newSourceIdVisitor()

	err = parser.Walk(
		visitor,
		expr,
		nil,
	)
	if err != nil {
		return nil, err
	}

	var sourceIds []string

	for sourceId := range visitor.sourceIds {
		sourceIds = append(sourceIds, sourceId)
	}

	return sourceIds, nil
}

type sourceIdVisitor struct {
	sourceIds map[string]struct{}
}

func newSourceIdVisitor() *sourceIdVisitor {
	return &sourceIdVisitor{
		sourceIds: make(map[string]struct{}),
	}
}

func (s *sourceIdVisitor) Visit(node parser.Node, _ []parser.Node) (parser.Visitor, error) {
	if node == nil {
		return nil, nil
	}

	var err error
	switch selector := node.(type) {
	case *parser.VectorSelector:
		err = s.addSourceIdsFromMatchers(selector.LabelMatchers)
	case *parser.MatrixSelector:
		err = s.addSourceIdsFromMatchers(selector.VectorSelector.(*parser.VectorSelector).LabelMatchers)
	}

	if err != nil {
		return nil, err
	}

	return s, nil
}

func (s *sourceIdVisitor) addSourceIdsFromMatchers(labelMatchers []*labels.Matcher) error {
	containsValidSourceId := false
	for _, labelMatcher := range labelMatchers {
		if labelMatcher.Name == "source_id" && labelMatcher.Value != "" {
			containsValidSourceId = true
			err := addSourceIdsFromLabelMatcher(s.sourceIds, labelMatcher)
			if err != nil {
				return err
			}
		}
	}

	if !containsValidSourceId {
		return errors.New("one or more terms lack a sourceId")
	}

	return nil
}

func addSourceIdsFromLabelMatcher(sourceIds map[string]struct{}, labelMatcher *labels.Matcher) error {
	switch labelMatcher.Type {
	case labels.MatchRegexp, labels.MatchNotRegexp, labels.MatchNotEqual:
		return errors.New("only strict equality is allowed on source id")
	case labels.MatchEqual:
		sourceIds[labelMatcher.Value] = struct{}{}
	}
	return nil
}
