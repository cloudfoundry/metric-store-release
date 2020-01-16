package api

import (
	"net/http"
	"regexp"
	"time"

	"github.com/cloudfoundry/metric-store-release/src/internal/logger"
	"github.com/cloudfoundry/metric-store-release/src/internal/rules"
	"github.com/prometheus/common/route"
	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/promql"
	"github.com/prometheus/prometheus/storage"
	prom_api "github.com/prometheus/prometheus/web/api/v1"
)

const (
	REMOTE_READ_SAMPLE_LIMIT       = 20000000
	REMOTE_READ_CONCURRENCY_LIMIT  = 50
	REMOTE_READ_MAX_BYTES_IN_FRAME = 1048576 // 1MB recommended by protobuf
)

type PromAPI struct {
	promQLEngine *promql.Engine
	log          *logger.Logger
}

func NewPromAPI(promQLEngine *promql.Engine, log *logger.Logger) *PromAPI {
	return &PromAPI{
		promQLEngine: promQLEngine,
		log:          log,
	}
}

func (api *PromAPI) RouterForStorage(storage storage.Storage, ruleManager rules.RuleManager) *route.Router {
	prometheusVersion := &prom_api.PrometheusVersion{
		Version:   "2.14.0",
		Revision:  "edeb7a44cbf745f1d8be4ea6f215e79e651bfe19",
		Branch:    "v2.14.0",
		BuildUser: "",
		BuildDate: "",
		GoVersion: "",
	}

	runtimeInfo := prom_api.RuntimeInfo{
		StartTime:           time.Now(),
		CWD:                 "",
		ReloadConfigSuccess: false,
		LastConfigTime:      time.Now(),
		ChunkCount:          0,
		TimeSeriesCount:     0,
		CorruptionCount:     0,
		GoroutineCount:      0,
		GOMAXPROCS:          0,
		GOGC:                "",
		GODEBUG:             "",
		StorageRetention:    "",
	}

	promAPI := prom_api.NewAPI(
		api.promQLEngine,
		storage,
		&nullTargetRetriever{},
		ruleManager,
		func() config.Config { return config.Config{} },
		nil,
		func(h http.HandlerFunc) http.HandlerFunc { return h },
		func() prom_api.TSDBAdmin { return &nullTSDBAdmin{} },
		false,
		api.log,
		ruleManager,
		REMOTE_READ_SAMPLE_LIMIT,
		REMOTE_READ_CONCURRENCY_LIMIT,
		REMOTE_READ_MAX_BYTES_IN_FRAME,
		&regexp.Regexp{},
		func() (prom_api.RuntimeInfo, error) { return runtimeInfo, nil },
		prometheusVersion,
	)

	promAPIRouter := route.New()
	promAPI.Register(promAPIRouter)

	return promAPIRouter
}
