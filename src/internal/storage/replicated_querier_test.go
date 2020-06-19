package storage_test

import (
	"context"
	"errors"
	"net"
	"time"

	"github.com/cloudfoundry/metric-store-release/src/internal/metric-store"
	"github.com/cloudfoundry/metric-store-release/src/internal/storage"
	"github.com/cloudfoundry/metric-store-release/src/internal/testing"
	"github.com/cloudfoundry/metric-store-release/src/pkg/logger"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	config_util "github.com/prometheus/common/config"
	"github.com/prometheus/prometheus/pkg/labels"
	prom_storage "github.com/prometheus/prometheus/storage"
)

var _ = Describe("Querier", func() {
	var simpleQuery *labels.Matcher

	BeforeEach(func() {
		var err error
		simpleQuery, err = labels.NewMatcher(labels.MatchEqual, labels.MetricName, "cpu")
		Expect(err).NotTo(HaveOccurred())

	})
	// Many of the tests for querier are in store_test.go
	Describe("Select()", func() {
		DescribeTable(
			"returns an error if given a query that uses a matcher other than = on __name__",
			func(in []*labels.Matcher, out error) {
				factory := &testFactory{}
				querier := storage.NewReplicatedQuerier(context.TODO(), nil, 0, factory, 5*time.Second,
					nil, logger.NewTestLogger(GinkgoWriter))
				_, _, err := querier.Select(nil, in...)
				Expect(err).To(Equal(out))
			},
			Entry("!= on __name__", []*labels.Matcher{{
				Name:  "__name__",
				Type:  labels.MatchNotEqual,
				Value: "irrelevantapp",
			}}, errors.New("only strict equality is supported for metric names")),
			Entry("=~ on __name__", []*labels.Matcher{{
				Name:  "__name__",
				Type:  labels.MatchRegexp,
				Value: "irrelevantapp",
			}}, errors.New("only strict equality is supported for metric names")),
			Entry("!~ on __name__", []*labels.Matcher{{
				Name:  "__name__",
				Type:  labels.MatchNotRegexp,
				Value: "irrelevantapp",
			}}, errors.New("only strict equality is supported for metric names")),
		)

	})

	Context("Select", func() {
		var createTestSubject = func(localQuerier, remoteQuerier prom_storage.Querier) *storage.ReplicatedQuerier {
			router := &mockRouting{lookupNodes: []int{1, 2, 3}}

			factory := &testFactory{
				queriers: []prom_storage.Querier{remoteQuerier, remoteQuerier, remoteQuerier},
			}
			return storage.NewReplicatedQuerier(context.TODO(), testing.NewSpyStorage(localQuerier), 0,
				factory, 5*time.Second, router, logger.NewTestLogger(GinkgoWriter))
		}

		Context("happy path", func() {
			It("doesn't nil-ref on duplicate node addresses", func() {
				subject := createTestSubject(nil, nil)
				Expect(func() { _, _, _ = subject.Select(nil) }).NotTo(Panic())
			})

			It("calls remote node", func() {
				spy := newSpyQuerier()
				subject := createTestSubject(nil, spy)

				var err error
				_, _, err = subject.Select(nil, simpleQuery)
				Expect(err).NotTo(HaveOccurred())
				Expect(spy.callCount).To(Equal(1))
			})

			It("prefers local node", func() {
				localQuerier := newSpyQuerier()
				remoteQuerier := newSpyQuerier()

				router := &mockRouting{lookupNodes: []int{0, 1}}

				subject := storage.NewReplicatedQuerier(context.TODO(), testing.NewSpyStorage(localQuerier), 0,
					&testFactory{queriers: []prom_storage.Querier{localQuerier, remoteQuerier}},
					5*time.Second, router, logger.NewTestLogger(GinkgoWriter))

				var attempts int
				Consistently(func() bool {
					attempts++
					var err error
					_, _, err = subject.Select(nil, simpleQuery)
					Expect(err).NotTo(HaveOccurred())
					Expect(remoteQuerier.callCount).To(Equal(0))
					return localQuerier.callCount == attempts
				}).Should(BeTrue())
			})
		})

		Context("node failover", func() {
			It("does not fail over calls on non-connection error", func() {
				spy := newSpyQuerierWithRepeatedErrors(errors.New("expected"), 3)
				subject := createTestSubject(nil, spy)
				var err error
				_, _, err = subject.Select(nil, simpleQuery)
				Expect(err).To(HaveOccurred())
				Expect(spy.callCount).To(Equal(1))
			})

			It("fails over calls on connection error", func() {
				spy := newSpyQuerierWithRepeatedErrors(&net.OpError{}, 3)
				subject := createTestSubject(nil, spy)
				var err error
				_, _, err = subject.Select(nil, simpleQuery)
				Expect(err).NotTo(HaveOccurred())
				Expect(spy.callCount).To(BeNumerically(">=", 3))
			})

			It("stops failing over once a result is returned", func() {
				spy := newSpyQuerierWithRepeatedErrors(&net.OpError{}, 1)
				subject := createTestSubject(nil, spy)
				var err error
				_, _, err = subject.Select(nil, simpleQuery)
				Expect(err).NotTo(HaveOccurred())
				Expect(spy.callCount).To(Equal(2))
			})

			Context("connection retries", func() {
				It("retries all nodes when their servers are unavailable", func() {
					spy := newSpyQuerierWithRepeatedErrors(&net.OpError{}, 7)
					localQuerier := newSpyQuerier()
					subject := createTestSubject(localQuerier, spy)
					var err error
					_, _, err = subject.Select(nil, simpleQuery)
					Expect(err).NotTo(HaveOccurred())
					Expect(spy.callCount).To(Equal(8))
					Expect(localQuerier.callCount).To(Equal(0))
				})
			})
		})

		Context("ReplicatedQuerierFactory", func() {
			defaultQuerierConfig := &config_util.TLSConfig{
				CAFile:     testing.Cert("metric-store-ca.crt"),
				CertFile:   testing.Cert("metric-store.crt"),
				KeyFile:    testing.Cert("metric-store.key"),
				ServerName: metric_store.COMMON_NAME,
			}
			It("returns all nodes when no indexes are specified", func() {
				expected := []string{"localhost:1234", "localhost:2345", "localhost:3456"}
				subject := storage.NewReplicatedQuerierFactory(
					testing.NewSpyStorage(newSpyQuerier()), 0, expected,
					defaultQuerierConfig, logger.NewTestLogger(GinkgoWriter))
				Expect(subject.Build(context.Background())).To(HaveLen(3))
			})
			It("filters nodes", func() {
				expected := []string{"localhost:1234", "localhost:2345", "localhost:3456"}
				subject := storage.NewReplicatedQuerierFactory(testing.NewSpyStorage(newSpyQuerier()), 0,
					expected, defaultQuerierConfig, logger.NewTestLogger(GinkgoWriter))
				Expect(subject.Build(context.Background(), 1, 2)).To(HaveLen(2))
			})
			It("filters nodes for which queriers cannot be created", func() {
				expected := []string{"localhost:1234", "badurl", "localhost:2345"}
				subject := storage.NewReplicatedQuerierFactory(testing.NewSpyStorage(newSpyQuerier()), 0,
					expected, defaultQuerierConfig, logger.NewTestLogger(GinkgoWriter))
				Expect(subject.Build(context.Background())).To(HaveLen(2))
			})
			It("fetches a local querier from the store", func() {
				expected := []string{"localhost:1234", "localhost:2345"}
				spy := newSpyQuerier()
				subject := storage.NewReplicatedQuerierFactory(testing.NewSpyStorage(spy), 0, expected,
					defaultQuerierConfig, logger.NewTestLogger(GinkgoWriter))
				Expect(subject.Build(context.Background())).To(And(
					HaveLen(2),
					ContainElement(spy),
				))
			})
			It("fetches a local querier from the store", func() {
				expected := []string{"localhost:1234", "localhost:2345"}
				subject := storage.NewReplicatedQuerierFactory(testing.NewSpyStorage(nil), 0, expected,
					defaultQuerierConfig, logger.NewTestLogger(GinkgoWriter))
				Expect(subject.Build(context.Background())).To(And(
					HaveLen(1),
					Not(ContainElement(BeNil())),
				))
			})
		})
	})
})

type mockRouting struct {
	lookupNodes []int
}

func (r *mockRouting) IsLocal(metricName string) bool {
	return false
}

func (r *mockRouting) Lookup(item string) []int {
	return r.lookupNodes
}

type testFactory struct {
	queriers []prom_storage.Querier
}

func (t *testFactory) Build(ctx context.Context, nodeIndexes ...int) []prom_storage.Querier {
	return t.queriers
}

type spyQuerier struct {
	callCount    int
	selectErrors chan error
}

func newSpyQuerier() *spyQuerier {
	return &spyQuerier{
		selectErrors: make(chan error, 10),
	}
}

func newSpyQuerierWithRepeatedErrors(err error, count int) *spyQuerier {
	spy := newSpyQuerier()
	for i := 0; i < count; i++ {
		spy.selectErrors <- err
	}
	return spy
}

func (q *spyQuerier) Select(*prom_storage.SelectParams, ...*labels.Matcher) (prom_storage.SeriesSet, prom_storage.Warnings, error) {
	q.callCount++
	var err error
	select {
	case err = <-q.selectErrors:
	default:
	}
	return nil, nil, err
}

func (q *spyQuerier) SelectSorted(params *prom_storage.SelectParams, labelMatchers ...*labels.Matcher) (prom_storage.SeriesSet, prom_storage.Warnings, error) {
	return q.Select(params, labelMatchers...)
}

func (*spyQuerier) LabelValues(_ string) ([]string, prom_storage.Warnings, error) {
	panic("implement me")
}

func (*spyQuerier) LabelNames() ([]string, prom_storage.Warnings, error) {
	panic("implement me")
}

func (*spyQuerier) Close() error {
	panic("implement me")
}
