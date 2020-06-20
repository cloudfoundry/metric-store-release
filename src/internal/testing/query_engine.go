package testing

import (
	"time"

	"github.com/cloudfoundry/metric-store-release/src/pkg/logger"
	"github.com/onsi/ginkgo"
	"github.com/prometheus/prometheus/promql"
)

func NewQueryEngine() *promql.Engine {
	engineOpts := promql.EngineOpts{
		MaxSamples:    20e6,
		Timeout:       time.Minute,
		Logger:        logger.NewTestLogger(ginkgo.GinkgoWriter),
		LookbackDelta: 5 * time.Minute,
	}
	return promql.NewEngine(engineOpts)
}
