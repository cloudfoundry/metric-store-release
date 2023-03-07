package persistence_test

import (
	"errors"
	"fmt"
	"github.com/cloudfoundry/metric-store-release/src/pkg/persistence"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/storage"
)

var _ = Describe("Querier", func() {
	// Many of the tests for querier are in store_test.go
	Describe("Select()", func() {
		DescribeTable(
			"returns an error if given a query that uses a matcher other than = on __name__",
			func(in []*labels.Matcher, out error) {

				querier := persistence.NewQuerier(nil, nil, nil)
				Expect(querier.Select(false, nil, in...).Err()).To(Equal(out))
			},
			Entry("!= on __name__", []*labels.Matcher{{
				Name:  "__name__",
				Type:  labels.MatchNotEqual,
				Value: "irrelevantapp",
			}}, errors.New("only strict equality is supported for metric names")),
			Entry("=~ on __name__", []*labels.Matcher{{
				Name:  "__name__",
				Type:  labels.MatchRegexp,
				Value: "irrelevantapp",
			}}, errors.New("only strict equality is supported for metric names")),
			Entry("!~ on __name__", []*labels.Matcher{{
				Name:  "__name__",
				Type:  labels.MatchNotRegexp,
				Value: "irrelevantapp",
			}}, errors.New("only strict equality is supported for metric names")),
		)

		DescribeTable(
			"returns an error if start date is after the end date",
			func(params *storage.SelectHints, out error) {

				querier := persistence.NewQuerier(nil, nil, nil)

				Expect(querier.Select(false, params,
					nil).Err()).To(Equal(out))
			},
			Entry("start > end", &storage.SelectHints{
				Start: 200000,
				End:   100000,
			}, fmt.Errorf("Start (%d) must be before End (%d)", 200000, 100000)),
		)

	})
})
