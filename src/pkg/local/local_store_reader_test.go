package local_test

import (
	"context"
	"errors"
	"time"

	"github.com/cloudfoundry/metric-store/src/pkg/local"
	"github.com/cloudfoundry/metric-store/src/pkg/persistence/transform"
	rpc "github.com/cloudfoundry/metric-store/src/pkg/rpc/metricstore_v1"
	"github.com/prometheus/prometheus/pkg/labels"
	"github.com/prometheus/prometheus/storage"

	"github.com/cloudfoundry/metric-store/src/pkg/testing"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("LocalStoreReader", func() {
	It("reads points from the store", func() {
		tc := setupLocalStoreReaderContext()

		builder := transform.NewSeriesBuilder()
		builder.AddPromQLSeries(&rpc.PromQL_Series{
			Metric: map[string]string{"__name__": "some-name"},
			Points: []*rpc.PromQL_Point{
				{Time: 99, Value: 99.0},
				{Time: 100, Value: 99.0},
			},
		})
		tc.spyStore.GetPoints = builder.SeriesSet()

		resp, err := tc.localStoreReader.Read(context.Background(),
			&storage.SelectParams{Start: 99, End: 100},
			&labels.Matcher{Type: labels.MatchEqual, Name: "__name__", Value: "some-name"},
		)
		Expect(err).ToNot(HaveOccurred())

		Expect(testing.ExplodeSeriesSet(resp)[0].Points).To(HaveLen(2))
		Expect(tc.spyStore.Name).To(Equal("some-name"))
		Expect(tc.spyStore.Start).To(Equal(int64(99)))
		Expect(tc.spyStore.End).To(Equal(int64(100)))
	})

	It("defaults Start to 0, End to now", func() {
		tc := setupLocalStoreReaderContext()

		_, err := tc.localStoreReader.Read(context.Background(),
			&storage.SelectParams{},
			&labels.Matcher{Type: labels.MatchEqual, Name: "__name__", Value: "some-name"},
		)
		Expect(err).ToNot(HaveOccurred())

		Expect(tc.spyStore.Name).To(Equal("some-name"))
		Expect(tc.spyStore.Start).To(Equal(int64(0)))
		Expect(tc.spyStore.End).To(BeNumerically("~", time.Now().UnixNano(), time.Second))
	})

	It("accepts query with a start time but without end time", func() {
		tc := setupLocalStoreReaderContext()

		_, err := tc.localStoreReader.Read(
			context.Background(),
			&storage.SelectParams{Start: 100},
			&labels.Matcher{Type: labels.MatchEqual, Name: "__name__", Value: "some-name"},
		)
		Expect(err).ToNot(HaveOccurred())
	})

	It("passes through errors from the store reader", func() {
		tc := setupLocalStoreReaderContext()
		tc.spyStore.GetErr = errors.New("some-error")

		_, err := tc.localStoreReader.Read(
			context.Background(),
			&storage.SelectParams{Start: 99, End: 100},
			&labels.Matcher{Type: labels.MatchEqual, Name: "__name__", Value: "some-name"},
		)
		Expect(err).To(HaveOccurred())
	})

	It("returns an error if the end time is before the start time", func() {
		tc := setupLocalStoreReaderContext()

		_, err := tc.localStoreReader.Read(
			context.Background(),
			&storage.SelectParams{Start: 100, End: 99},
			&labels.Matcher{Type: labels.MatchEqual, Name: "__name__", Value: "some-name"},
		)
		Expect(err).To(HaveOccurred())
	})
})

type localStoreReaderContext struct {
	spyStore         *testing.SpyStore
	localStoreReader *local.LocalStoreReader
}

func setupLocalStoreReaderContext() *localStoreReaderContext {
	spyStore := testing.NewSpyStoreReader()
	localStoreReader := local.NewLocalStoreReader(spyStore)

	return &localStoreReaderContext{
		spyStore:         spyStore,
		localStoreReader: localStoreReader,
	}
}
