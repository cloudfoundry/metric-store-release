package rules_test

import (
	"context"
	"io/ioutil"
	"os"
	"time"

	"github.com/cloudfoundry/metric-store-release/src/internal/logger"
	. "github.com/cloudfoundry/metric-store-release/src/internal/rules"
	"github.com/cloudfoundry/metric-store-release/src/internal/testing"
	shared "github.com/cloudfoundry/metric-store-release/src/internal/testing"
	"github.com/cloudfoundry/metric-store-release/src/pkg/persistence"
	"github.com/influxdata/influxql"
	"github.com/prometheus/prometheus/pkg/labels"
	"github.com/prometheus/prometheus/promql"
	"github.com/prometheus/prometheus/storage"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const (
	storagePathPrefix     = "metric-store"
	minTimeInMilliseconds = influxql.MinTime / int64(time.Millisecond)
	maxTimeInMilliseconds = influxql.MaxTime / int64(time.Millisecond)
)

var _ = Describe("Prom Manager", func() {
	Describe("Start()", func() {
		It("Records recording rule metrics", func() {
			deps, teardown := setupDependencies(`
groups:
- name: foo-group
  rules:
  - record: testRecordingRule
    expr: avg(metric_store_test_metric)
`)
			defer teardown()

			promManager := NewPromRuleManager("manager", deps.ruleFile.Name(), "",
				time.Second, deps.store, deps.queryEngine, logger.NewTestLogger(GinkgoWriter), deps.spyMetrics)
			err := promManager.Start()
			Expect(err).ToNot(HaveOccurred())

			querier, err := deps.store.Querier(context.Background(), 0, influxql.MaxTime)
			Expect(err).ToNot(HaveOccurred())

			Eventually(func() int {
				points := queryByName(querier, "testRecordingRule")
				if len(points) == 0 {
					return 0
				}

				return int(points[0].Value)
			}).Should(Equal(3))
		})

		It("Sends an alert when an alertmanager configured", func() {
			alertSpy := testing.NewAlertManagerSpy()
			alertSpy.Start()
			defer alertSpy.Stop()

			deps, teardown := setupDependencies(`
groups:
- name: foo-group
  rules:
  - alert: testAlertingRule
    expr: metric_store_test_metric > 2
`)
			defer teardown()

			promManager := NewPromRuleManager("manager", deps.ruleFile.Name(), alertSpy.Addr(),
				time.Second, deps.store, deps.queryEngine, logger.NewTestLogger(GinkgoWriter), deps.spyMetrics)
			err := promManager.Start()
			Expect(err).ToNot(HaveOccurred())

			Eventually(alertSpy.AlertsReceived, 10).Should(BeNumerically(">", 0))
		})
	})

	Describe("Reload()", func() {
		It("Loads recording rules", func() {
			tmpfile := CreateTempFile(`
groups:
- name: foo-group
  rules:
  - record: testRecordingRule
    expr: avg(metric_store_test_metric) by (node)
`)
			defer os.Remove(tmpfile.Name())

			promManager := NewPromRuleManager("manager", tmpfile.Name(), "localhost", time.Second, nil, nil, logger.NewTestLogger(GinkgoWriter), shared.NewSpyMetricRegistrar())

			err := promManager.Reload()
			Expect(err).ToNot(HaveOccurred())

			ruleGroup := promManager.RuleGroups()[0]
			Expect(ruleGroup.Name()).To(Equal("foo-group"))

			rule := ruleGroup.Rules()[0]
			Expect(rule.Name()).To(Equal("testRecordingRule"))
		})

		It("Loads alerting rules", func() {
			tmpfile := CreateTempFile(`
groups:
- name: foo-group
  rules:
  - alert: testAlertingRule
    expr: metric_store_test_metric > 2
`)
			defer os.Remove(tmpfile.Name())

			promManager := NewPromRuleManager("manager", tmpfile.Name(), "localhost", time.Second, nil, nil, logger.NewTestLogger(GinkgoWriter), shared.NewSpyMetricRegistrar())

			err := promManager.Reload()
			Expect(err).ToNot(HaveOccurred())

			ruleGroup := promManager.RuleGroups()[0]
			Expect(ruleGroup.Name()).To(Equal("foo-group"))

			rule := ruleGroup.Rules()[0]
			Expect(rule.Name()).To(Equal("testAlertingRule"))
		})

		It("Returns an error when the file doesn't exist", func() {
			promManager := NewPromRuleManager("manager", "badfile.yml", "localhost", time.Second, nil, nil, logger.NewTestLogger(GinkgoWriter), shared.NewSpyMetricRegistrar())

			err := promManager.Reload()
			Expect(err).To(HaveOccurred())
		})

		It("Returns an error when the file format isn't valid", func() {
			tmpfile := CreateTempFile(`
not
valid
yaml
`)
			defer os.Remove(tmpfile.Name())

			promManager := NewPromRuleManager("manager", tmpfile.Name(), "localhost", time.Second, nil, nil, logger.NewTestLogger(GinkgoWriter), shared.NewSpyMetricRegistrar())

			err := promManager.Reload()
			Expect(err).To(HaveOccurred())
		})
	})
})

type ruleManagerDependencies struct {
	ruleFile    *os.File
	spyMetrics  *shared.SpyMetricRegistrar
	store       *persistence.Store
	queryEngine *promql.Engine
}

func setupDependencies(rules string) (*ruleManagerDependencies, func()) {
	storagePath, err := ioutil.TempDir("", storagePathPrefix)
	if err != nil {
		panic(err)
	}

	spyMetrics := shared.NewSpyMetricRegistrar()

	persistentStore := persistence.NewStore(
		storagePath,
		spyMetrics,
	)
	loadMetric(persistentStore)

	queryEngine := promql.NewEngine(promql.EngineOpts{
		MaxConcurrent: 10,
		MaxSamples:    1e6,
		Timeout:       time.Second,
		Logger:        logger.NewTestLogger(GinkgoWriter),
		Reg:           spyMetrics.Registerer(),
	})

	deps := &ruleManagerDependencies{
		ruleFile:    CreateTempFile(rules),
		spyMetrics:  spyMetrics,
		store:       persistentStore,
		queryEngine: queryEngine,
	}

	return deps, func() {
		persistentStore.Close()
		os.Remove(deps.ruleFile.Name())
		os.RemoveAll(storagePath)
	}
}

// TODO minimize scope -> use TestContext
func loadMetric(store *persistence.Store) {
	appender, err := store.Appender()
	Expect(err).ToNot(HaveOccurred())
	appender.Add(
		labels.FromMap(map[string]string{"__name__": "metric_store_test_metric"}),
		time.Now().UnixNano(),
		3,
	)
	appender.Commit()

	querier, err := store.Querier(context.Background(), 0, influxql.MaxTime)
	Expect(err).ToNot(HaveOccurred())

	Eventually(func() bool {
		return len(queryByName(querier, "metric_store_test_metric")) > 0
	}).Should(BeTrue())
}

func queryByName(querier storage.Querier, name string) []testing.Point {
	seriesSet, _, err := querier.Select(
		&storage.SelectParams{Start: minTimeInMilliseconds, End: maxTimeInMilliseconds},
		&labels.Matcher{Name: "__name__", Value: name, Type: labels.MatchEqual},
	)
	if err != nil {
		return []testing.Point{}
	}

	series := shared.ExplodeSeriesSet(seriesSet)
	if len(series) == 0 {
		return []testing.Point{}
	}

	return series[0].Points
}

func CreateTempFile(content string) *os.File {
	tmpfile, err := ioutil.TempFile("", "rules_yml")
	Expect(err).NotTo(HaveOccurred())
	if _, err := tmpfile.Write([]byte(content)); err != nil {
		panic(err)
	}
	if err := tmpfile.Close(); err != nil {
		panic(err)
	}
	return tmpfile
}
