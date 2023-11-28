package api

import (
	"context"
	"github.com/prometheus/client_golang/prometheus"
	"net/http"
	"time"

	"github.com/cloudfoundry/metric-store-release/src/internal/rules"
	"github.com/cloudfoundry/metric-store-release/src/pkg/logger"
	"github.com/grafana/regexp"
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

func (api *PromAPI) RouterForStorage(storage storage.Storage, ruleManager rules.RuleManager, gatherer prometheus.Gatherer, registerer prometheus.Registerer) *route.Router {
	targetRetriever := func(context.Context) prom_api.TargetRetriever {
		return &nullTargetRetriever{}
	}

	alertmanagerRetriever := func(context.Context) prom_api.AlertmanagerRetriever {
		return ruleManager
	}

	rulesRetriever := func(context.Context) prom_api.RulesRetriever {
		return ruleManager
	}

	nullDBDir := ""

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
		storage,
		nil,
		nil,
		targetRetriever,
		alertmanagerRetriever,
		func() config.Config { return config.Config{} },
		nil,
		prom_api.GlobalURLOptions{},
		func(h http.HandlerFunc) http.HandlerFunc { return h },
		&nullTSDBAdminStats{},
		nullDBDir,
		false,
		api.log,
		rulesRetriever,
		REMOTE_READ_SAMPLE_LIMIT,
		REMOTE_READ_CONCURRENCY_LIMIT,
		REMOTE_READ_MAX_BYTES_IN_FRAME,
		false,
		&regexp.Regexp{},
		func() (prom_api.RuntimeInfo, error) { return runtimeInfo, nil },
		prometheusVersion,
		gatherer,
		registerer,
		nil,
		false,
		false,
	)

	promAPIRouter := route.New()
	promAPI.Register(promAPIRouter)

	return promAPIRouter
}
