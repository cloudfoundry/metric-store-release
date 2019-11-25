package performance_test

import (
	"context"
	"path"
	"runtime"
	"time"

	"github.com/cloudfoundry/metric-store-release/src/pkg/persistence"
	"github.com/influxdata/influxql"
	"github.com/prometheus/prometheus/pkg/labels"
	"github.com/prometheus/prometheus/storage"

	shared "github.com/cloudfoundry/metric-store-release/src/internal/testing"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const (
	storagePathPrefix     = "metric-store"
	minTimeInMilliseconds = influxql.MinTime / int64(time.Millisecond)
	maxTimeInMilliseconds = influxql.MaxTime / int64(time.Millisecond)
)

var _ = Describe("Performance", func() {
	Measure("runs faster", func(b Benchmarker) {
		_, filename, _, _ := runtime.Caller(0)
		storagePath := path.Join(path.Dir(filename), "./data")

		spyPersistentStoreMetrics := shared.NewSpyMetricRegistrar()
		persistentStore := persistence.NewStore(
			storagePath,
			spyPersistentStoreMetrics,
		)

		var seriesSet storage.SeriesSet
		var err error

		query := b.Time("query", func() {
			querier, _ := persistentStore.Querier(context.Background(), 0, 0)
			seriesSet, _, err = querier.Select(
				&storage.SelectParams{Start: minTimeInMilliseconds, End: maxTimeInMilliseconds},
				&labels.Matcher{Name: "__name__", Value: "bigmetric", Type: labels.MatchEqual},
			)
		})
		Expect(err).ToNot(HaveOccurred())

		var totalPoints, totalSeries int
		series := shared.ExplodeSeriesSet(seriesSet)
		for _, s := range series {
			totalSeries++
			totalPoints += len(s.Points)
		}
		Expect(totalSeries).To(BeNumerically(">", 100))
		Expect(totalPoints).To(Equal(1_000_000))
		Expect(query.Seconds()).To(BeNumerically("<", 10))
	}, 3)
})
