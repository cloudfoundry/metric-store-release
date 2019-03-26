package acceptance

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"

	ms "github.com/cloudfoundry/metric-store-release/src/pkg/client"
	"github.com/cloudfoundry/metric-store-release/src/pkg/rpc/metricstore_v1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc"
)

var _ = Describe("Metric Store on a CF", func() {
	var (
		client *ms.Client
		cfg    *TestConfig
	)
	Context("using gRPC client", func() {
		BeforeEach(func() {
			cfg = Config()
			client = ms.NewClient(
				cfg.MetricStoreAddr,
				ms.WithViaGRPC(
					grpc.WithTransportCredentials(
						cfg.TLS.Credentials("metric-store"),
					),
				),
			)
		})

		It("returns results for /api/v1/query", func() {
			// absolute_entitlement requires an app to be deployed on the CF
			// if it is failing, check if there are any apps deployed
			ctx := context.Background()
			result, err := client.PromQL(ctx, "absolute_entitlement")
			Expect(err).ToNot(HaveOccurred())

			samples := result.GetVector().GetSamples()
			Expect(len(samples)).ToNot(BeZero())
			Expect(samples[0].Metric["__name__"]).To(Equal("absolute_entitlement"))
			Expect(samples[0].Metric["unit"]).To(Equal("nanoseconds"))
			Expect(samples[0].Point).ToNot(BeNil())
		})

		XIt("returns results for /api/v1/label/job/values", func() {
			// when I query that endpoint
			// metric store should def be in this list
		})

		// I don't know that we need this endpoint but merh
		XIt("returns series results", func() {
			// when i query the series endpoint for metric-store job egress
			// metric over 30 seconds
			// there should be a metric
		})
	})

	Context("using HTTP client to traverse the auth proxy", func() {
		BeforeEach(func() {
			cfg = Config()
			oauthClient := newOauth2HTTPClient(cfg)
			client = ms.NewClient(
				cfg.MetricStoreCFAuthProxyURL,
				ms.WithHTTPClient(oauthClient),
			)
		})

		It("returns results for /api/v1/query", func() {
			// absolute_entitlement requires an app to be deployed on the CF
			// if it is failing, check if there are any apps deployed
			ctx := context.Background()
			result, err := client.PromQL(ctx, "absolute_entitlement")
			Expect(err).ToNot(HaveOccurred())

			samples := result.GetVector().GetSamples()
			Expect(len(samples)).ToNot(BeZero())
			Expect(samples[0].Metric["__name__"]).To(Equal("absolute_entitlement"))
			Expect(samples[0].Metric["unit"]).To(Equal("nanoseconds"))
			Expect(samples[0].Point).ToNot(BeNil())
		})

		XIt("returns results for /api/v1/label/job/values", func() {
			// absolute_entitlement requires an app to be deployed on the CF
			// if it is failing, check if there are any apps deployed
		})

		XIt("returns series results", func() {
			// when i query the series endpoint for metric-store job egress
			// metric over 30 seconds
			// there should be a metric
			// NOTE: https://metric-store.SYSTEM-DOMAIN/api/v1/series --data-urlencode 'match[]=egress{job="metric-store"}' --data-urlencode 'start=1552603300' --data-urlencode 'end=1552603397'
		})

		XIt("returns query results", func() {
			// when i query the series endpoint for metric-store job egress
			// metric over 30 seconds
			// there should be a metric
		})
	})
})

func flattenVector(v *metricstore_v1.PromQL_Vector) []string {
	var m []string
	for k, v := range v.GetSamples()[0].Metric {
		m = append(m, k)
		m = append(m, v)
	}

	return m
}

func newOauth2HTTPClient(cfg *TestConfig) *ms.Oauth2HTTPClient {
	oauth_client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: cfg.SkipCertVerify,
			},
		},
	}

	return ms.NewOauth2HTTPClient(
		cfg.UAAURL,
		cfg.ClientID,
		cfg.ClientSecret,
		ms.WithOauth2HTTPClient(oauth_client),
	)
}
