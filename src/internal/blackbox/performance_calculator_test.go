package blackbox_test

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/metric-store-release/src/internal/blackbox"
	prom_api_client "github.com/prometheus/client_golang/api"
	prom_http_client "github.com/prometheus/client_golang/api"
	"github.com/prometheus/common/model"
)

var _ = Describe("Performance Calculator", func() {
	Describe("Calculate()", func() {
		It("calculates query benchmark metrics", func() {
			client := &mockPerfClient{}
			sourceID := "someSourceId"
			calc := blackbox.NewPerformanceCalculator(sourceID)
			pMetrics, err := calc.Calculate(client)

			Expect(err).ToNot(HaveOccurred())

			Expect(client.query).To(Equal(fmt.Sprintf(`sum(count_over_time(%s{source_id="%s"}[1w]))`, blackbox.BlackboxPerformanceTestCanary, sourceID)))
			Expect(int64(pMetrics.Latency)).Should(BeNumerically(">=", int64(10*time.Millisecond))) // TODO best way to validate latency?
			Expect(pMetrics.Magnitude).To(Equal(750000))

		})

		It("reports query errors", func() {
			client := &mockUnresponsiveClient{}
			sourceID := "someSourceId"
			calc := blackbox.NewPerformanceCalculator(sourceID)
			_, err := calc.Calculate(client)

			Expect(err).To(HaveOccurred())
		})
	})
})

type mockPerfClient struct {
	query string
}

func (c *mockPerfClient) Query(ctx context.Context, query string, ts time.Time) (model.Value, prom_api_client.Warnings, error) {
	c.query = query

	value := model.Vector{
		&model.Sample{
			Metric:    model.Metric{},
			Value:     750000,
			Timestamp: model.Now(),
		}}

	time.Sleep(10 * time.Millisecond)
	return value, nil, nil
}

func (c *mockPerfClient) LabelValues(context.Context, string) (model.LabelValues, prom_http_client.Warnings, error) {
	return nil, nil, fmt.Errorf("unexpected status code 500")
}
