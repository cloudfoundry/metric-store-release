package metricstore_test

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	shared_api "github.com/cloudfoundry/metric-store-release/src/internal/api"
	"github.com/cloudfoundry/metric-store-release/src/internal/metrics"
	"github.com/cloudfoundry/metric-store-release/src/internal/metricstore"
	"github.com/cloudfoundry/metric-store-release/src/internal/version"
	"github.com/cloudfoundry/metric-store-release/src/pkg/ingressclient"
	"github.com/cloudfoundry/metric-store-release/src/pkg/persistence/transform"
	"github.com/cloudfoundry/metric-store-release/src/pkg/rpc"
	sharedtls "github.com/cloudfoundry/metric-store-release/src/pkg/tls"
	prom_api_client "github.com/prometheus/client_golang/api"
	prom_versioned_api_client "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/notifier"

	shared "github.com/cloudfoundry/metric-store-release/src/internal/testing"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

// Sentinel to detect build failures early
var __ *metricstore.MetricStore

var storagePaths = []string{"/tmp/metric-store-node1", "/tmp/metric-store-node2"}

type testInstantQuery struct {
	Query         string
	TimeInSeconds string
}

func (q *testInstantQuery) Timestamp() time.Time {
	timeInSeconds, _ := strconv.Atoi(q.TimeInSeconds)
	return time.Unix(int64(timeInSeconds), 0)
}

type testRangeQuery struct {
	Query          string
	StartInSeconds string
	EndInSeconds   string
	StepDuration   string
}

func (q *testRangeQuery) Range() prom_versioned_api_client.Range {
	startTimeInSeconds, _ := strconv.Atoi(q.StartInSeconds)
	endTimeInSeconds, _ := strconv.Atoi(q.EndInSeconds)
	stepDuration, _ := time.ParseDuration(q.StepDuration)

	return prom_versioned_api_client.Range{
		Start: time.Unix(int64(startTimeInSeconds), 0),
		End:   time.Unix(int64(endTimeInSeconds), 0),
		Step:  stepDuration,
	}
}

type testSeriesQuery struct {
	Match          []string
	StartInSeconds string
	EndInSeconds   string
}

func (q *testSeriesQuery) StartTimestamp() time.Time {
	timeInSeconds, _ := strconv.Atoi(q.StartInSeconds)
	return time.Unix(int64(timeInSeconds), 0)
}

func (q *testSeriesQuery) EndTimestamp() time.Time {
	timeInSeconds, _ := strconv.Atoi(q.EndInSeconds)
	return time.Unix(int64(timeInSeconds), 0)
}

const (
	MAGIC_MEASUREMENT_NAME      = "cpu"
	MAGIC_MEASUREMENT_PEER_NAME = "memory"
)

var _ = Describe("MetricStore", func() {
	type testContext struct {
		numNodes             int
		addrs                []string
		internodeAddrs       []string
		ingressAddrs         []string
		healthPorts          []string
		metricStoreProcesses []*gexec.Session
		tlsConfig            *tls.Config
		caCert               string
		cert                 string
		key                  string

		rulesPath         string
		alertmanagerAddr  string
		localEgressClient prom_versioned_api_client.API
	}

	var startNode = func(tc *testContext, index int) {
		metricStoreProcess := shared.StartGoProcess(
			"github.com/cloudfoundry/metric-store-release/src/cmd/metric-store",
			[]string{
				"ADDR=" + tc.addrs[index],
				"INGRESS_ADDR=" + tc.ingressAddrs[index],
				"INTERNODE_ADDR=" + tc.internodeAddrs[index],
				"HEALTH_PORT=" + tc.healthPorts[index],
				"STORAGE_PATH=" + storagePaths[index],
				"RETENTION_PERIOD_IN_DAYS=1",
				fmt.Sprintf("NODE_INDEX=%d", index),
				"NODE_ADDRS=" + strings.Join(tc.addrs, ","),
				"INTERNODE_ADDRS=" + strings.Join(tc.internodeAddrs, ","),
				"CA_PATH=" + tc.caCert,
				"CERT_PATH=" + tc.cert,
				"KEY_PATH=" + tc.key,
				"METRIC_STORE_SERVER_CA_PATH=" + tc.caCert,
				"METRIC_STORE_SERVER_CERT_PATH=" + tc.cert,
				"METRIC_STORE_SERVER_KEY_PATH=" + tc.key,
				"METRIC_STORE_INTERNODE_CA_PATH=" + tc.caCert,
				"METRIC_STORE_INTERNODE_CERT_PATH=" + tc.cert,
				"METRIC_STORE_INTERNODE_KEY_PATH=" + tc.key,
				"RULES_PATH=" + tc.rulesPath,
				"ALERTMANAGER_ADDR=" + tc.alertmanagerAddr,
			},
		)

		shared.WaitForHealthCheck(tc.healthPorts[index])
		tc.metricStoreProcesses[index] = metricStoreProcess
	}

	var stopNode = func(tc *testContext, index int) {
		tc.metricStoreProcesses[index].Terminate()
		Eventually(tc.metricStoreProcesses[index].Exited, 2*time.Second).Should(BeClosed())
	}

	var performOnAllNodes = func(tc *testContext, operation func(*testContext, int)) {
		wg := &sync.WaitGroup{}
		for index := 0; index < tc.numNodes; index++ {
			wg.Add(1)
			go func(index int) {
				defer GinkgoRecover()
				defer wg.Done()
				operation(tc, index)
			}(index)
		}
		wg.Wait()
	}

	type WithTestContextOption func(*testContext)

	var setup = func(numNodes int, opts ...WithTestContextOption) (*testContext, func()) {
		tc := &testContext{
			numNodes:             numNodes,
			metricStoreProcesses: make([]*gexec.Session, numNodes),
			caCert:               shared.Cert("metric-store-ca.crt"),
			cert:                 shared.Cert("metric-store.crt"),
			key:                  shared.Cert("metric-store.key"),
		}

		for _, opt := range opts {
			opt(tc)
		}

		var err error
		tc.tlsConfig, err = sharedtls.NewMutualTLSConfig(tc.caCert, tc.cert, tc.key, "metric-store")
		if err != nil {
			fmt.Printf("ERROR: invalid mutal TLS config: %s\n", err)
		}

		for i := 0; i < numNodes; i++ {
			tc.addrs = append(tc.addrs, fmt.Sprintf("localhost:%d", shared.GetFreePort()))
			tc.ingressAddrs = append(tc.ingressAddrs, fmt.Sprintf("localhost:%d", shared.GetFreePort()))
			tc.internodeAddrs = append(tc.internodeAddrs, fmt.Sprintf("localhost:%d", shared.GetFreePort()))
			tc.healthPorts = append(tc.healthPorts, strconv.Itoa(shared.GetFreePort()))
		}

		performOnAllNodes(tc, startNode)

		localPromAPIClient, _ := prom_api_client.NewClient(prom_api_client.Config{
			Address: fmt.Sprintf("https://%s", tc.addrs[0]),
			RoundTripper: &http.Transport{
				TLSClientConfig: tc.tlsConfig,
			},
		})
		tc.localEgressClient = prom_versioned_api_client.NewAPI(localPromAPIClient)

		return tc, func() {
			performOnAllNodes(tc, stopNode)
			for i := 0; i < numNodes; i++ {
				os.RemoveAll(storagePaths[i])
			}
		}
	}

	var WithOptionRulesPath = func(path string) WithTestContextOption {
		return func(tc *testContext) {
			tc.rulesPath = path
		}
	}

	var WithOptionAlertManagerAddr = func(addr string) WithTestContextOption {
		return func(tc *testContext) {
			tc.alertmanagerAddr = addr
		}
	}

	var makeInstantQuery = func(tc *testContext, query testInstantQuery) (model.Value, error) {
		value, _, err := tc.localEgressClient.Query(context.Background(), query.Query, query.Timestamp())
		return value, err
	}

	var makeRangeQuery = func(tc *testContext, query testRangeQuery) (model.Value, error) {
		value, _, err := tc.localEgressClient.QueryRange(context.Background(), query.Query, query.Range())
		return value, err
	}

	var makeSeriesQuery = func(tc *testContext, query testSeriesQuery) ([]model.LabelSet, error) {
		value, _, err := tc.localEgressClient.Series(context.Background(), query.Match, query.StartTimestamp(), query.EndTimestamp())
		return value, err
	}

	type testPoint struct {
		Name               string
		TimeInMilliseconds int64
		Value              float64
		Labels             map[string]string
	}

	var optimisticallyWritePoints = func(tc *testContext, points []testPoint) map[string]int {
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

		client, err := ingressclient.NewIngressClient(
			tc.ingressAddrs[0],
			tc.tlsConfig,
		)
		Expect(err).ToNot(HaveOccurred())
		defer client.Close()

		err = client.Write(rpcPoints)
		Expect(err).ToNot(HaveOccurred())

		return metricNameCounts
	}

	var writePoints = func(tc *testContext, points []testPoint) {
		metricNameCounts := optimisticallyWritePoints(tc, points)

		if tc.metricStoreProcesses[0].ExitCode() == -1 {
			Eventually(func() bool {
				value, _, _ := tc.localEgressClient.LabelValues(context.Background(), model.MetricNameLabel)
				return len(value) == len(metricNameCounts)
			}, 3).Should(BeTrue())
		}
	}

	Context("with a single node", func() {
		It("deletes shards with old data when Metric Store starts", func() {
			tc, cleanup := setup(1)
			defer cleanup()

			now := time.Now()
			Eventually(func() model.LabelValues {
				writePoints(
					tc,
					[]testPoint{
						{
							Name:               "metric_name_old",
							TimeInMilliseconds: 1000,
						},
						{
							Name:               "metric_name_new",
							TimeInMilliseconds: now.UnixNano() / int64(time.Millisecond),
						},
					},
				)

				value, _, err := tc.localEgressClient.LabelValues(context.Background(), model.MetricNameLabel)
				if err != nil {
					return nil
				}
				return value
			}, 5).Should(
				Or(
					Equal(model.LabelValues{"metric_name_old", "metric_name_new"}),
					Equal(model.LabelValues{"metric_name_new", "metric_name_old"}),
				),
			)

			stopNode(tc, 0)

			startNode(tc, 0)

			Eventually(func() error {
				_, _, err := tc.localEgressClient.LabelValues(context.Background(), model.MetricNameLabel)
				return err
			}, 5).Should(Succeed())

			Eventually(func() model.LabelValues {
				value, _, err := tc.localEgressClient.LabelValues(context.Background(), model.MetricNameLabel)
				if err != nil {
					return nil
				}
				return value
			}, 1).Should(Equal(model.LabelValues{
				"metric_name_new",
			}))
		})
	})

	Context("when the health check endpoint is called", func() {
		It("returns information about metrics store", func() {
			tc, cleanup := setup(1)
			defer cleanup()

			client := &http.Client{Transport: &http.Transport{TLSClientConfig: tc.tlsConfig}}

			var resp *http.Response
			Eventually(func() error {
				var err error
				resp, err = client.Get("https://" + tc.addrs[0] + "/health")
				return err
			}).Should(BeNil())

			Expect(resp.StatusCode).To(Equal(200))
			defer resp.Body.Close()

			value, err := ioutil.ReadAll(resp.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(string(value)).To(MatchJSON(
				fmt.Sprintf(`{ "version":"%s", "sha": "dev" }`, version.VERSION),
			))
		})
	})

	Context("as a cluster of nodes", func() {
		It("replays data when a node comes back online", func() {
			tc, cleanup := setup(2)
			defer cleanup()

			stopNode(tc, 1)

			optimisticallyWritePoints(
				tc,
				[]testPoint{
					{Name: MAGIC_MEASUREMENT_PEER_NAME, TimeInMilliseconds: 1},
					{Name: MAGIC_MEASUREMENT_PEER_NAME, TimeInMilliseconds: 2},
					{Name: MAGIC_MEASUREMENT_PEER_NAME, TimeInMilliseconds: 3},
					{Name: MAGIC_MEASUREMENT_PEER_NAME, TimeInMilliseconds: 4},
				},
			)

			startNode(tc, 1)

			Eventually(func() (err error) {
				_, _, err = tc.localEgressClient.LabelNames(context.Background())
				return
			}, 5).Should(Succeed())

			Eventually(func() []int64 {
				client, err := shared_api.NewPromHTTPClient(
					tc.addrs[1],
					"",
					tc.tlsConfig,
				)
				if err != nil {
					return nil
				}

				value, _, err := client.Query(
					context.Background(),
					fmt.Sprintf("%s[60s]", MAGIC_MEASUREMENT_PEER_NAME),
					time.Unix(1, 0),
				)
				if err != nil {
					return nil
				}

				var result []int64
				switch serieses := value.(type) {
				case model.Matrix:
					if len(serieses) != 1 {
						return nil
					}
					for _, point := range serieses[0].Values {
						result = append(result, point.Timestamp.UnixNano())
					}
				default:
					return nil
				}

				return result
			}, 5).Should(
				And(
					ContainElement(int64(1*time.Millisecond)),
					ContainElement(int64(2*time.Millisecond)),
					ContainElement(int64(3*time.Millisecond)),
					ContainElement(int64(4*time.Millisecond)),
				),
			)
		})
	})

	Context("when using HTTP", func() {
		Context("when a instant query is made", func() {
			It("returns locally stored metrics from a simple query", func() {
				tc, cleanup := setup(1)
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

				value, err := makeInstantQuery(tc, testInstantQuery{
					Query:         "metric_name",
					TimeInSeconds: "2",
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(value).To(Equal(
					model.Vector{
						&model.Sample{
							Metric: model.Metric{
								model.MetricNameLabel: "metric_name",
								"source_id":           "1",
							},
							Value:     99.0,
							Timestamp: 2000,
						},
					},
				))
			})

			It("returns remotely stored metrics from a simple query", func() {
				tc, cleanup := setup(2)
				defer cleanup()

				writePoints(
					tc,
					[]testPoint{
						{
							Name:               MAGIC_MEASUREMENT_PEER_NAME,
							Value:              99,
							TimeInMilliseconds: 1000,
							Labels: map[string]string{
								"source_id": "1",
							},
						},
					},
				)

				value, err := makeInstantQuery(tc, testInstantQuery{
					Query:         MAGIC_MEASUREMENT_PEER_NAME,
					TimeInSeconds: "2",
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(value).To(Equal(
					model.Vector{
						&model.Sample{
							Metric: model.Metric{
								model.MetricNameLabel: MAGIC_MEASUREMENT_PEER_NAME,
								"source_id":           "1",
							},
							Value:     99.0,
							Timestamp: 2000,
						},
					},
				))
			})

			It("a complex query where some metric names are local and some are remote", func() {
				tc, cleanup := setup(2)
				defer cleanup()

				writePoints(
					tc,
					[]testPoint{
						{
							Name:               "metric_name_for_node_0",
							Value:              99,
							TimeInMilliseconds: 1000,
							Labels: map[string]string{
								"source_id": "1",
							},
						},
						{
							Name:               "metric_name_for_node_1",
							Value:              99,
							TimeInMilliseconds: 1000,
							Labels: map[string]string{
								"source_id": "1",
							},
						},
					},
				)

				value, err := makeInstantQuery(tc, testInstantQuery{
					Query:         "metric_name_for_node_0+metric_name_for_node_1",
					TimeInSeconds: "2",
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(value).To(Equal(
					model.Vector{
						&model.Sample{
							Metric: model.Metric{
								"source_id": "1",
							},
							Value:     198.0,
							Timestamp: 2000,
						},
					},
				))
			})
		})

		Context("when a range query is made", func() {
			It("returns locally stored metrics from a simple query", func() {
				tc, cleanup := setup(1)
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

				value, err := makeRangeQuery(tc, testRangeQuery{
					Query:          "metric_name",
					StartInSeconds: "1",
					EndInSeconds:   "5",
					StepDuration:   "2s",
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(value).To(Equal(
					model.Matrix{
						&model.SampleStream{
							Metric: model.Metric{
								model.MetricNameLabel: "metric_name",
								"source_id":           "1",
							},
							Values: []model.SamplePair{
								{Value: 93.0, Timestamp: 3000},
								{Value: 88.0, Timestamp: 5000},
							},
						},
						&model.SampleStream{
							Metric: model.Metric{
								model.MetricNameLabel: "metric_name",
								"source_id":           "2",
							},
							Values: []model.SamplePair{
								{Value: 99.0, Timestamp: 5000},
							},
						},
					},
				))
			})

			It("returns remotely stored metrics from a simple range query", func() {
				tc, cleanup := setup(2)
				defer cleanup()

				writePoints(
					tc,
					[]testPoint{
						{
							Name:               "metric_name_for_node_1",
							Value:              99,
							TimeInMilliseconds: 1000,
							Labels: map[string]string{
								"source_id": "1",
							},
						},
					},
				)

				value, err := makeRangeQuery(tc, testRangeQuery{
					Query:          "metric_name_for_node_1",
					StartInSeconds: "1",
					EndInSeconds:   "2",
					StepDuration:   "1s",
				})

				Expect(err).ToNot(HaveOccurred())
				Expect(value).To(Equal(
					model.Matrix{
						&model.SampleStream{
							Metric: model.Metric{
								model.MetricNameLabel: "metric_name_for_node_1",
								"source_id":           "1",
							},
							Values: []model.SamplePair{
								{Value: 99.0, Timestamp: 1000},
								{Value: 99.0, Timestamp: 2000},
							},
						},
					},
				))
			})

			It("a complex query where some metric names are local and some are remote", func() {
				tc, cleanup := setup(2)
				defer cleanup()

				writePoints(
					tc,
					[]testPoint{
						{
							Name:               "metric_name_for_node_0",
							Value:              99,
							TimeInMilliseconds: 1500,
							Labels: map[string]string{
								"source_id": "1",
							},
						},
						{
							Name:               "metric_name_for_node_0",
							Value:              101,
							TimeInMilliseconds: 2100,
							Labels: map[string]string{
								"source_id": "1",
							},
						},
						{
							Name:               "metric_name_for_node_1",
							Value:              50,
							TimeInMilliseconds: 1600,
							Labels: map[string]string{
								"source_id": "1",
							},
						},
					},
				)

				value, err := makeRangeQuery(tc, testRangeQuery{
					Query:          "metric_name_for_node_0 + metric_name_for_node_1",
					StartInSeconds: "1",
					EndInSeconds:   "3",
					StepDuration:   "1s",
				})
				Expect(err).ToNot(HaveOccurred())

				Expect(value).To(Equal(
					model.Matrix{
						&model.SampleStream{
							Metric: model.Metric{
								"source_id": "1",
							},
							Values: []model.SamplePair{
								{Value: 149.0, Timestamp: 2000},
								{Value: 151.0, Timestamp: 3000},
							},
						},
					},
				))
			})
		})
	})

	Context("when a labels query is made", func() {
		It("returns labels from Metric Store aggregated across nodes", func() {
			tc, cleanup := setup(2)
			defer cleanup()

			writePoints(
				tc,
				[]testPoint{
					{
						Name:               "metric_name_for_node_0",
						TimeInMilliseconds: 1,
						Labels: map[string]string{
							"source_id":  "1",
							"user_agent": "phil",
						},
					},
					{
						Name:               "metric_name_for_node_1",
						TimeInMilliseconds: 2,
						Labels: map[string]string{
							"source_id":      "2",
							"content_length": "42",
						},
					},
				},
			)

			value, _, err := tc.localEgressClient.LabelNames(context.Background())
			Expect(err).ToNot(HaveOccurred())
			Expect(value).To(Equal(
				[]string{model.MetricNameLabel, "source_id"},
			))
		})
	})

	Context("when a label values query is made", func() {
		It("returns values for a label name", func() {
			tc, cleanup := setup(2)
			defer cleanup()

			writePoints(
				tc,
				[]testPoint{
					{
						Name:               "metric_name_for_node_0",
						TimeInMilliseconds: 1,
						Labels: map[string]string{
							"source_id":  "1",
							"user_agent": "100",
						},
					},
					{
						Name:               "metric_name_for_node_1",
						TimeInMilliseconds: 2,
						Labels: map[string]string{
							"source_id":  "10",
							"user_agent": "200",
						},
					},
					{
						Name:               "metric_name_for_node_2",
						TimeInMilliseconds: 3,
						Labels: map[string]string{
							"source_id":  "10",
							"user_agent": "100",
						},
					},
				},
			)

			value, _, err := tc.localEgressClient.LabelValues(context.Background(), "source_id")
			Expect(err).ToNot(HaveOccurred())
			Expect(value).To(Equal(
				model.LabelValues{"1", "10"},
			))

			value, _, err = tc.localEgressClient.LabelValues(context.Background(), "user_agent")
			Expect(err).ToNot(HaveOccurred())
			Expect(value).To(Equal(model.LabelValues{}))

			value, _, err = tc.localEgressClient.LabelValues(context.Background(), model.MetricNameLabel)
			Expect(err).ToNot(HaveOccurred())
			Expect(value).To(Equal(
				model.LabelValues{"metric_name_for_node_0", "metric_name_for_node_1", "metric_name_for_node_2"},
			))
		})
	})

	Context("when a series query is made", func() {
		It("returns locally stored metrics from a simple query", func() {
			tc, cleanup := setup(1)
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

			value, err := makeSeriesQuery(tc, testSeriesQuery{
				Match:          []string{"metric_name"},
				StartInSeconds: "1",
				EndInSeconds:   "2",
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(value).To(Equal(
				[]model.LabelSet{
					{
						model.MetricNameLabel: "metric_name",
						"source_id":           "1",
					},
				},
			))
		})

		It("returns remotely stored metrics from a simple query", func() {
			tc, cleanup := setup(2)
			defer cleanup()

			writePoints(
				tc,
				[]testPoint{
					{
						Name:               "metric_name_for_node_1",
						Value:              99,
						TimeInMilliseconds: 1000,
						Labels: map[string]string{
							"source_id": "1",
						},
					},
				},
			)

			value, err := makeSeriesQuery(tc, testSeriesQuery{
				Match:          []string{"metric_name_for_node_1"},
				StartInSeconds: "1",
				EndInSeconds:   "2",
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(value).To(Equal(
				[]model.LabelSet{
					{
						model.MetricNameLabel: "metric_name_for_node_1",
						"source_id":           "1",
					},
				},
			))
		})

		It("a complex query where some metric names are local and some are remote", func() {
			tc, cleanup := setup(2)
			defer cleanup()

			writePoints(
				tc,
				[]testPoint{
					{
						Name:               "metric_name_for_node_0",
						Value:              99,
						TimeInMilliseconds: 1000,
						Labels: map[string]string{
							"source_id": "1",
						},
					},
					{
						Name:               "metric_name_for_node_1",
						Value:              99,
						TimeInMilliseconds: 1000,
						Labels: map[string]string{
							"source_id": "1",
						},
					},
				},
			)

			value, err := makeSeriesQuery(tc, testSeriesQuery{
				Match:          []string{"metric_name_for_node_0", "metric_name_for_node_1"},
				StartInSeconds: "1",
				EndInSeconds:   "2",
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(value).To(Equal(
				[]model.LabelSet{
					{
						model.MetricNameLabel: "metric_name_for_node_0",
						"source_id":           "1",
					},
					{
						model.MetricNameLabel: "metric_name_for_node_1",
						"source_id":           "1",
					},
				},
			))
		})
	})

	It("exposes internal metrics", func() {
		tc, cleanup := setup(2)
		defer cleanup()

		writePoints(
			tc,
			[]testPoint{
				{
					Name:               MAGIC_MEASUREMENT_PEER_NAME,
					Value:              99,
					TimeInMilliseconds: 1000,
					Labels: map[string]string{
						"source_id": "1",
					},
				},
			},
		)

		resp, err := http.Get("http://localhost:" + tc.healthPorts[0] + "/metrics")
		Expect(err).NotTo(HaveOccurred())
		defer resp.Body.Close()

		bytes, err := ioutil.ReadAll(resp.Body)
		Expect(err).NotTo(HaveOccurred())
		body := string(bytes)

		Expect(body).To(ContainSubstring(metrics.MetricStoreDistributedPointsTotal))
		Expect(body).To(ContainSubstring("go_threads"))
	})

	It("processes recording rules to record metrics", func() {
		rules_yml := []byte(`
---
groups:
- name: example
  interval: 1s
  rules:
  - record: testRecordingRule
    expr: avg(metric_store_test_metric) by (node)
    labels:
      foo: bar
`)

		tmpfile, err := ioutil.TempFile("", "rules_yml")
		Expect(err).NotTo(HaveOccurred())
		defer os.Remove(tmpfile.Name())
		if _, err := tmpfile.Write(rules_yml); err != nil {
			log.Fatal(err)
		}
		if err := tmpfile.Close(); err != nil {
			log.Fatal(err)
		}

		tc, cleanup := setup(
			1,
			WithOptionRulesPath(tmpfile.Name()),
		)
		defer cleanup()

		client, err := ingressclient.NewIngressClient(
			tc.ingressAddrs[0],
			tc.tlsConfig,
		)
		pointTimestamp := time.Now().UnixNano()
		points := []*rpc.Point{
			{
				Name:      "metric_store_test_metric",
				Timestamp: pointTimestamp,
				Value:     1,
				Labels: map[string]string{
					"node":  "1",
					"other": "foo",
				},
			},
			{
				Name:      "metric_store_test_metric",
				Timestamp: pointTimestamp,
				Value:     11,
				Labels: map[string]string{
					"node":  "1",
					"other": "bar",
				},
			},
			{
				Name:      "metric_store_test_metric",
				Timestamp: pointTimestamp,
				Value:     2,
				Labels: map[string]string{
					"node":  "2",
					"other": "foo",
				},
			},
			{
				Name:      "metric_store_test_metric",
				Timestamp: pointTimestamp,
				Value:     12,
				Labels: map[string]string{
					"node":  "2",
					"other": "bar",
				},
			},
			{
				Name:      "metric_store_test_metric",
				Timestamp: pointTimestamp,
				Value:     3,
				Labels: map[string]string{
					"node":  "3",
					"other": "foo",
				},
			},
			{
				Name:      "metric_store_test_metric",
				Timestamp: pointTimestamp,
				Value:     13,
				Labels: map[string]string{
					"node":  "3",
					"other": "bar",
				},
			},
		}
		err = client.Write(points)

		checkForRecordedMetric := func() bool {
			queryTimestamp := time.Now().UnixNano()
			value, err := makeInstantQuery(tc, testInstantQuery{
				Query:         "testRecordingRule",
				TimeInSeconds: strconv.Itoa(int(queryTimestamp / int64(time.Second))),
			})
			Expect(err).ToNot(HaveOccurred())

			samples := value.(model.Vector)
			if len(samples) == 0 {
				return false
			}

			sample := &model.Sample{}
			for _, s := range samples {
				if s.Metric["node"] != "1" {
					continue
				}

				sample = s
				break
			}

			if int64(sample.Timestamp) < transform.NanosecondsToMilliseconds(pointTimestamp) {
				return false
			}

			if int64(sample.Timestamp) > transform.NanosecondsToMilliseconds(queryTimestamp) {
				return false
			}

			if sample.Value != 6 {
				return false
			}

			expectedMetric := model.Metric{
				model.MetricNameLabel: "testRecordingRule",
				"foo":                 "bar",
				"node":                "1",
			}
			if !sample.Metric.Equal(expectedMetric) {
				return false
			}

			return true
		}
		Eventually(checkForRecordedMetric, 5).Should(BeTrue())
	})

	It("processes alerting rules to trigger alerts", func() {
		rules_yml := []byte(`
---
groups:
- name: example
  rules:
  - alert: HighNumberOfTestMetrics
    expr: metric_store_test_metric > 2
    for: 1s
    labels:
      severity: page
    annotations:
      summary: High Test Metric Count
      description: this is a good thing
`)

		tmpfile, err := ioutil.TempFile("", "rules_yml")
		Expect(err).NotTo(HaveOccurred())
		defer os.Remove(tmpfile.Name())
		if _, err := tmpfile.Write(rules_yml); err != nil {
			log.Fatal(err)
		}
		if err := tmpfile.Close(); err != nil {
			log.Fatal(err)
		}

		receivedAlertCountChan := make(chan int)
		f := func(w http.ResponseWriter, r *http.Request) {
			var receivedAlerts []*notifier.Alert
			defer r.Body.Close()

			err := json.NewDecoder(r.Body).Decode(&receivedAlerts)
			Expect(err).NotTo(HaveOccurred())

			receivedAlertCountChan <- len(receivedAlerts)
		}

		spyAlertManager := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			f(w, r)
			w.WriteHeader(http.StatusOK)
		}))
		spyAlertManagerAddr, _ := url.Parse(spyAlertManager.URL)

		tc, cleanup := setup(
			1,
			WithOptionRulesPath(tmpfile.Name()),
			WithOptionAlertManagerAddr(fmt.Sprintf(":%s", spyAlertManagerAddr.Port())),
		)
		defer cleanup()
		defer spyAlertManager.Close()

		client, err := ingressclient.NewIngressClient(
			tc.ingressAddrs[0],
			tc.tlsConfig,
		)
		points := []*rpc.Point{
			{
				Name:      "metric_store_test_metric",
				Timestamp: time.Now().UnixNano(),
				Value:     1,
				Labels: map[string]string{
					"node": "1",
				},
			},
			{
				Name:      "metric_store_test_metric",
				Timestamp: time.Now().UnixNano(),
				Value:     2,
				Labels: map[string]string{
					"node": "2",
				},
			},
			{
				Name:      "metric_store_test_metric",
				Timestamp: time.Now().UnixNano(),
				Value:     3,
				Labels: map[string]string{
					"node": "3",
				},
			},
		}
		err = client.Write(points)

		checkCount := func() bool {
			select {
			case count := <-receivedAlertCountChan:
				return count > 0
			default:
				return false
			}
		}
		Eventually(checkCount, 20).Should(BeTrue())
	})
})
