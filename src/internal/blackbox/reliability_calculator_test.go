package blackbox_test

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/cloudfoundry/metric-store-release/src/internal/blackbox"
	"github.com/cloudfoundry/metric-store-release/src/internal/logger"
	"github.com/cloudfoundry/metric-store-release/src/pkg/rpc"
	prom_http_client "github.com/prometheus/client_golang/api"
	"github.com/prometheus/common/model"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
)

var _ = Describe("ReliabilityCalculator", func() {
	Describe("Calculate()", func() {
		It("calculates reliabilities real good", func() {
			rc := blackbox.ReliabilityCalculator{
				EmissionInterval: time.Second,
				WindowInterval:   10 * time.Minute,
				WindowLag:        15 * time.Minute,
				SourceId:         "source-1",
				Log:              logger.NewNop(),
			}
			client := &mockClient{
				responseCounts: []int{597, 597, 597, 597, 597, 597},
				responseErrors: []error{nil, nil, nil, nil, nil, nil},
			}

			Expect(rc.Calculate(client)).To(BeNumerically("==", 0.9950))
		})

		It("returns an error when metric-store is unresponsive", func() {
			rc := blackbox.ReliabilityCalculator{
				EmissionInterval: time.Second,
				WindowInterval:   10 * time.Minute,
				WindowLag:        15 * time.Minute,
				SourceId:         "source-1",
				Log:              logger.NewNop(),
			}
			unresponsiveClient := &mockUnresponsiveClient{}

			_, err := rc.Calculate(unresponsiveClient)
			Expect(err).NotTo(BeNil())
		})

		Context("when a node returns a query error", func() {
			It("calculates reliability excluding the erring node", func() {
				rc := blackbox.ReliabilityCalculator{
					EmissionInterval: time.Second,
					WindowInterval:   10 * time.Minute,
					WindowLag:        15 * time.Minute,
					SourceId:         "source-1",
					Log:              logger.NewNop(),
				}

				client := &mockClient{
					responseCounts: []int{600, 0, 600, 600, 600, 600},
					responseErrors: []error{nil, errors.New("blah!"), nil, nil, nil, nil},
				}
				reliability, err := rc.Calculate(client)

				Expect(err).NotTo(HaveOccurred())
				Expect(reliability).To(BeNumerically("==", 1.0))
			})
		})
	})

	It("emits one test metric per emission interval per MagicMetricName", func() {
		tc := setup()
		defer tc.teardown()

		tc.waitGroup.Add(1)
		emissionInterval := 10 * time.Millisecond
		numberOfMagicMetricNames := len(blackbox.MagicMetricNames())
		expectedEmissionCount := int(tc.testDuration/emissionInterval) * numberOfMagicMetricNames

		rc := blackbox.ReliabilityCalculator{
			EmissionInterval: emissionInterval,
			WindowInterval:   10 * time.Minute,
			WindowLag:        15 * time.Minute,
			SourceId:         "source-1",
			Log:              logger.NewNop(),
		}
		startTime := time.Now().UnixNano()
		go func() {
			rc.EmitReliabilityMetrics(tc.client, tc.stop)
			tc.waitGroup.Done()
		}()

		var points []*rpc.Point
		Eventually(func() int {
			points = tc.metricStore.GetPoints()
			return len(points)
		}, tc.testDuration+tc.timingFudgeFactor).Should(BeNumerically(">=", expectedEmissionCount))

		for i, expected_metric_name := range blackbox.MagicMetricNames() {
			Expect(points[i]).To(PointTo(MatchFields(IgnoreExtras, Fields{
				"Timestamp": BeNumerically("~", startTime, int64(time.Second)),
				"Name":      Equal(expected_metric_name),
				"Value":     Equal(10.0),
				"Labels":    HaveKeyWithValue("source_id", "source-1"),
			})))
		}

		close(tc.stop)
		tc.waitGroup.Wait()
	})

})

type mockClient struct {
	responseCounts []int
	responseErrors []error
}

func (c *mockClient) Query(context.Context, string, time.Time) (model.Value, prom_http_client.Warnings, error) {
	var points []model.SamplePair

	responseCount := c.responseCounts[0]
	c.responseCounts = c.responseCounts[1:]

	responseError := c.responseErrors[0]
	c.responseErrors = c.responseErrors[1:]

	if responseError != nil {
		return nil, nil, responseError
	}

	ts := 0
	for i := 0; i <= 200; i++ {
		points = append(points, model.SamplePair{
			Timestamp: model.Time(int64(ts + i*1000)),
			Value:     10.0,
		})
	}

	// TODO: What did 201 ever do to you?

	for i := 202; i < (responseCount + 1); i++ {
		points = append(points, model.SamplePair{
			Timestamp: model.Time(int64(ts + i*1000)),
			Value:     10.0,
		})
	}

	return model.Matrix{
		&model.SampleStream{
			Metric: nil,
			Values: points,
		},
	}, nil, nil
}

func (c *mockClient) LabelValues(context.Context, string) (model.LabelValues, prom_http_client.Warnings, error) {
	return nil, nil, fmt.Errorf("unexpected status code 500")
}

type mockUnresponsiveClient struct {
}

func (c *mockUnresponsiveClient) Query(context.Context, string, time.Time) (model.Value, prom_http_client.Warnings, error) {
	return nil, nil, fmt.Errorf("unexpected status code 500")
}

func (c *mockUnresponsiveClient) LabelValues(context.Context, string) (model.LabelValues, prom_http_client.Warnings, error) {
	return nil, nil, nil
}
