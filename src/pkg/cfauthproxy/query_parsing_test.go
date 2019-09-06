package cfauthproxy_test

import (
	"github.com/cloudfoundry/metric-store-release/src/pkg/cfauthproxy"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Query Parser", func() {
	Describe("ExtractSourceIds()", func() {
		It("returns all the sourceIds from a query", func() {
			qp := &cfauthproxy.QueryParser{}
			query := `metric{source_id="source-0"}+metric{source_id="source-1"}`

			ids, err := qp.ExtractSourceIds(query)

			Expect(err).ToNot(HaveOccurred())
			Expect(ids).To(ConsistOf("source-0", "source-1"))

		})

		It("returns an error if a query is not valid", func() {
			qp := &cfauthproxy.QueryParser{}
			query := `metric=what`

			ids, err := qp.ExtractSourceIds(query)

			Expect(err).To(HaveOccurred())
			Expect(ids).To(BeEmpty())
		})

		It("returns an error if a query term lacks a sourceId", func() {
			qp := &cfauthproxy.QueryParser{}
			query := `metric{source_id="source-0"}+metric`

			ids, err := qp.ExtractSourceIds(query)

			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("one or more terms lack a sourceId"))
			Expect(ids).To(BeEmpty())
		})

		It("returns an error if a query term has an empty sourceId", func() {
			qp := &cfauthproxy.QueryParser{}
			query := `metric{source_id=""}`

			ids, err := qp.ExtractSourceIds(query)

			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("one or more terms lack a sourceId"))
			Expect(ids).To(BeEmpty())
		})

		It("returns an error if a query term has a regex match on source id", func() {
			qp := &cfauthproxy.QueryParser{}
			query := `cpu{source_id=~"platform"}`

			ids, err := qp.ExtractSourceIds(query)

			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("regular expressions are unavailable on source ids"))
			Expect(ids).To(BeEmpty())
		})
	})
})
