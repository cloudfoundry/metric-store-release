package matchers

import (
	"fmt"

	"github.com/onsi/gomega/types"
	"github.com/prometheus/client_golang/prometheus"
	io_prometheus_client "github.com/prometheus/client_model/go"
)

func ContainCounterMetric(expectedName string, expectedValue float64) types.GomegaMatcher {
	return &containMetricMatcher{
		expectedName:  expectedName,
		expectedValue: expectedValue,
		valueExtractor: func(metric *io_prometheus_client.Metric) float64 {
			return metric.GetCounter().GetValue()
		},
	}
}

func ContainGaugeMetric(expectedName, expectedUnit string, expectedValue float64) types.GomegaMatcher {
	return &containMetricMatcher{
		expectedName:  expectedName,
		expectedValue: expectedValue,
		expectedUnit:  expectedUnit,
		valueExtractor: func(metric *io_prometheus_client.Metric) float64 {
			return metric.GetGauge().GetValue()
		},
	}
}

type containMetricMatcher struct {
	expectedName   string
	expectedValue  float64
	expectedUnit   string
	valueExtractor func(*io_prometheus_client.Metric) float64
}

func (matcher *containMetricMatcher) Match(actual interface{}) (success bool, err error) {
	metrics, err := normalizeMetrics(actual)
	if err != nil {
		return false, err
	}

	for _, metricFamily := range metrics {
		if metricFamily.GetName() != matcher.expectedName {
			continue
		}
		for _, metric := range metricFamily.GetMetric() {
			if matcher.valueExtractor(metric) != matcher.expectedValue {
				continue
			}

			if matcher.expectedUnit != "" {
				for _, labelPair := range metric.GetLabel() {
					if labelPair.GetName() == "unit" && labelPair.GetValue() == matcher.expectedUnit {
						return true, nil
					}
				}

				continue
			}

			return true, nil
		}
	}

	return false, nil
}

func (matcher *containMetricMatcher) FailureMessage(actual interface{}) (message string) {
	metrics, err := normalizeMetrics(actual)
	if err != nil {
		return fmt.Sprintf("Expected registry to contain the metric\n\t%s: %f", matcher.expectedName, matcher.expectedValue)
	}

	var actualOutput string
	for _, a := range metrics {
		actualOutput += fmt.Sprintf("\t%+v\n", a)
	}
	expectedOutput := fmt.Sprintf("\tname: %s, value: %f", matcher.expectedName, matcher.expectedValue)
	if matcher.expectedUnit != "" {
		expectedOutput += fmt.Sprintf(", unit: %s", matcher.expectedUnit)
	}
	expectedOutput += "\n"
	return fmt.Sprintf("Expected\n%s\nto contain the metric \n%s", actualOutput, expectedOutput)
}

func (matcher *containMetricMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	metrics, err := normalizeMetrics(actual)
	if err != nil {
		return fmt.Sprintf("Expected registry not to contain the metric\n\t%s: %f", matcher.expectedName, matcher.expectedValue)
	}

	var actualOutput string
	for _, a := range metrics {
		actualOutput += fmt.Sprintf("\t%+v\n", a)
	}
	expectedOutput := fmt.Sprintf("\tname: %s, value: %f", matcher.expectedName, matcher.expectedValue)
	if matcher.expectedUnit != "" {
		expectedOutput += fmt.Sprintf(", unit: %s", matcher.expectedUnit)
	}
	expectedOutput += "\n"
	return fmt.Sprintf("Expected\n%s\nnot to contain the metric \n%s", actualOutput, expectedOutput)
}

func normalizeMetrics(actual interface{}) ([]*io_prometheus_client.MetricFamily, error) {
	var metrics []*io_prometheus_client.MetricFamily
	var err error

	switch v := actual.(type) {
	case *prometheus.Registry:
		metrics, err = v.Gather()
		if err != nil {
			return metrics, err
		}

	case map[string]*io_prometheus_client.MetricFamily:
		for _, m := range v {
			metrics = append(metrics, m)
		}

	default:
		return metrics, fmt.Errorf("unexpected actual: %+v", actual)
	}

	return metrics, nil
}
