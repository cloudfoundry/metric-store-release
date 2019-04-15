package acceptance

import (
	"context"
	"crypto/tls"
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
			ctx := context.Background()
			result, err := client.PromQL(ctx, "doppler_egress")
			Expect(err).ToNot(HaveOccurred())

			samples := result.GetVector().GetSamples()
			Expect(len(samples)).ToNot(BeZero())
			Expect(samples[0].Metric["__name__"]).To(Equal("doppler_egress"))
			Expect(samples[0].Metric["unit"]).To(Equal("nanoseconds"))
			Expect(samples[0].Point).ToNot(BeNil())
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
			ctx := context.Background()
			result, err := client.PromQL(ctx, "doppler_egress")
			Expect(err).ToNot(HaveOccurred())

			samples := result.GetVector().GetSamples()
			Expect(len(samples)).ToNot(BeZero())
			Expect(samples[0].Metric["__name__"]).To(Equal("doppler_egress"))
			Expect(samples[0].Metric["unit"]).To(Equal("nanoseconds"))
			Expect(samples[0].Point).ToNot(BeNil())
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
