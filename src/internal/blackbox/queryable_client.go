package blackbox

import (
	"context"
	"time"

	prom_versioned_api_client "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

type QueryableClient interface {
	Query(ctx context.Context, query string, ts time.Time, opts ...prom_versioned_api_client.
		Option) (model.Value, prom_versioned_api_client.Warnings, error)
	LabelValues(ctx context.Context, label string, matches []string, startTime,
		endTime time.Time) (model.LabelValues, prom_versioned_api_client.Warnings, error)
}
