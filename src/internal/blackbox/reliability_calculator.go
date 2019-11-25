package blackbox

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/cloudfoundry/metric-store-release/src/internal/logger"
	"github.com/cloudfoundry/metric-store-release/src/pkg/persistence/transform"
	"github.com/prometheus/common/model"
)

type ReliabilityCalculator struct {
	SampleInterval   time.Duration
	WindowInterval   time.Duration
	WindowLag        time.Duration
	EmissionInterval time.Duration
	SourceId         string
	Log              *logger.Logger
}

func (rc ReliabilityCalculator) Calculate(qc QueryableClient) (float64, error) {
	var totalReceivedCount uint64
	expectedEmissionCount := rc.ExpectedSamples()
	magicMetricNames := MagicMetricNames()
	var successfulClients int

	for _, metricName := range magicMetricNames {
		receivedCount, err := rc.CountMetricPoints(metricName, qc)
		if err != nil {
			rc.Log.Error("error counting metric points", err)
			continue
		}

		successfulClients++

		if float64(receivedCount) < expectedEmissionCount {
			rc.Log.Info(metricName,
				logger.Int("received", int64(receivedCount)),
				logger.Int("expected", int64(expectedEmissionCount)))
		}
		totalReceivedCount += receivedCount
	}

	if successfulClients == 0 {
		return 0, errors.New("No clients responded successfully")
	}

	return float64(totalReceivedCount) / float64(int(expectedEmissionCount)*successfulClients), nil
}

func (rc ReliabilityCalculator) ExpectedSamples() float64 {
	return rc.WindowInterval.Seconds() / rc.EmissionInterval.Seconds()
}

func (rc ReliabilityCalculator) printMissingSamples(points []model.SamplePair, queryTimestamp time.Time) {
	expectedTimestampsMap := make(map[int64]bool, int64(rc.ExpectedSamples()))
	queryTimestampInMillis := transform.NanosecondsToMilliseconds(queryTimestamp.UnixNano())
	intervalInMillis := transform.NanosecondsToMilliseconds(rc.EmissionInterval.Nanoseconds())

	for i := queryTimestampInMillis; i < int64(rc.ExpectedSamples()); i += intervalInMillis {
		expectedTimestampsMap[i] = false
	}

	for _, point := range points {
		expectedTimestampsMap[point.Timestamp.UnixNano()] = true
	}

	var missingTimestamps []int64
	for expectedTimestamp, found := range expectedTimestampsMap {
		if !found {
			missingTimestamps = append(missingTimestamps, expectedTimestamp)
		}
	}

	if len(missingTimestamps) > 0 {
		rc.Log.Info("WARNING: Missing points", logger.Count(len(missingTimestamps)), logger.Int("start time", queryTimestampInMillis))
		for _, missingTimestamp := range missingTimestamps {
			rc.Log.Info("missed point", logger.Int("timestamp", missingTimestamp))
		}
	}
}

func (rc ReliabilityCalculator) CountMetricPoints(metricName string, client QueryableClient) (uint64, error) {
	queryString := fmt.Sprintf(`%s{source_id="%s"}[%.0fs]`, metricName, rc.SourceId, rc.WindowInterval.Seconds())

	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	queryTimestamp := time.Now().Add(-rc.WindowLag).Truncate(rc.SampleInterval)

	queryResult, _, err := client.Query(ctx, queryString, queryTimestamp)
	if err != nil {
		rc.Log.Error("failed to count test metrics", err, logger.String("query", queryString))
		return 0, err
	}

	var series model.Matrix
	switch value := queryResult.(type) {
	case model.Matrix:
		series = value
	default:
		return 0, errors.New("expected queryResult to be a model.Matrix")
	}

	if len(series) == 0 {
		return 0, fmt.Errorf("couldn't find series for %s\n", queryString)
	}

	points := series[0].Values
	if len(points) == 0 {
		rc.Log.Info("did not find any points", logger.String("query result", queryResult.String()))
	}

	if len(points) < int(rc.ExpectedSamples()) {
		rc.printMissingSamples(points, queryTimestamp)
	}

	return uint64(len(points)), nil
}
