package blackbox

import (
	"context"
	"fmt"
	"time"

	"github.com/prometheus/common/model"
)

type PerformanceCalculator struct {
	sourceID string
}

type PerformanceMetrics struct {
	Latency   time.Duration
	Magnitude int
}

func NewPerformanceCalculator(sourceID string) *PerformanceCalculator {
	return &PerformanceCalculator{
		sourceID: sourceID,
	}
}

func (c *PerformanceCalculator) Calculate(client QueryableClient) (PerformanceMetrics, error) {
	ctx, _ := context.WithTimeout(context.Background(), 60*time.Second)

	query := fmt.Sprintf(`sum(count_over_time(%s{source_id="%s"}[1w]))`, BlackboxPerformanceTestCanary, c.sourceID)

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
