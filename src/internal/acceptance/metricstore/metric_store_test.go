package metricstore_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"

	"github.com/cloudfoundry/metric-store-release/src/pkg/metricstore"
	"github.com/cloudfoundry/metric-store-release/src/pkg/persistence/transform"
	rpc "github.com/cloudfoundry/metric-store-release/src/pkg/rpc/metricstore_v1"
	"github.com/prometheus/common/expfmt"

	. "github.com/cloudfoundry/metric-store-release/src/pkg/matchers"
	"github.com/cloudfoundry/metric-store-release/src/pkg/testing"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

// Sentinel to detect build failures early
var __ *metricstore.MetricStore

var storagePath = "/tmp/metric-store-node"

var _ = Describe("MetricStore", func() {
	type testContext struct {
		addr               string
		healthAddr         string
		gatewayAddr        string
		gatewayHealthAddr  string
		metricStoreProcess *gexec.Session
		gatewayProcess     *gexec.Session
	}

	var start = func(tc *testContext) {
		caCert := testing.Cert("metric-store-ca.crt")
		cert := testing.Cert("metric-store.crt")
		key := testing.Cert("metric-store.key")

		tc.metricStoreProcess = testing.StartGoProcess(
			"github.com/cloudfoundry/metric-store-release/src/cmd/metric-store",
			[]string{
				"ADDR=" + tc.addr,
				"HEALTH_ADDR=" + tc.healthAddr,
				"STORAGE_PATH=" + storagePath,
				"RETENTION_PERIOD_IN_DAYS=1",
				"CA_PATH=" + caCert,
				"CERT_PATH=" + cert,
				"KEY_PATH=" + key,
			},
		)

		tc.gatewayProcess = testing.StartGoProcess(
			"github.com/cloudfoundry/metric-store-release/src/cmd/gateway",
			[]string{
				"ADDR=" + tc.gatewayAddr,
				"HEALTH_ADDR=" + tc.gatewayHealthAddr,
				"METRIC_STORE_ADDR=" + tc.addr,
				"CA_PATH=" + caCert,
				"CERT_PATH=" + cert,
				"KEY_PATH=" + key,
			},
		)

		testing.WaitForHealthCheck(tc.healthAddr)
		testing.WaitForServer(tc.gatewayAddr)
	}

	var stop = func(tc *testContext) {
		tc.metricStoreProcess.Terminate()
		tc.gatewayProcess.Terminate()
		Eventually(tc.metricStoreProcess.Exited).Should(BeClosed())
		Eventually(tc.gatewayProcess.Exited).Should(BeClosed())
	}

	var perform = func(tc *testContext, operation func(*testContext)) {
		wg := &sync.WaitGroup{}

		wg.Add(1)
		go func() {
			defer GinkgoRecover()
			defer wg.Done()
			operation(tc)
		}()
		wg.Wait()
	}

	var setup = func() (*testContext, func()) {
		tc := &testContext{}

		tc.addr = fmt.Sprintf("localhost:%d", testing.GetFreePort())
		tc.healthAddr = fmt.Sprintf("localhost:%d", testing.GetFreePort())
		tc.gatewayAddr = fmt.Sprintf("localhost:%d", testing.GetFreePort())
		tc.gatewayHealthAddr = fmt.Sprintf("localhost:%d", testing.GetFreePort())

		perform(tc, start)

		return tc, func() {
			perform(tc, stop)
			os.RemoveAll(storagePath)
		}
	}

	type testInstantQuery struct {
		Query         string
		TimeInSeconds string
	}

	var makeInstantQuery = func(tc *testContext, query testInstantQuery) (*http.Response, error) {
		queryUrl, err := url.Parse(fmt.Sprintf("http://%s/api/v1/query", tc.gatewayAddr))
		Expect(err).ToNot(HaveOccurred())

		queryString := queryUrl.Query()
		queryString.Set("query", query.Query)
		queryString.Set("time", query.TimeInSeconds)
		queryUrl.RawQuery = queryString.Encode()

		return http.Get(queryUrl.String())
	}

	type testRangeQuery struct {
		Query          string
		StartInSeconds string
		EndInSeconds   string
		StepDuration   string
	}

	var makeRangeQuery = func(tc *testContext, query testRangeQuery) (*http.Response, error) {
		queryUrl, err := url.Parse(fmt.Sprintf("http://%s/api/v1/query_range", tc.gatewayAddr))
		Expect(err).ToNot(HaveOccurred())

		queryString := queryUrl.Query()
		queryString.Set("query", query.Query)
		queryString.Set("start", query.StartInSeconds)
		queryString.Set("end", query.EndInSeconds)
		queryString.Set("step", query.StepDuration)
		queryUrl.RawQuery = queryString.Encode()

		return http.Get(queryUrl.String())
	}

	type testSeriesQuery struct {
		Match          []string
		StartInSeconds string
		EndInSeconds   string
	}

	var makeSeriesQuery = func(tc *testContext, query testSeriesQuery) (*http.Response, error) {
		queryUrl, err := url.Parse(fmt.Sprintf("http://%s/api/v1/series", tc.gatewayAddr))
		Expect(err).ToNot(HaveOccurred())

		queryString := queryUrl.Query()
		for _, match := range query.Match {
			queryString.Add("match[]", match)
		}
		queryString.Set("start", query.StartInSeconds)
		queryString.Set("end", query.EndInSeconds)
		queryUrl.RawQuery = queryString.Encode()

		return http.Get(queryUrl.String())
	}

	type testPoint struct {
		Name               string
		TimeInMilliseconds int64
		Value              float64
		Labels             map[string]string
	}

	type testLabelValuesResult struct {
		Status string   `json:"status"`
		Data   []string `json:"data"`
	}

	var writePoints = func(tc *testContext, points []testPoint) {
		ingressClient, cleanup := testing.NewIngressClient(tc.addr)
		defer cleanup()

		var rpcPoints []*rpc.Point
		metricNameCounts := make(map[string]int)
		for _, point := range points {
			timestamp := transform.MillisecondsToNanoseconds(point.TimeInMilliseconds)

			rpcPoints = append(rpcPoints, &rpc.Point{
				Name:      point.Name,
				Value:     point.Value,
				Timestamp: timestamp,
				Labels:    point.Labels,
			})

			metricNameCounts[point.Name]++
		}

		_, err := ingressClient.Send(context.Background(), &rpc.SendRequest{
			Batch: &rpc.Points{
				Points: rpcPoints,
			},
		})
		Expect(err).ToNot(HaveOccurred())

		Eventually(func() bool {
			resp, _ := http.Get(fmt.Sprintf("http://%s/api/v1/label/__name__/values", tc.gatewayAddr))
			jsonBytes, _ := ioutil.ReadAll(resp.Body)

			var result testLabelValuesResult
			json.Unmarshal(jsonBytes, &result)

			return len(result.Data) == len(metricNameCounts)
		}, 3).Should(BeTrue())
	}

	It("deletes shards with old data when Metric Store starts", func() {
		tc, cleanup := setup()
		defer cleanup()

		now := time.Now()
		Eventually(func() []string {
			ic1, cleanup1 := testing.NewIngressClient(tc.addr)
			defer cleanup1()

			_, err := ic1.Send(context.Background(), &rpc.SendRequest{
				Batch: &rpc.Points{
					Points: []*rpc.Point{
						{Name: "metric_name_old", Timestamp: 1},
						{Name: "metric_name_new", Timestamp: now.UnixNano()},
					},
				},
			})

			Expect(err).ToNot(HaveOccurred())

			resp, err := http.Get(fmt.Sprintf("http://%s/api/v1/label/__name__/values", tc.gatewayAddr))
			if err != nil {
				return nil
			}
			jsonBytes, _ := ioutil.ReadAll(resp.Body)

			var result testLabelValuesResult
			json.Unmarshal(jsonBytes, &result)

			return result.Data
		}, 5).Should(ConsistOf([]string{
			"metric_name_old",
			"metric_name_new",
		}))

		stop(tc)

		start(tc)

		Eventually(func() error {
			_, err := http.Get(fmt.Sprintf("http://%s/api/v1/label/__name__/values", tc.gatewayAddr))

			return err
		}, 5).Should(Succeed())

		Eventually(func() []string {
			resp, err := http.Get(fmt.Sprintf("http://%s/api/v1/label/__name__/values", tc.gatewayAddr))
			Expect(err).ToNot(HaveOccurred())

			jsonBytes, _ := ioutil.ReadAll(resp.Body)

			var result testLabelValuesResult
			json.Unmarshal(jsonBytes, &result)

			return result.Data
		}, 1).Should(ConsistOf([]string{
			"metric_name_new",
		}))
	})

	Context("when using HTTP", func() {
		Context("when a instant query is made", func() {
			It("returns metrics from a simple query", func() {
				tc, cleanup := setup()
				defer cleanup()

				writePoints(
					tc,
					[]testPoint{
						{
							Name:               "metric_name",
							Value:              99,
							TimeInMilliseconds: 1500,
							Labels: map[string]string{
								"source_id": "1",
							},
						},
					},
				)

				resp, err := makeInstantQuery(tc, testInstantQuery{
					Query:         "metric_name",
					TimeInSeconds: "2",
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(http.StatusOK))
				body, err := ioutil.ReadAll(resp.Body)
				Expect(err).ToNot(HaveOccurred())

				Expect(body).To(MatchJSON(`{
					"status":"success",
					"data": {
					  "resultType":"vector",
					  "result": [
						{
						  "metric": {
							"__name__": "metric_name",
							"source_id": "1"
						  },
						  "value": [ 2.000, "99" ]
						}
					  ]
					}
				  }`))
			})
		})

		Context("when a range query is made", func() {
			It("returns metrics from a simple query", func() {
				tc, cleanup := setup()
				defer cleanup()

				writePoints(
					tc,
					[]testPoint{
						{
							Name:               "metric_name",
							Value:              99,
							TimeInMilliseconds: 1500,
							Labels: map[string]string{
								"source_id": "1",
							},
						},
						{
							Name:               "metric_name",
							Value:              93,
							TimeInMilliseconds: 1700,
							Labels: map[string]string{
								"source_id": "1",
							},
						},
						{
							Name:               "metric_name",
							Value:              88,
							TimeInMilliseconds: 3800,
							Labels: map[string]string{
								"source_id": "1",
							},
						},
						{
							Name:               "metric_name",
							Value:              99,
							TimeInMilliseconds: 3500,
							Labels: map[string]string{
								"source_id": "2",
							},
						},
					},
				)

				resp, err := makeRangeQuery(tc, testRangeQuery{
					Query:          "metric_name",
					StartInSeconds: "1",
					EndInSeconds:   "5",
					StepDuration:   "2",
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(http.StatusOK))
				body, err := ioutil.ReadAll(resp.Body)
				Expect(err).ToNot(HaveOccurred())

				Expect(body).To(MatchJSON(`{
					"status":"success",
					"data": {
					  "resultType":"matrix",
					  "result": [
						{
						  "metric": {
							"__name__": "metric_name",
							"source_id": "1"
						  },
						  "values": [[3,"93"], [5,"88"]]
						},
						{
						  "metric": {
							"__name__": "metric_name",
							"source_id": "2"
						  },
						  "values": [[5,"99"]]
						}
					  ]
					}
				  }`))
			})
		})
	})

	Context("when a labels query is made", func() {
		It("returns labels from Metric Store", func() {
			tc, cleanup := setup()
			defer cleanup()

			writePoints(
				tc,
				[]testPoint{
					{
						Name:               "metric_name_0",
						TimeInMilliseconds: 1,
						Labels: map[string]string{
							"source_id":  "1",
							"user_agent": "phil",
						},
					},
					{
						Name:               "metric_name_1",
						TimeInMilliseconds: 2,
						Labels: map[string]string{
							"source_id":      "2",
							"content_length": "42",
						},
					},
				},
			)

			resp, err := http.Get(fmt.Sprintf("http://%s/api/v1/labels", tc.gatewayAddr))
			Expect(err).ToNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			body, err := ioutil.ReadAll(resp.Body)
			Expect(err).ToNot(HaveOccurred())

			Expect(body).To(MatchJSON(`{
				"status":"success",
				"data":["__name__", "source_id"]
			}`))
		})
	})

	Context("when a label values query is made", func() {
		It("returns values for a label name", func() {
			tc, cleanup := setup()
			defer cleanup()

			writePoints(
				tc,
				[]testPoint{
					{
						Name:               "metric_name_0",
						TimeInMilliseconds: 1,
						Labels: map[string]string{
							"source_id":  "1",
							"user_agent": "100",
						},
					},
					{
						Name:               "metric_name_1",
						TimeInMilliseconds: 2,
						Labels: map[string]string{
							"source_id":  "10",
							"user_agent": "200",
						},
					},
					{
						Name:               "metric_name_2",
						TimeInMilliseconds: 3,
						Labels: map[string]string{
							"source_id":  "10",
							"user_agent": "100",
						},
					},
				},
			)

			resp, err := http.Get(fmt.Sprintf("http://%s/api/v1/label/source_id/values", tc.gatewayAddr))
			Expect(err).ToNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			body, err := ioutil.ReadAll(resp.Body)
			Expect(err).ToNot(HaveOccurred())

			Expect(body).To(Or(
				MatchJSON(`{
					"status":"success",
					"data":["1", "10"]
				}`),
				MatchJSON(`{
					"status":"success",
					"data":["10", "1"]
				}`),
			))

			resp, err = http.Get(fmt.Sprintf("http://%s/api/v1/label/user_agent/values", tc.gatewayAddr))
			Expect(err).ToNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			body, err = ioutil.ReadAll(resp.Body)
			Expect(err).ToNot(HaveOccurred())

			Expect(body).To(MatchJSON(`{
				"status":"success",
				"data":[]
			}`))

			resp, err = http.Get(fmt.Sprintf("http://%s/api/v1/label/__name__/values", tc.gatewayAddr))
			Expect(err).ToNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			body, err = ioutil.ReadAll(resp.Body)
			Expect(err).ToNot(HaveOccurred())

			Expect(body).To(MatchJSON(`{
					"status":"success",
					"data":["metric_name_0", "metric_name_1", "metric_name_2"]
				}`))
		})
	})

	Context("when a series query is made", func() {
		It("returns metrics from a simple query", func() {
			tc, cleanup := setup()
			defer cleanup()

			writePoints(
				tc,
				[]testPoint{
					{
						Name:               "metric_name",
						Value:              99,
						TimeInMilliseconds: 1500,
						Labels: map[string]string{
							"source_id": "1",
						},
					},
				},
			)

			resp, err := makeSeriesQuery(tc, testSeriesQuery{
				Match:          []string{"metric_name"},
				StartInSeconds: "1",
				EndInSeconds:   "2",
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			body, err := ioutil.ReadAll(resp.Body)
			Expect(err).ToNot(HaveOccurred())

			Expect(body).To(MatchJSON(`{
		 		"status": "success",
		 		"data": [
		 		  {
		 		    "__name__": "metric_name",
		 		    "source_id": "1"
		 		  }
		 		]
		    }`))
		})
	})

	It("exposes metrics in prometheus format", func() {
		tc, cleanup := setup()
		defer cleanup()

		ic1, cleanup1 := testing.NewIngressClient(tc.addr)
		defer cleanup1()

		_, err := ic1.Send(context.Background(), &rpc.SendRequest{
			Batch: &rpc.Points{
				Points: []*rpc.Point{
					{Name: "metric_name", Timestamp: 1},
				},
			},
		})

		resp, err := http.Get(fmt.Sprintf("http://%s/metrics", tc.healthAddr))
		Expect(err).ToNot(HaveOccurred())
		Expect(resp.StatusCode).To(Equal(http.StatusOK))

		var parser expfmt.TextParser
		parsed, err := parser.TextToMetricFamilies(resp.Body)
		Expect(err).ToNot(HaveOccurred())

		Expect(parsed).To(ContainCounterMetric("metric_store_ingress", float64(1)))
	})
})
