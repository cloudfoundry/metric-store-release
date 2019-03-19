package local_test

import (
	"context"
	"io/ioutil"
	"log"

	"github.com/cloudfoundry/metric-store-release/src/pkg/local"
	rpc "github.com/cloudfoundry/metric-store-release/src/pkg/rpc/metricstore_v1"

	"github.com/cloudfoundry/metric-store-release/src/pkg/testing"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("EgressReverseProxy", func() {
	Describe("InstantQuery()", func() {
		It("hands the local reader to prometheus query engine", func() {
			tc := egressReverseProxySetup()

			req := &rpc.PromQL_InstantQueryRequest{Query: "metric_name_1 + metric_name_2", Time: "1"}
			_, err := tc.erp.InstantQuery(context.Background(), req)
			Expect(err).ToNot(HaveOccurred())

			Expect(tc.engine.InstantQueryDataReader).To(Equal(tc.spyLocalDataReader))
		})

		It("returns an error if the local node fails", func() {
			tc := egressReverseProxySetup()

			req := &rpc.PromQL_InstantQueryRequest{Query: "metric_name_1 + metric_name_2", Time: "1"}
			tc.engine.RespondWithError = true

			_, err := tc.erp.InstantQuery(context.Background(), req)
			Expect(err).To(HaveOccurred())

			Expect(tc.engine.InstantQueryDataReader).To(Equal(tc.spyLocalDataReader))
		})
	})

	Describe("RangeQuery()", func() {
		Context("given a query where all metric_names are on one node", func() {
			It("hands the local reader to prometheus query engine", func() {
				tc := egressReverseProxySetup()

				req := &rpc.PromQL_RangeQueryRequest{
					Query: "metric_name_1 + metric_name_2",
					Start: "1",
					End:   "10",
					Step:  "1",
				}
				_, err := tc.erp.RangeQuery(context.Background(), req)
				Expect(err).ToNot(HaveOccurred())

				Expect(tc.engine.RangeQueryDataReader).To(Equal(tc.spyLocalDataReader))
			})

			It("returns an error if the local node fails", func() {
				tc := egressReverseProxySetup()

				req := &rpc.PromQL_RangeQueryRequest{
					Query: "metric_name_1 + metric_name_2",
					Start: "1",
					End:   "4",
					Step:  "1",
				}
				tc.engine.RespondWithError = true

				_, err := tc.erp.RangeQuery(context.Background(), req)
				Expect(err).To(HaveOccurred())

				Expect(tc.engine.RangeQueryDataReader).To(Equal(tc.spyLocalDataReader))
			})
		})
	})

	Describe("SeriesQuery()", func() {
		It("hands the local reader to prometheus query engine", func() {
			tc := egressReverseProxySetup()

			req := &rpc.PromQL_SeriesQueryRequest{Match: []string{"metric_name_1", "metric_name_2"}, Start: "1", End: "2"}
			_, err := tc.erp.SeriesQuery(context.Background(), req)
			Expect(err).ToNot(HaveOccurred())

			Expect(tc.engine.SeriesQueryDataReader).To(Equal(tc.spyLocalDataReader))
		})

		It("returns an error if the local node fails", func() {
			tc := egressReverseProxySetup()

			req := &rpc.PromQL_SeriesQueryRequest{Match: []string{"metric_name_1", "metric_name_2"}, Start: "1", End: "2"}
			tc.engine.RespondWithError = true

			_, err := tc.erp.SeriesQuery(context.Background(), req)
			Expect(err).To(HaveOccurred())

			Expect(tc.engine.SeriesQueryDataReader).To(Equal(tc.spyLocalDataReader))
		})
	})

	Context("LabelsQuery()", func() {
		It("returns all labels and prepends __name__ for promql compatibility", func() {
			tc := egressReverseProxySetup()

			tc.spyLocalDataReader.LabelsResponse = &rpc.PromQL_LabelsQueryResult{
				Labels: []string{"foo", "bar", "baz"},
			}

			result, err := tc.erp.LabelsQuery(
				context.Background(),
				&rpc.PromQL_LabelsQueryRequest{},
			)

			Expect(err).ToNot(HaveOccurred())
			Expect(result.Labels).To(ConsistOf([]string{"__name__", "foo", "bar", "baz"}))
		})
	})

	Context("LabelValues()", func() {
		It("returns label values", func() {
			tc := egressReverseProxySetup()

			tc.spyLocalDataReader.LabelValuesResponse = &rpc.PromQL_LabelValuesQueryResult{
				Values: []string{"100", "10"},
			}

			result, err := tc.erp.LabelValuesQuery(
				context.Background(),
				&rpc.PromQL_LabelValuesQueryRequest{Name: "label-name"},
			)

			Expect(err).ToNot(HaveOccurred())
			Expect(result.Values).To(ConsistOf([]string{"100", "10"}))
		})
	})

})

type egressReverseProxyTestContext struct {
	erp                *local.EgressReverseProxy
	engine             *testing.SpyQueryEngine
	spyLocalDataReader *testing.SpyDataReader
}

func egressReverseProxySetup() *egressReverseProxyTestContext {
	spyEngine := testing.NewSpyQueryEngine()

	localReader := testing.NewSpyDataReader()

	erp := local.NewEgressReverseProxy(
		localReader,
		spyEngine,
		local.WithLogger(log.New(ioutil.Discard, "", 0)),
	)

	return &egressReverseProxyTestContext{
		erp:                erp,
		engine:             spyEngine,
		spyLocalDataReader: localReader,
	}
}
