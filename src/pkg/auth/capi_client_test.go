package auth_test

import (
	"fmt"
	"io/ioutil"
	"log"
	"sync"
	"time"

	"github.com/cloudfoundry/metric-store-release/src/pkg/auth"
	"github.com/cloudfoundry/metric-store-release/src/pkg/testing"

	"errors"
	"net/http"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("CAPIClient", func() {
	type testContext struct {
		capiClient *spyHTTPClient
		client     *auth.CAPIClient
		metrics    *testing.SpyMetrics
	}

	var setup = func(capiOpts ...auth.CAPIOption) *testContext {
		capiClient := newSpyHTTPClient()
		metrics := testing.NewSpyMetrics()
		client := auth.NewCAPIClient(
			"http://external.capi.com",
			capiClient,
			metrics,
			log.New(ioutil.Discard, "", 0),
			capiOpts...,
		)

		return &testContext{
			capiClient: capiClient,
			metrics:    metrics,
			client:     client,
		}
	}

	Describe("IsAuthorized", func() {
		It("caches CAPI response for /v3/apps and /v3/service_instances request", func() {
			tc := setup(
				auth.WithCacheExpirationInterval(250 * time.Millisecond),
			)

			tc.capiClient.resps = []response{
				newCapiResp("37cbff06-79ef-4146-a7b0-01838940f185", http.StatusOK),
				newCapiResp("afbdcab7-6fd1-418d-bfd0-95c60276507b", http.StatusOK),
			}

			By("calling isAuthorized the first time and caching response")
			authorized := tc.client.IsAuthorized("37cbff06-79ef-4146-a7b0-01838940f185", "some-token")

			Expect(len(tc.capiClient.requests)).To(Equal(2))
			Expect(authorized).To(BeTrue())

			By("reusing the previously cached response")
			authorized = tc.client.IsAuthorized("afbdcab7-6fd1-418d-bfd0-95c60276507b", "some-token")

			Expect(len(tc.capiClient.requests)).To(Equal(2))
			Expect(authorized).To(BeTrue())

			tc.capiClient.resps = []response{
				newCapiResp("37cbff06-79ef-4146-a7b0-01838940f185", http.StatusOK),
				newCapiResp("afbdcab7-6fd1-418d-bfd0-95c60276507b", http.StatusOK),
			}

			By("continuing to call until the previously cached response expires")
			Eventually(func() int {
				authorized = tc.client.IsAuthorized("37cbff06-79ef-4146-a7b0-01838940f185", "some-token")
				Expect(authorized).To(BeTrue())

				return len(tc.capiClient.requests)
			}).Should(Equal(4))
		})

		It("retries CAPI requests up to 3 times when a cache miss occurs", func() {
			tc := setup(
				auth.WithCacheExpirationInterval(250 * time.Millisecond),
			)

			tc.capiClient.resps = []response{
				newCapiResp("37cbff06-79ef-4146-a7b0-01838940f185", http.StatusOK),
				newCapiResp("afbdcab7-6fd1-418d-bfd0-95c60276507b", http.StatusOK),

				newCapiResp("37cbff06-79ef-4146-a7b0-01838940f185", http.StatusOK),
				newCapiResp("afbdcab7-6fd1-418d-bfd0-95c60276507b", http.StatusOK),

				newCapiResp("37cbff06-79ef-4146-a7b0-01838940f185", http.StatusOK),
				newCapiResp("afbdcab7-6fd1-418d-bfd0-95c60276507b", http.StatusOK),

				newCapiResp("37cbff06-79ef-4146-a7b0-01838940f185", http.StatusOK),
				newCapiResp("afbdcab7-6fd1-418d-bfd0-95c60276507b", http.StatusOK),

				newCapiResp("37cbff06-79ef-4146-a7b0-01838940f185", http.StatusOK),
				newCapiResp("afbdcab7-6fd1-418d-bfd0-95c60276507b", http.StatusOK),
			}

			// the first time we call IsAuthorized, we'll cache the response
			authorized := tc.client.IsAuthorized("37cbff06-79ef-4146-a7b0-01838940f185", "some-token")

			Expect(len(tc.capiClient.requests)).To(Equal(2))
			Expect(authorized).To(BeTrue())

			// when we use the same token to look for an unknown sourceId, the
			// request should fail to authorize, but we've still made 2 new
			// calls out to CAPI - this is retry #1
			authorized = tc.client.IsAuthorized("abcd1234", "some-token")
			Expect(len(tc.capiClient.requests)).To(Equal(4))
			Expect(authorized).To(BeFalse())

			// this is retry #2
			authorized = tc.client.IsAuthorized("abcd1234", "some-token")
			Expect(len(tc.capiClient.requests)).To(Equal(6))
			Expect(authorized).To(BeFalse())

			// this is retry #3
			authorized = tc.client.IsAuthorized("abcd1234", "some-token")
			Expect(len(tc.capiClient.requests)).To(Equal(8))
			Expect(authorized).To(BeFalse())

			// this would be retry #4, but exceeds our max of 3, thus no new
			// requests are made out to CAPI
			authorized = tc.client.IsAuthorized("abcd1234", "some-token")
			Expect(len(tc.capiClient.requests)).To(Equal(8))
			Expect(authorized).To(BeFalse())
		})

		It("succeeds when a CAPI retry returns a valid sourceId", func() {
			tc := setup(
				auth.WithCacheExpirationInterval(250 * time.Millisecond),
			)

			tc.capiClient.resps = []response{
				newCapiResp("37cbff06-79ef-4146-a7b0-01838940f185", http.StatusOK),
				newCapiResp("afbdcab7-6fd1-418d-bfd0-95c60276507b", http.StatusOK),

				newCapiResp("37cbff06-79ef-4146-a7b0-01838940f185", http.StatusOK),
				newCapiResp("afbdcab7-6fd1-418d-bfd0-95c60276507b", http.StatusOK),

				newCapiResp("abcd1234", http.StatusOK),
				newCapiResp("afbdcab7-6fd1-418d-bfd0-95c60276507b", http.StatusOK),
			}

			// the first time we call IsAuthorized, we'll cache the response
			authorized := tc.client.IsAuthorized("37cbff06-79ef-4146-a7b0-01838940f185", "some-token")

			Expect(len(tc.capiClient.requests)).To(Equal(2))
			Expect(authorized).To(BeTrue())

			// when we use the same token to look for an unknown sourceId, the
			// request should fail to authorize, but we've still made 2 new
			// calls out to CAPI - this is retry #1
			authorized = tc.client.IsAuthorized("abcd1234", "some-token")
			Expect(len(tc.capiClient.requests)).To(Equal(4))
			Expect(authorized).To(BeFalse())

			// this is retry #2, which now has the correct sourceId and
			// should authorize correctly
			authorized = tc.client.IsAuthorized("abcd1234", "some-token")
			Expect(len(tc.capiClient.requests)).To(Equal(6))
			Expect(authorized).To(BeTrue())
		})

		It("sourceIDs from expired cached tokens are not authorized", func() {
			tc := setup(
				auth.WithCacheExpirationInterval(250 * time.Millisecond),
			)

			tc.capiClient.resps = []response{
				newCapiResp("8208c86c-7afe-45f8-8999-4883d5868cf2", http.StatusOK),
				newCapiResp("dc94ebb2-5038-4645-afbf-1093bbd58e94", http.StatusOK),
			}

			authorized := tc.client.IsAuthorized(
				"8208c86c-7afe-45f8-8999-4883d5868cf2",
				"token-0",
			)

			Expect(authorized).To(BeTrue())
			time.Sleep(250 * time.Millisecond)

			tc.capiClient.resps = []response{
				newCapiResp("37cbff06-79ef-4146-a7b0-01838940f185", http.StatusOK),
				newCapiResp("afbdcab7-6fd1-418d-bfd0-95c60276507b", http.StatusOK),
			}

			authorized = tc.client.IsAuthorized(
				"8208c86c-7afe-45f8-8999-4883d5868cf2",
				"token-1",
			)

			Expect(authorized).To(BeFalse())
		})

		It("regularly removes tokens from cache", func() {
			tc := setup(
				auth.WithTokenPruningInterval(250*time.Millisecond),
				auth.WithCacheExpirationInterval(250*time.Millisecond),
			)

			tc.client.IsAuthorized("8208c86c-7afe-45f8-8999-4883d5868cf2", "token-1")
			tc.client.IsAuthorized("8208c86c-7afe-45f8-8999-4883d5868cf2", "token-2")

			Eventually(tc.client.TokenCacheSize).Should(BeZero())
		})
	})

	Describe("AvailableSourceIDs", func() {
		It("returns the available app and service instance IDs", func() {
			tc := setup()

			tc.capiClient.resps = []response{
				{status: http.StatusOK, body: []byte(`{"resources": [{"guid": "app-0"}, {"guid": "app-1"}]}`)},
				{status: http.StatusOK, body: []byte(`{"resources": [{"guid": "service-2"}, {"guid": "service-3"}]}`)},
			}
			sourceIds := tc.client.AvailableSourceIDs("some-token")
			Expect(sourceIds).To(ConsistOf("app-0", "app-1", "service-2", "service-3"))

			Expect(tc.capiClient.requests).To(HaveLen(2))

			appsReq := tc.capiClient.requests[0]
			Expect(appsReq.Method).To(Equal(http.MethodGet))
			Expect(appsReq.URL.String()).To(Equal("http://external.capi.com/v3/apps"))
			Expect(appsReq.Header.Get("Authorization")).To(Equal("some-token"))

			servicesReq := tc.capiClient.requests[1]
			Expect(servicesReq.Method).To(Equal(http.MethodGet))
			Expect(servicesReq.URL.String()).To(Equal("http://external.capi.com/v3/service_instances"))
			Expect(servicesReq.Header.Get("Authorization")).To(Equal("some-token"))
		})

		It("iterates through all pages returned by /v3/apps", func() {
			tc := setup()

			By("replacing the scheme and host to match original requests' capi addr")
			tc.capiClient.resps = []response{
				{status: http.StatusOK, body: []byte(`{
                  "pagination": {
                    "next": {
                      "href": "https://thisismycapi.com/v3/apps?page=2&per_page=1"
                    }
                  },
                  "resources": [
                      {"guid": "app-1", "name": "app-name"}
                  ]
                }`)},
				{status: http.StatusOK, body: []byte(`{
                  "resources": [
                      {"guid": "app-2", "name": "app-name"}
                  ]
                }`)},
				emptyCapiResp,
			}

			sourceIds := tc.client.AvailableSourceIDs("some-token")
			Expect(tc.capiClient.requests).To(HaveLen(3))
			secondPageReq := tc.capiClient.requests[1]

			Expect(secondPageReq.URL.String()).To(Equal("http://external.capi.com/v3/apps?page=2&per_page=1"))
			Expect(sourceIds).To(ConsistOf("app-1", "app-2"))
		})

		It("iterates through all pages returned by /v3/service_instances", func() {
			tc := setup()

			tc.capiClient.resps = []response{
				emptyCapiResp,
				{status: http.StatusOK, body: []byte(`{
                  "pagination": {
                    "next": {
                      "href": "https://external.capi.com/v3/service_instances?page=2&per_page=2"
                    }
                  },
                  "resources": [
                    {"guid": "service-1"},
                    {"guid": "service-2"}
                  ]
                }`)},
				{status: http.StatusOK, body: []byte(`{
                  "resources": [
                    {"guid": "service-3"}
                  ]
                }`)},
			}
			sourceIds := tc.client.AvailableSourceIDs("some-token")
			Expect(tc.capiClient.requests).To(HaveLen(3))
			secondPageReq := tc.capiClient.requests[2]
			Expect(secondPageReq.URL.Path).To(Equal("/v3/service_instances"))
			Expect(secondPageReq.URL.Query().Get("page")).To(Equal("2"))
			Expect(secondPageReq.URL.Query().Get("per_page")).To(Equal("2"))

			Expect(sourceIds).To(ConsistOf("service-1", "service-2", "service-3"))
		})

		It("returns empty slice when CAPI apps request returns non 200", func() {
			tc := setup()

			tc.capiClient.resps = []response{
				{status: http.StatusNotFound},
				{status: http.StatusOK, body: []byte(`{"resources": [{"metadata":{"guid": "service-2"}}, {"metadata":{"guid": "service-3"}}]}`)},
			}
			sourceIds := tc.client.AvailableSourceIDs("some-token")
			Expect(sourceIds).To(BeEmpty())
		})

		It("returns empty slice when CAPI apps request fails", func() {
			tc := setup()

			tc.capiClient.resps = []response{
				{err: errors.New("intentional error")},
				{status: http.StatusOK, body: []byte(`{"resources": [{"metadata":{"guid": "service-2"}}, {"metadata":{"guid": "service-3"}}]}`)},
			}
			sourceIds := tc.client.AvailableSourceIDs("some-token")
			Expect(sourceIds).To(BeEmpty())
		})

		It("returns empty slice when CAPI service_instances request returns non 200", func() {
			tc := setup()

			tc.capiClient.resps = []response{
				{status: http.StatusOK, body: []byte(`{"resources": [{"guid": "app-0"}, {"guid": "app-1"}]}`)},
				{status: http.StatusNotFound},
			}
			sourceIds := tc.client.AvailableSourceIDs("some-token")
			Expect(sourceIds).To(BeEmpty())
		})

		It("returns empty slice when CAPI service_instances request fails", func() {
			tc := setup()

			tc.capiClient.resps = []response{
				{status: http.StatusOK, body: []byte(`{"resources": [{"guid": "app-0"}, {"guid": "app-1"}]}`)},
				{err: errors.New("intentional error")},
			}
			sourceIds := tc.client.AvailableSourceIDs("some-token")
			Expect(sourceIds).To(BeEmpty())
		})

		It("stores the latency", func() {
			tc := setup()

			tc.capiClient.resps = []response{
				emptyCapiResp,
				emptyCapiResp,
			}
			tc.client.AvailableSourceIDs("my-token")

			Expect(tc.metrics.Get("cf_auth_proxy_last_capiv3_apps_latency")).ToNot(BeZero())
			Expect(tc.metrics.Get("cf_auth_proxy_last_capiv3_list_service_instances_latency")).ToNot(BeZero())
			Expect(tc.metrics.GetUnit("cf_auth_proxy_last_capiv3_list_service_instances_latency")).To(Equal("nanoseconds"))
		})

		It("is goroutine safe", func() {
			tc := setup()

			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				for i := 0; i < 1000; i++ {
					tc.client.AvailableSourceIDs("some-token")
				}

				wg.Done()
			}()

			for i := 0; i < 1000; i++ {
				tc.client.AvailableSourceIDs("some-token")
			}
			wg.Wait()
		})
	})

	Describe("GetRelatedSourceIds", func() {
		It("hits CAPI correctly", func() {
			tc := setup()

			tc.client.GetRelatedSourceIds([]string{"app-name-1", "app-name-2"}, "some-token")
			Expect(tc.capiClient.requests).To(HaveLen(1))

			appsReq := tc.capiClient.requests[0]
			Expect(appsReq.Method).To(Equal(http.MethodGet))
			Expect(appsReq.URL.Host).To(Equal("external.capi.com"))
			Expect(appsReq.URL.Path).To(Equal("/v3/apps"))
			Expect(appsReq.URL.Query().Get("names")).To(Equal("app-name-1,app-name-2"))
			// Expect(appsReq.URL.Query().Get("per_page")).To(Equal("5000"))
			Expect(appsReq.Header.Get("Authorization")).To(Equal("some-token"))
		})

		It("gets related source IDs for a single app", func() {
			tc := setup()

			tc.capiClient.resps = []response{
				{status: http.StatusOK, body: []byte(
					`{
                  "resources": [
                      {"guid": "app-0", "name": "app-name"},
                      {"guid": "app-1", "name": "app-name"}
                  ]
                }`)},
			}

			sourceIds := tc.client.GetRelatedSourceIds([]string{"app-name"}, "some-token")
			Expect(sourceIds).To(HaveKeyWithValue("app-name", ConsistOf("app-0", "app-1")))
		})

		It("iterates through all pages returned by /v3/apps", func() {
			tc := setup()

			tc.capiClient.resps = []response{
				{status: http.StatusOK, body: []byte(`{
                  "pagination": {
                    "next": {
                      "href": "https://external.capi.com/v3/apps?page=2&per_page=2"
                    }
                  },
                  "resources": [
                      {"guid": "app-0", "name": "app-name"},
                      {"guid": "app-1", "name": "app-name"}
                  ]
                }`)},
				{status: http.StatusOK, body: []byte(`{
                  "resources": [
                      {"guid": "app-2", "name": "app-name"}
                  ]
                }`)},
			}

			sourceIds := tc.client.GetRelatedSourceIds([]string{"app-name"}, "some-token")

			Expect(tc.capiClient.requests).To(HaveLen(2))
			secondPageReq := tc.capiClient.requests[1]
			Expect(secondPageReq.URL.Path).To(Equal("/v3/apps"))
			Expect(secondPageReq.URL.Query().Get("page")).To(Equal("2"))
			Expect(secondPageReq.URL.Query().Get("per_page")).To(Equal("2"))
			Expect(sourceIds).To(HaveKeyWithValue("app-name", ConsistOf("app-0", "app-1", "app-2")))
		})

		It("gets related source IDs for multiple apps", func() {
			tc := setup()

			tc.capiClient.resps = []response{
				{status: http.StatusOK, body: []byte(`{
                "pagination": {
                  "next": {
                    "href": "https://api.example.org/v3/apps?page=2&per_page=2"
                  }
                },
                "resources": [
                  {"guid": "app-a-0", "name": "app-a"},
                  {"guid": "app-a-1", "name": "app-a"}
                ]
              }`)},
				{status: http.StatusOK, body: []byte(`{
                "resources": [
                  {"guid": "app-b-0", "name": "app-b"}
                ]
              }`)},
			}

			sourceIds := tc.client.GetRelatedSourceIds([]string{"app-a", "app-b"}, "some-token")
			Expect(sourceIds).To(HaveKeyWithValue("app-a", ConsistOf("app-a-0", "app-a-1")))
			Expect(sourceIds).To(HaveKeyWithValue("app-b", ConsistOf("app-b-0")))
		})

		It("doesn't issue a request when given no app names", func() {
			tc := setup()

			tc.client.GetRelatedSourceIds([]string{}, "some-token")
			Expect(tc.capiClient.requests).To(HaveLen(0))
		})

		It("stores the latency", func() {
			tc := setup()

			tc.capiClient.resps = []response{
				{status: http.StatusNotFound},
			}
			tc.client.GetRelatedSourceIds([]string{"app-name"}, "some-token")

			Expect(tc.metrics.Get("cf_auth_proxy_last_capiv3_apps_by_name_latency")).ToNot(BeZero())
			Expect(tc.metrics.GetUnit("cf_auth_proxy_last_capiv3_apps_by_name_latency")).To(Equal("nanoseconds"))
		})

		It("returns no source IDs when the request fails", func() {
			tc := setup()

			tc.capiClient.resps = []response{
				{status: http.StatusNotFound},
			}
			sourceIds := tc.client.GetRelatedSourceIds([]string{"app-name"}, "some-token")
			Expect(sourceIds).To(HaveLen(0))
		})

		It("returns no source IDs when the request returns a non-200 status code", func() {
			tc := setup()

			tc.capiClient.resps = []response{
				{err: errors.New("intentional error")},
			}
			sourceIds := tc.client.GetRelatedSourceIds([]string{"app-name"}, "some-token")
			Expect(sourceIds).To(HaveLen(0))
		})

		It("returns no source IDs when JSON decoding fails", func() {
			tc := setup()

			tc.capiClient.resps = []response{
				{status: http.StatusOK, body: []byte(`{`)},
			}
			sourceIds := tc.client.GetRelatedSourceIds([]string{"app-name"}, "some-token")
			Expect(sourceIds).To(HaveLen(0))
		})
	})
})

func newCapiResp(guid string, status int) response {
	return response{
		status: status,
		body: []byte(fmt.Sprintf(
			`{
               "resources": [
                 {
                   "guid": "%s"
                 }
               ]
             }`, guid)),
	}
}

var emptyCapiResp = response{
	status: http.StatusOK,
	body:   []byte(`{"resources": []}`),
}
