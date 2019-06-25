package query

import (
	"github.com/prometheus/prometheus/rules"
	"github.com/prometheus/prometheus/storage"
)

func EngineQueryFunc(engine *Engine, dataReader storage.Querier) rules.QueryFunc {
	queryable, promEngine := engine.createPromQLEngine(dataReader)
	return rules.EngineQueryFunc(promEngine, queryable)
}
