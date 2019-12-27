package blackbox

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/api"
	prom_api_client "github.com/prometheus/client_golang/api"
	"github.com/prometheus/common/model"
)

type QueryableClient interface {
	Query(ctx context.Context, query string, ts time.Time) (model.Value, prom_api_client.Warnings, error)
	LabelValues(context.Context, string) (model.LabelValues, api.Warnings, error)
}
