package blackbox

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/cloudfoundry/metric-store-release/src/internal/metrics"
	"github.com/cloudfoundry/metric-store-release/src/pkg/ingressclient"
	"github.com/cloudfoundry/metric-store-release/src/pkg/logger"
	"github.com/cloudfoundry/metric-store-release/src/pkg/persistence/transform"
	"github.com/cloudfoundry/metric-store-release/src/pkg/rpc"
	"github.com/prometheus/common/model"
)

type ReliabilityCalculator struct {
	SampleInterval   time.Duration
	WindowInterval   time.Duration
	WindowLag        time.Duration
	EmissionInterval time.Duration
	SourceId         string
	Log              *logger.Logger
	DebugRegistrar   metrics.Registrar
}

const (
	// see scripts/hack/hash.go
	MAGIC_METRIC_NAME_NODES_0_AND_1 = "blackbox_test_metric_003"
	MAGIC_METRIC_NAME_NODES_1_AND_2 = "blackbox_test_metric_021"
	MAGIC_METRIC_NAME_NODES_2_AND_3 = "blackbox_test_metric_001"
	MAGIC_METRIC_NAME_NODES_3_AND_4 = "blackbox_test_metric_010"
	MAGIC_METRIC_NAME_NODES_4_AND_5 = "blackbox_test_metric_005"
	MAGIC_METRIC_NAME_NODES_5_AND_0 = "blackbox_test_metric_002"
)

func MagicMetricNames() []string {
	return []string{
		MAGIC_METRIC_NAME_NODES_0_AND_1,
		MAGIC_METRIC_NAME_NODES_1_AND_2,
		MAGIC_METRIC_NAME_NODES_2_AND_3,
		MAGIC_METRIC_NAME_NODES_3_AND_4,
		MAGIC_METRIC_NAME_NODES_4_AND_5,
		MAGIC_METRIC_NAME_NODES_5_AND_0,
	}
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

func (rc *ReliabilityCalculator) EmitReliabilityMetrics(ingressClient *ingressclient.IngressClient, stopChan chan bool) {
	var lastTimestamp time.Time
	var expectedTimestamp time.Time
	var timestamp time.Time

	rc.Log.Info("reliability: emitter started")

	for range time.NewTicker(rc.EmissionInterval).C {
		expectedTimestamp = lastTimestamp.Add(rc.EmissionInterval)
		timestamp = time.Now().Truncate(rc.EmissionInterval)

		if !lastTimestamp.IsZero() && expectedTimestamp != timestamp {
			rc.Log.Info("reliability: WARNING: an expected emission was missed", logger.String("missed", expectedTimestamp.String()), logger.String("sent", timestamp.String()))
		}

		rc.emitReliabilityMetrics(rc.SourceId, ingressClient, timestamp)
		lastTimestamp = timestamp

		select {
		case <-stopChan:
			return
		default:
		}
	}
}

func (rc *ReliabilityCalculator) emitReliabilityMetrics(sourceId string, client *ingressclient.IngressClient, timestamp time.Time) {
	var points []*rpc.Point

	for _, metric_name := range MagicMetricNames() {
		points = append(points, &rpc.Point{
			Timestamp: timestamp.UnixNano(),
			Name:      metric_name,
			Value:     10.0,
			Labels:    map[string]string{"source_id": sourceId},
		})
	}

	var err error
	if err != nil {
		rc.Log.Error("reliability: failed to marshal test metric points", err)
		return
	}

	// TODO is a reliability metric that hides errors behind retries valid?
	for retries := 5; retries > 0; retries-- {
		err = client.Write(points)

		if err == nil {
			break
		}

		time.Sleep(5 * time.Millisecond)
	}

	if err != nil {
		rc.Log.Error("reliability: failed to write test metric envelope", err)
	}
}

func (rc *ReliabilityCalculator) CalculateReliability(egressClient QueryableClient, stopChan chan bool) {
	rc.Log.Info("reliability: starting calculator")
	t := time.NewTicker(rc.SampleInterval)

	for range t.C {
		httpReliability, err := rc.Calculate(egressClient)
		if err != nil {
			rc.Log.Error("reliability: error calculating", err)
		}

		rc.DebugRegistrar.Set(BlackboxHTTPReliability, httpReliability)
		rc.Log.Info("reliability: ", logger.Float64("percent", httpReliability*100))
	}
}
