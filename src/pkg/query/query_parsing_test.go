package query_test

import (
	"github.com/cloudfoundry/metric-store-release/src/pkg/query"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Query Parser", func() {
	Describe("ExtractSourceIds()", func() {
		It("returns all the sourceIds from a query", func() {
			qp := &query.QueryParser{}
			query := `metric{source_id="source-0"}+metric{source_id="source-1"}`

			ids, err := qp.ExtractSourceIds(query)

			Expect(err).ToNot(HaveOccurred())
			Expect(ids).To(ConsistOf("source-0", "source-1"))

		})

		It("returns an error if a query is not valid", func() {
			qp := &query.QueryParser{}
			query := `metric=what`

			ids, err := qp.ExtractSourceIds(query)

			Expect(err).To(HaveOccurred())
			Expect(ids).To(BeEmpty())
		})

		It("returns an error if a query term lacks a sourceId", func() {
			qp := &query.QueryParser{}
			query := `metric{source_id="source-0"}+metric`

			ids, err := qp.ExtractSourceIds(query)

			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("one or more terms lack a sourceId"))
			Expect(ids).To(BeEmpty())
		})

		It("returns an error if a query term has an empty sourceId", func() {
			qp := &query.QueryParser{}
			query := `metric{source_id=""}`

			ids, err := qp.ExtractSourceIds(query)

			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("one or more terms lack a sourceId"))
			Expect(ids).To(BeEmpty())
		})
	})
})
