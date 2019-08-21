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
	"sync"
	"time"

	"github.com/cloudfoundry/metric-store-release/src/pkg/ingressclient"
	"github.com/cloudfoundry/metric-store-release/src/pkg/metricstore"
	"github.com/cloudfoundry/metric-store-release/src/pkg/persistence/transform"
	rpc "github.com/cloudfoundry/metric-store-release/src/pkg/rpc/metricstore_v1"
	sharedtls "github.com/cloudfoundry/metric-store-release/src/pkg/tls"
	prom_api_client "github.com/prometheus/client_golang/api"
	prom_versioned_api_client "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/expfmt"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/notifier"

	. "github.com/cloudfoundry/metric-store-release/src/pkg/matchers"
	"github.com/cloudfoundry/metric-store-release/src/pkg/testing"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

// Sentinel to detect build failures early
var __ *metricstore.MetricStore

var storagePath = "/tmp/metric-store-node"

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

var _ = Describe("MetricStore", func() {
	type testContext struct {
		addr               string
		ingressAddr        string
		healthPort         string
		metricStoreProcess *gexec.Session
		tlsConfig          *tls.Config
		tlsConfigLocal     *tls.Config

		rulesPath        string
		alertmanagerAddr string
		egressClient     prom_versioned_api_client.API
	}

	var start = func(tc *testContext) {
		caCert := testing.Cert("metric-store-ca.crt")
		cert := testing.Cert("metric-store.crt")
		key := testing.Cert("metric-store.key")

		tlsConfig, err := sharedtls.NewMutualTLSConfig(caCert, cert, key, "metric-store")
		if err != nil {
			fmt.Printf("ERROR: invalid mutal TLS config: %s\n", err)
		}
		tc.tlsConfig = tlsConfig

		localCert := testing.Cert("localhost.crt")
		localKey := testing.Cert("localhost.key")

		tlsConfigLocal, err := sharedtls.NewMutualTLSConfig(caCert, localCert, localKey, "localhost")
		if err != nil {
			fmt.Printf("ERROR: invalid mutal TLS config: %s\n", err)
		}
		tc.tlsConfigLocal = tlsConfigLocal

		tc.metricStoreProcess = testing.StartGoProcess(
			"github.com/cloudfoundry/metric-store-release/src/cmd/metric-store",
			[]string{
				"ADDR=" + tc.addr,
				"INGRESS_ADDR=" + tc.ingressAddr,
				"HEALTH_PORT=" + tc.healthPort,
				"STORAGE_PATH=" + storagePath,
				"RETENTION_PERIOD_IN_DAYS=1",
				"CA_PATH=" + caCert,
				"CERT_PATH=" + localCert,
				"KEY_PATH=" + localKey,
				"METRIC_STORE_SERVER_CA_PATH=" + caCert,
				"METRIC_STORE_SERVER_CERT_PATH=" + cert,
				"METRIC_STORE_SERVER_KEY_PATH=" + key,
				"RULES_PATH=" + tc.rulesPath,
				"ALERTMANAGER_ADDR=" + tc.alertmanagerAddr,
			},
		)

		testing.WaitForHealthCheck(tc.healthPort)
	}

	var stop = func(tc *testContext) {
		tc.metricStoreProcess.Kill()
		Eventually(tc.metricStoreProcess.Exited).Should(BeClosed())
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

	type WithTestContextOption func(*testContext)

	var setup = func(opts ...WithTestContextOption) (*testContext, func()) {
		tc := &testContext{}

		for _, opt := range opts {
			opt(tc)
		}

		tc.addr = fmt.Sprintf("localhost:%d", testing.GetFreePort())
		tc.ingressAddr = fmt.Sprintf("localhost:%d", testing.GetFreePort())
		tc.healthPort = strconv.Itoa(testing.GetFreePort())

		perform(tc, start)

		promAPIClient, _ := prom_api_client.NewClient(prom_api_client.Config{
			Address: fmt.Sprintf("https://%s", tc.addr),
			RoundTripper: &http.Transport{
				TLSClientConfig: tc.tlsConfigLocal,
			},
		})
		tc.egressClient = prom_versioned_api_client.NewAPI(promAPIClient)

		return tc, func() {
			perform(tc, stop)
			os.RemoveAll(storagePath)
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
		value, _, err := tc.egressClient.Query(context.Background(), query.Query, query.Timestamp())
		return value, err
	}

	var makeRangeQuery = func(tc *testContext, query testRangeQuery) (model.Value, error) {
		value, _, err := tc.egressClient.QueryRange(context.Background(), query.Query, query.Range())
		return value, err
	}

	var makeSeriesQuery = func(tc *testContext, query testSeriesQuery) ([]model.LabelSet, error) {
		value, _, err := tc.egressClient.Series(context.Background(), query.Match, query.StartTimestamp(), query.EndTimestamp())
		return value, err
	}

	type testPoint struct {
		Name               string
		TimeInMilliseconds int64
		Value              float64
		Labels             map[string]string
	}

	var writePoints = func(tc *testContext, points []testPoint) {
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
			tc.ingressAddr,
			tc.tlsConfig,
		)
		defer client.Close()

		err = client.Write(rpcPoints)
		Expect(err).ToNot(HaveOccurred())

		Eventually(func() bool {
			value, _ := tc.egressClient.LabelValues(context.Background(), model.MetricNameLabel)
			return len(value) == len(metricNameCounts)
		}, 3).Should(BeTrue())
	}

	It("deletes shards with old data when Metric Store starts", func() {
		tc, cleanup := setup()
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

			value, err := tc.egressClient.LabelValues(context.Background(), model.MetricNameLabel)
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

		stop(tc)

		start(tc)

		Eventually(func() error {
			_, err := tc.egressClient.LabelValues(context.Background(), model.MetricNameLabel)
			return err
		}, 5).Should(Succeed())

		Eventually(func() model.LabelValues {
			value, err := tc.egressClient.LabelValues(context.Background(), model.MetricNameLabel)
			Expect(err).ToNot(HaveOccurred())
			return value
		}, 1).Should(Equal(model.LabelValues{
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

			value, err := tc.egressClient.LabelNames(context.Background())
			Expect(err).ToNot(HaveOccurred())
			Expect(value).To(Equal(
				[]string{model.MetricNameLabel, "source_id"},
			))
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

			value, err := tc.egressClient.LabelValues(context.Background(), "source_id")
			Expect(err).ToNot(HaveOccurred())
			Expect(value).To(Equal(
				model.LabelValues{"1", "10"},
			))

			value, err = tc.egressClient.LabelValues(context.Background(), "user_agent")
			Expect(err).ToNot(HaveOccurred())
			Expect(value).To(BeNil())

			value, err = tc.egressClient.LabelValues(context.Background(), model.MetricNameLabel)
			Expect(err).ToNot(HaveOccurred())
			Expect(value).To(Equal(
				model.LabelValues{"metric_name_0", "metric_name_1", "metric_name_2"},
			))
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

			value, err := makeSeriesQuery(tc, testSeriesQuery{
				Match:          []string{"metric_name"},
				StartInSeconds: "1",
				EndInSeconds:   "2",
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(value).To(Equal(
				[]model.LabelSet{
					model.LabelSet{
						model.MetricNameLabel: "metric_name",
						"source_id":           "1",
					},
				},
			))
		})
	})

	It("exposes metrics in prometheus format", func() {
		tc, cleanup := setup()
		defer cleanup()

		writePoints(
			tc,
			[]testPoint{
				{
					Name:               "metric_name",
					TimeInMilliseconds: 1000,
				},
			},
		)

		resp, err := http.Get(fmt.Sprintf("http://localhost:%s/metrics", tc.healthPort))
		Expect(err).ToNot(HaveOccurred())
		Expect(resp.StatusCode).To(Equal(http.StatusOK))

		var parser expfmt.TextParser
		parsed, err := parser.TextToMetricFamilies(resp.Body)
		Expect(err).ToNot(HaveOccurred())

		Expect(parsed).To(ContainCounterMetric("metric_store_ingress", float64(1)))
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
			WithOptionRulesPath(tmpfile.Name()),
			WithOptionAlertManagerAddr(fmt.Sprintf(":%s", spyAlertManagerAddr.Port())),
		)
		defer cleanup()
		defer spyAlertManager.Close()

		client, err := ingressclient.NewIngressClient(
			tc.ingressAddr,
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
