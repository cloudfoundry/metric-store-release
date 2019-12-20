package blackbox

import (
	"context"
	"fmt"
	"time"

	"github.com/cloudfoundry/metric-store-release/src/internal/logger"
	"github.com/cloudfoundry/metric-store-release/src/pkg/ingressclient"
	"github.com/cloudfoundry/metric-store-release/src/pkg/rpc"
	"github.com/prometheus/client_golang/api"
	prom_api_client "github.com/prometheus/client_golang/api"
	"github.com/prometheus/common/model"
)

const (
	// https://play.golang.org/p/Qroq0HGXjdL
	MAGIC_METRIC_NAME_NODE_1_OF_6 = "blackbox_test_metric_001"
	MAGIC_METRIC_NAME_NODE_2_OF_6 = "blackbox_test_metric_002"
	MAGIC_METRIC_NAME_NODE_3_OF_6 = "blackbox_test_metric_005"
	MAGIC_METRIC_NAME_NODE_4_OF_6 = "blackbox_test_metric_007"
	MAGIC_METRIC_NAME_NODE_5_OF_6 = "blackbox_test_metric_021"
	MAGIC_METRIC_NAME_NODE_6_OF_6 = "blackbox_test_metric_003"
)

func MagicMetricNames() []string {
	return []string{
		MAGIC_METRIC_NAME_NODE_1_OF_6,
		MAGIC_METRIC_NAME_NODE_2_OF_6,
		MAGIC_METRIC_NAME_NODE_3_OF_6,
		MAGIC_METRIC_NAME_NODE_4_OF_6,
		MAGIC_METRIC_NAME_NODE_5_OF_6,
		MAGIC_METRIC_NAME_NODE_6_OF_6,
	}
}

type QueryableClient interface {
	Query(ctx context.Context, query string, ts time.Time) (model.Value, prom_api_client.Warnings, error)
	LabelValues(context.Context, string) (model.LabelValues, api.Warnings, error)
}

type Blackbox struct {
	log *logger.Logger
}

func NewBlackbox(log *logger.Logger) *Blackbox {
	return &Blackbox{
		log: log,
	}
}

func (b *Blackbox) StartEmittingPerformanceTestMetrics(sourceId string, emissionInterval time.Duration, ingressClient *ingressclient.IngressClient, stopChan chan bool, labels map[string][]string) {
	var lastTimestamp time.Time
	var expectedTimestamp time.Time
	var timestamp time.Time

	labelValueIndex := 0

	for range time.NewTicker(emissionInterval).C {
		expectedTimestamp = lastTimestamp.Add(emissionInterval)
		timestamp = time.Now().Truncate(emissionInterval)

		if !lastTimestamp.IsZero() && expectedTimestamp != timestamp {
			b.log.Info("WARNING: an expected performance emission was missed", logger.String("missed", expectedTimestamp.String()), logger.String("sent", timestamp.String()))
		}

		emitLabels := map[string]string{}
		for key, allValues := range labels {
			emitLabels[key] = allValues[labelValueIndex%len(allValues)]
		}
		labelValueIndex++

		b.emitPerformanceMetrics(sourceId, ingressClient, time.Now(), emitLabels)
		lastTimestamp = timestamp

		select {
		case <-stopChan:
			return
		default:
		}
	}
}

func (b *Blackbox) emitPerformanceMetrics(sourceId string, client *ingressclient.IngressClient, timestamp time.Time, labels map[string]string) {
	labels["source_id"] = sourceId

	points := []*rpc.Point{{
		Timestamp: timestamp.UnixNano(),
		Name:      BlackboxPerformanceTestCanary,
		Value:     10.0,
		Labels:    labels,
	}}

	err := client.Write(points)

	if err != nil {
		b.log.Error("failed to write test metric envelope", err)
	} else {
		b.log.Info(fmt.Sprintf("performance: wrote %d %s points", len(points), BlackboxPerformanceTestCanary))
	}
}

func (b *Blackbox) StartEmittingReliabilityMetrics(sourceId string, emissionInterval time.Duration, ingressClient *ingressclient.IngressClient, stopChan chan bool) {
	var lastTimestamp time.Time
	var expectedTimestamp time.Time
	var timestamp time.Time

	b.log.Info("reliability: emitter started")

	for range time.NewTicker(emissionInterval).C {
		expectedTimestamp = lastTimestamp.Add(emissionInterval)
		timestamp = time.Now().Truncate(emissionInterval)

		if !lastTimestamp.IsZero() && expectedTimestamp != timestamp {
			b.log.Info("reliability: WARNING: an expected emission was missed", logger.String("missed", expectedTimestamp.String()), logger.String("sent", timestamp.String()))
		}

		b.emitReliabilityMetrics(sourceId, ingressClient, timestamp)
		lastTimestamp = timestamp

		select {
		case <-stopChan:
			return
		default:
		}
	}
}

func (b *Blackbox) emitReliabilityMetrics(sourceId string, client *ingressclient.IngressClient, timestamp time.Time) {
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
		b.log.Error("reliability: failed to marshal test metric points", err)
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

	if err == nil {
		b.log.Info("reliability: interval metrics emitted")
	} else {
		b.log.Error("reliability: failed to write test metric envelope", err)
	}
}
