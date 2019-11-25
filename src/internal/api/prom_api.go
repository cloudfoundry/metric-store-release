package api

import (
	"net/http"
	"regexp"

	"github.com/cloudfoundry/metric-store-release/src/internal/logger"
	"github.com/prometheus/common/route"
	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/notifier"
	"github.com/prometheus/prometheus/promql"
	"github.com/prometheus/prometheus/rules"
	"github.com/prometheus/prometheus/storage"
	prom_api "github.com/prometheus/prometheus/web/api/v1"
)

const (
	REMOTE_READ_SAMPLE_LIMIT       = 1000000
	REMOTE_READ_CONCURRENCY_LIMIT  = 50
	REMOTE_READ_MAX_BYTES_IN_FRAME = 1048576 // 1MB recommended by protobuf
)

type PromAPI struct {
	promQLEngine *promql.Engine
	notifier     *notifier.Manager
	ruleManager  *rules.Manager
	log          *logger.Logger
}

func NewPromAPI(promQLEngine *promql.Engine, notifier *notifier.Manager, ruleManager *rules.Manager, log *logger.Logger) *PromAPI {
	return &PromAPI{
		promQLEngine: promQLEngine,
		notifier:     notifier,
		ruleManager:  ruleManager,
		log:          log,
	}
}

func (api *PromAPI) RouterForStorage(storage storage.Storage) *route.Router {
	promAPI := prom_api.NewAPI(
		api.promQLEngine,
		storage,
		&nullTargetRetriever{},
		api.notifier,
		func() config.Config { return config.Config{} },
		nil,
		func(h http.HandlerFunc) http.HandlerFunc { return h },
		func() prom_api.TSDBAdmin { return &nullTSDBAdmin{} },
		false,
		api.log,
		api.ruleManager,
		REMOTE_READ_SAMPLE_LIMIT,
		REMOTE_READ_CONCURRENCY_LIMIT,
		REMOTE_READ_MAX_BYTES_IN_FRAME,
		&regexp.Regexp{},
	)

	promAPIRouter := route.New()
	promAPI.Register(promAPIRouter)

	return promAPIRouter
}
