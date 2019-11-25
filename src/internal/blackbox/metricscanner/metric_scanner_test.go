package metricscanner_test

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/cloudfoundry/metric-store-release/src/internal/logger"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/prometheus/client_golang/api"
	"github.com/prometheus/common/model"

	"github.com/cloudfoundry/metric-store-release/src/internal/blackbox"
	. "github.com/cloudfoundry/metric-store-release/src/internal/blackbox/metricscanner"
)

type mockRegistrar struct {
	IncCalledWith string
}

func (mr *mockRegistrar) Inc(metricName string, _ ...string) {
	mr.IncCalledWith = metricName
}

type testContext struct {
	client    *mockMetricStoreClient
	registrar *mockRegistrar
	scanner   *MetricScanner
}

func setup() *testContext {
	registrar := mockRegistrar{}
	client := mockMetricStoreClient{}
	scanner := NewMetricScanner(&client, &registrar, logger.NewTestLogger())

	return &testContext{
		client:    &client,
		registrar: &registrar,
		scanner:   scanner,
	}
}

type mockMetricStoreClient struct {
	queries []string

	listMetricNameError error
	queriesToError      map[string]error
}

func (client *mockMetricStoreClient) LabelValues(_ context.Context, labelName string) (model.LabelValues, api.Warnings, error) {
	if client.listMetricNameError != nil {
		return nil, nil, client.listMetricNameError
	}

	return model.LabelValues{"metricA", "metricB"}, nil, nil
}

func (client *mockMetricStoreClient) Query(_ context.Context, query string, ts time.Time) (model.Value, api.Warnings, error) {
	client.queries = append(client.queries, query)

	for search, err := range client.queriesToError {
		if strings.Contains(query, search) {
			return nil, nil, err
		}
	}
	return nil, nil, nil
}

var _ = Describe("MetricScanner", func() {
	It("Checks all available metrics", func() {
		tc := setup()
		tc.scanner.TestCurrentMetrics()

		Expect(tc.client.queries).To(ContainElement(ContainSubstring("count(metricA)")))
		Expect(tc.client.queries).To(ContainElement(ContainSubstring("count(metricB)")))
	})

	It("reports error on querying metrics", func() {
		tc := setup()
		tc.client.queriesToError = map[string]error{"metricA": errors.New("oops")}
		err := tc.scanner.TestCurrentMetrics()

		Expect(err).ToNot(HaveOccurred())
		Expect(tc.registrar.IncCalledWith).To(Equal(blackbox.MalfunctioningMetricsTotal))
	})

	Describe("handles errors with metric store", func() {
		It("handles error on listing metric names", func() {
			tc := setup()
			tc.client.listMetricNameError = errors.New("something went terribly wrong")

			err := tc.scanner.TestCurrentMetrics()

			Expect(err).To(HaveOccurred())
		})

	})
})
