package gateway_test

import (
	"crypto/tls"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"

	. "github.com/cloudfoundry/metric-store-release/src/pkg/gateway"
	internal_tls "github.com/cloudfoundry/metric-store-release/src/pkg/tls"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/cloudfoundry/metric-store-release/src/pkg/testing"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("Gateway", func() {
	var (
		spyMetricStore *testing.SpyMetricStore
		gw             *Gateway
	)

	BeforeEach(func() {
		tlsConfig, err := internal_tls.NewMutualTLSConfig(
			testing.Cert("metric-store-ca.crt"),
			testing.Cert("metric-store.crt"),
			testing.Cert("metric-store.key"),
			"metric-store",
		)
		Expect(err).ToNot(HaveOccurred())

		spyMetricStore = testing.NewSpyMetricStore(tlsConfig)
		metricStoreAddr := spyMetricStore.Start()

		gw = NewGateway(
			metricStoreAddr,
			"127.0.0.1:0",
			testing.Cert("localhost.crt"),
			testing.Cert("localhost.key"),
			WithGatewayMetricStoreDialOpts(
				grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)),
			),
		)
		gw.Start()
	})

	It("upgrades HTTPS requests for instant queries via PromQLAPI GETs into gRPC requests", func() {
		path := `api/v1/query?query=metric{source_id="some-id"}&time=1234.000`
		URL := fmt.Sprintf("%s/%s", gw.Addr(), path)
		resp, err := makeTLSReq("https", URL)
		Expect(err).ToNot(HaveOccurred())
		Expect(resp.StatusCode).To(Equal(http.StatusOK))

		reqs := spyMetricStore.GetQueryRequests()
		Expect(reqs).To(HaveLen(1))
		Expect(reqs[0].Query).To(Equal(`metric{source_id="some-id"}`))
		Expect(reqs[0].Time).To(Equal("1234.000"))
	})

	It("upgrades HTTPS requests for range queries via PromQLAPI GETs into gRPC requests", func() {
		path := `api/v1/query_range?query=metric{source_id="some-id"}&start=1234.000&end=5678.000&step=30s`
		URL := fmt.Sprintf("%s/%s", gw.Addr(), path)
		resp, err := makeTLSReq("https", URL)
		Expect(err).ToNot(HaveOccurred())
		Expect(resp.StatusCode).To(Equal(http.StatusOK))

		reqs := spyMetricStore.GetRangeQueryRequests()
		Expect(reqs).To(HaveLen(1))
		Expect(reqs[0].Query).To(Equal(`metric{source_id="some-id"}`))
		Expect(reqs[0].Start).To(Equal("1234.000"))
		Expect(reqs[0].End).To(Equal("5678.000"))
		Expect(reqs[0].Step).To(Equal("30s"))
	})

	It("upgrades HTTPS requests for series queries via PromQLAPI GETs into gRPC requests", func() {
		path := `api/v1/series?match[]=metric{source_id="some-id"}&match[]=metric_2&start=1234.000&end=5678.000`
		URL := fmt.Sprintf("%s/%s", gw.Addr(), path)
		resp, err := makeTLSReq("https", URL)
		Expect(err).ToNot(HaveOccurred())
		Expect(resp.StatusCode).To(Equal(http.StatusOK))

		reqs := spyMetricStore.GetSeriesQueryRequests()
		Expect(reqs).To(HaveLen(1))
		Expect(reqs[0].Match).To(ConsistOf(`metric{source_id="some-id"}`, `metric_2`))
		Expect(reqs[0].Start).To(Equal("1234.000"))
		Expect(reqs[0].End).To(Equal("5678.000"))
	})

	It("outputs json with zero-value points and correct Prometheus API fields", func() {
		path := `api/v1/query?query=metric{source_id="some-id"}&time=1234`
		URL := fmt.Sprintf("%s/%s", gw.Addr(), path)
		spyMetricStore.SetValue(0)
		resp, err := makeTLSReq("https", URL)

		Expect(err).ToNot(HaveOccurred())
		Expect(resp.StatusCode).To(Equal(http.StatusOK))

		body, _ := ioutil.ReadAll(resp.Body)
		Expect(body).To(MatchJSON(`{"status":"success","data":{"resultType":"scalar","result":[99,"0"]}}`))
	})

	It("does not accept unencrypted connections", func() {
		resp, err := makeTLSReq("http", fmt.Sprintf("%s/api/v1/query", gw.Addr()))
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))
	})

	Context("errors", func() {
		It("passes through content-type correctly on errors", func() {
			path := `api/v1/query?query=metric{source_id="some-id"}&time=1234`
			spyMetricStore.QueryError = errors.New("expected error")
			URL := fmt.Sprintf("%s/%s", gw.Addr(), path)
			resp, err := makeTLSReq("https", URL)

			Expect(err).ToNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusInternalServerError))
			Expect(resp.Header).To(HaveKeyWithValue("Content-Type", []string{"application/json"}))
		})

		DescribeTable("adds necessary fields to match Prometheus API", func(path string) {
			spyMetricStore.QueryError = errors.New("expected error")
			URL := fmt.Sprintf("%s/%s", gw.Addr(), path)
			resp, err := makeTLSReq("https", URL)

			Expect(err).ToNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusInternalServerError))

			body, _ := ioutil.ReadAll(resp.Body)
			Expect(body).To(MatchJSON(`{
				"status": "error",

				"errorType": "internal",
				"error": "expected error"
			}`))
		},
			Entry("query", `api/v1/query?query=metric{source_id="some-id"}&time=1234`),
			Entry("query_range", `api/v1/query_range?query=metric{source_id="some-id"}&start=1234&end=1235&step=1`),
			Entry("series", `api/v1/series?match[]=metric{source_id="some-id"}&match[]=metric_2{}`),
		)
	})
})

func makeTLSReq(scheme, addr string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s://%s", scheme, addr), nil)
	Expect(err).ToNot(HaveOccurred())

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}

	return client.Do(req)
}
