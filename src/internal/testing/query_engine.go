package testing

import (
	"time"

	"github.com/cloudfoundry/metric-store-release/src/internal/logger"
	"github.com/onsi/ginkgo"
	"github.com/prometheus/prometheus/promql"
)

func NewQueryEngine() *promql.Engine {
	engineOpts := promql.EngineOpts{
		MaxConcurrent: 10,
		MaxSamples:    20e6,
		Timeout:       time.Minute,
		Logger:        logger.NewTestLogger(ginkgo.GinkgoWriter),
	}
	return promql.NewEngine(engineOpts)
}
