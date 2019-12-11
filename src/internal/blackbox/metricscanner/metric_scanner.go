package metricscanner

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cloudfoundry/metric-store-release/src/internal/blackbox"
	"github.com/cloudfoundry/metric-store-release/src/internal/logger"
)

type metrics interface {
	Inc(string, ...string)
}

type MetricScanner struct {
	storeClient blackbox.QueryableClient
	registrar   metrics
	log         *logger.Logger
}

func NewMetricScanner(client blackbox.QueryableClient, metricsRegistrar metrics, log *logger.Logger) *MetricScanner {
	return &MetricScanner{
		storeClient: client,
		registrar:   metricsRegistrar,
		log:         log,
	}
}

func (ms *MetricScanner) TestCurrentMetrics() error {
	metricNames, _, err := ms.storeClient.LabelValues(context.Background(), "__name__")
	ms.log.Info("metric scanner: name labels discovered", logger.Int("count", int64(len(metricNames))))

	if err != nil {
		return err
	}

	errorsFound := int64(0)

	ms.log.Info("metric scanner: query group started")

	startTime := time.Now()

	failingMetricNames := make([]string, 0)
	for _, metricName := range metricNames {
		query := fmt.Sprintf("count(%s)", string(metricName))
		_, _, err := ms.storeClient.Query(context.Background(), query, time.Now())
		if err != nil {
			ms.log.Debug("metric scanner: failed to query",
				logger.String("metric", string(metricName)),
				logger.String("error", err.Error()))
			ms.registrar.Inc(blackbox.MalfunctioningMetricsTotal)
			errorsFound++
			failingMetricNames = append(failingMetricNames, string(metricName))
		}
	}
	ms.log.Info("metric scanner: query group finished",
		logger.Int("errors found", errorsFound),
		logger.String("erroring metrics", strings.Join(failingMetricNames, ",")),
		logger.String("duration", time.Since(startTime).String()),
	)

	return nil
}
