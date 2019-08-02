package api

import (
	"net/http"
	"os"
	"regexp"

	prom_log "github.com/go-kit/kit/log"
	"github.com/prometheus/common/route"
	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/promql"
	"github.com/prometheus/prometheus/storage"
	prom_api "github.com/prometheus/prometheus/web/api/v1"
)

const (
	REMOTE_READ_SAMPLE_LIMIT      = 1000000
	REMOTE_READ_CONCURRENCY_LIMIT = 50
)

type PromAPI struct {
	promQLEngine *promql.Engine
}

func NewPromAPI(promQLEngine *promql.Engine) *PromAPI {
	return &PromAPI{
		promQLEngine: promQLEngine,
	}
}

func (api *PromAPI) RouterForStorage(storage storage.Storage) *route.Router {
	promAPI := prom_api.NewAPI(
		api.promQLEngine,
		storage,
		&nullTargetRetriever{},
		&nullAlertmanagerRetriever{},
		func() config.Config { return config.Config{} },
		nil,
		func(h http.HandlerFunc) http.HandlerFunc { return h },
		func() prom_api.TSDBAdmin { return &nullTSDBAdmin{} },
		false,
		prom_log.NewLogfmtLogger(prom_log.NewSyncWriter(os.Stderr)),
		&nullRulesRetriever{},
		REMOTE_READ_SAMPLE_LIMIT,
		REMOTE_READ_CONCURRENCY_LIMIT,
		&regexp.Regexp{},
	)

	promAPIRouter := route.New()
	promAPI.Register(promAPIRouter)

	return promAPIRouter
}
