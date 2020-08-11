package blackbox

import (
	"context"
	"fmt"
	"time"

	"github.com/cloudfoundry/metric-store-release/src/internal/metrics"
	"github.com/cloudfoundry/metric-store-release/src/pkg/logger"
	"github.com/cloudfoundry/metric-store-release/src/pkg/ingressclient"
	"github.com/cloudfoundry/metric-store-release/src/pkg/rpc"
	"github.com/prometheus/common/model"
)

type PerformanceCalculator struct {
	cfg            *Config
	log            *logger.Logger
	debugRegistrar metrics.Registrar
}

type PerformanceMetrics struct {
	Latency   time.Duration
	Magnitude int
}

var labels = map[string][]string{
	"app_id":              {"bde5831e-a819-4a34-9a46-012fd2e821e6b"},
	"app_name":            {"bblog"},
	"bosh_environment":    {"vpc-bosh-run-pivotal-io"},
	"deployment":          {"pws-diego-cellblock-09"},
	"index":               {"9b74a5b1-9af9-4715-a57a-bd28ad7e7f1b"},
	"instance_id":         {"0", "1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11", "12", "13", "14", "15"},
	"ip":                  {"10.10.148.146"},
	"job":                 {"diego-cell"},
	"organization_id":     {"ab2de77b-a484-4690-9201-b8eaf707fd87"},
	"organization_name":   {"blars"},
	"origin":              {"rep"},
	"process_id":          {"328de02b-79f1-4f8d-b3b2-b81112809603"},
	"process_instance_id": {"2652349b-4d40-4b51-4165-7129"},
	"process_type":        {"web"},
	"source_id":           {"5ee5831e-a819-4a34-9a46-012fd2e821e7"},
	"space_id":            {"eb94778d-66b5-4804-abcb-e9efd7f725aa"},
	"space_name":          {"bblog"},
	"unit":                {"percentage"},
}

func NewPerformanceCalculator(cfg *Config, log *logger.Logger, debugRegistrar metrics.Registrar) *PerformanceCalculator {
	return &PerformanceCalculator{
		cfg:            cfg,
		log:            log,
		debugRegistrar: debugRegistrar,
	}
}

func (c *PerformanceCalculator) Calculate(client QueryableClient) (PerformanceMetrics, error) {
	ctx, _ := context.WithTimeout(context.Background(), 60*time.Second)

	query := fmt.Sprintf(`sum(count_over_time(%s{source_id="%s"}[1w]))`, BlackboxPerformanceTestCanary, c.cfg.SourceId)

	startTime := time.Now()
	pointCount, _, err := client.Query(ctx, query, time.Now())

	if err != nil {
		return PerformanceMetrics{}, err
	}

	return PerformanceMetrics{
		Latency:   time.Now().Sub(startTime),
		Magnitude: int(pointCount.(model.Vector)[0].Value),
	}, nil
}

func (pc *PerformanceCalculator) CalculatePerformance(egressClient QueryableClient, stopChan chan bool) {
	pc.log.Info("performance: starting calculator")
	t := time.NewTicker(pc.cfg.SampleInterval)

	for range t.C {
		perf, err := pc.Calculate(egressClient)
		if err != nil {
			pc.log.Error("performance: error calculating", err)
			continue
		}

		latency := perf.Latency.Seconds()
		quantity := float64(perf.Magnitude)

		pc.debugRegistrar.Set(BlackboxPerformanceLatency, latency)
		pc.debugRegistrar.Set(BlackboxPerformanceCount, quantity)
		pc.log.Info("performance: ", logger.Float64("duration", latency), logger.Float64("count", quantity))
	}
}

func (pc *PerformanceCalculator) EmitPerformanceTestMetrics(sourceId string, emissionInterval time.Duration, ingressClient *ingressclient.IngressClient, stopChan chan bool) {
	var lastTimestamp time.Time
	var expectedTimestamp time.Time
	var timestamp time.Time

	labelValueIndex := 0
	defer pc.log.Info("performance: halting canary emit")

	for range time.NewTicker(emissionInterval).C {
		expectedTimestamp = lastTimestamp.Add(emissionInterval)
		timestamp = time.Now().Truncate(emissionInterval)

		if !lastTimestamp.IsZero() && expectedTimestamp != timestamp {
			pc.log.Info("WARNING: an expected performance emission was missed", logger.String("missed", expectedTimestamp.String()), logger.String("sent", timestamp.String()))
		}

		emitLabels := map[string]string{}
		for key, allValues := range labels {
			emitLabels[key] = allValues[labelValueIndex%len(allValues)]
		}
		labelValueIndex++

		pc.emitPerformanceMetrics(sourceId, ingressClient, time.Now(), emitLabels)
		lastTimestamp = timestamp

		select {
		case <-stopChan:
			pc.log.Info("performance: emission stop signal received")
			return
		default:
		}
	}
}

func (pc *PerformanceCalculator) emitPerformanceMetrics(sourceId string, client *ingressclient.IngressClient, timestamp time.Time, labels map[string]string) {
	labels["source_id"] = sourceId

	points := []*rpc.Point{{
		Timestamp: timestamp.UnixNano(),
		Name:      BlackboxPerformanceTestCanary,
		Value:     10.0,
		Labels:    labels,
	}}

	err := client.Write(points)

	if err != nil {
		pc.log.Error("failed to write test metric envelope", err)
	}
}
