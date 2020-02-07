package storage_test

import (
	"errors"
	"github.com/cloudfoundry/metric-store-release/src/internal/storage"
	"github.com/cloudfoundry/metric-store-release/src/internal/testing"
	"github.com/cloudfoundry/metric-store-release/src/pkg/logger"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/prometheus/prometheus/pkg/labels"
	prom_storage "github.com/prometheus/prometheus/storage"
	"net"
)

var _ = Describe("Querier", func() {
	// Many of the tests for querier are in store_test.go
	Describe("Select()", func() {
		DescribeTable(
			"returns an error if given a query that uses a matcher other than = on __name__",
			func(in []*labels.Matcher, out error) {

				querier := storage.NewReplicatedQuerier(nil, 0, nil, nil,
					logger.NewTestLogger(GinkgoWriter))
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
			var lookup = func(_ string) []int { return []int{1, 2, 3} }
			return storage.NewReplicatedQuerier(testing.NewSpyStorage(localQuerier), 0,
				[]prom_storage.Querier{localQuerier, remoteQuerier, remoteQuerier, remoteQuerier}, lookup, logger.NewTestLogger(GinkgoWriter))
		}

		Context("happy path", func() {
			It("doesn't nil-ref on duplicate node addresses", func() {
				subject := createTestSubject(nil,nil)
				Expect(func() { subject.Select(nil) }).NotTo(Panic())
			})

			It("calls remote node", func() {
				spy := newSpyQuerier()
				subject := createTestSubject(nil,spy)
				var err error
				_, _, err = subject.Select(nil)
				Expect(err).NotTo(HaveOccurred())
				Expect(spy.callCount).To(Equal(1))
			})

			It("prefers local node", func() {
				localQuerier := newSpyQuerier()
				remoteQuerier := newSpyQuerier()

				lookup := func(_ string) []int { return []int{0, 1} }

				subject := storage.NewReplicatedQuerier(testing.NewSpyStorage(localQuerier), 0,
					[]prom_storage.Querier{localQuerier, remoteQuerier}, lookup, logger.NewTestLogger(GinkgoWriter))

				var attempts int
				Consistently(func() bool {
					attempts++
					var err error
					_, _, err = subject.Select(nil)
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
				_, _, err = subject.Select(nil)
				Expect(err).To(HaveOccurred())
				Expect(spy.callCount).To(Equal(1))
			})

			It("fails over calls on connection error", func() {
				spy := newSpyQuerierWithRepeatedErrors(&net.OpError{}, 3)
				subject := createTestSubject(nil, spy)
				var err error
				_, _, err = subject.Select(nil)
				Expect(err).NotTo(HaveOccurred())
				Expect(spy.callCount).To(BeNumerically(">=", 3))
			})

			It("stops failing over once a result is returned", func() {
				spy := newSpyQuerierWithRepeatedErrors(&net.OpError{}, 1)
				subject := createTestSubject(nil, spy)
				var err error
				_, _, err = subject.Select(nil)
				Expect(err).NotTo(HaveOccurred())
				Expect(spy.callCount).To(Equal(2))
			})

			Context("connection retries", func() {
				It("retries all nodes when their servers are unavailable", func() {
					spy := newSpyQuerierWithRepeatedErrors(&net.OpError{}, 7)
					localQuerier := newSpyQuerier()
					subject := createTestSubject(localQuerier, spy)
					var err error
					_, _, err = subject.Select(nil)
					Expect(err).NotTo(HaveOccurred())
					Expect(spy.callCount).To(Equal(8))
					Expect(localQuerier.callCount).To(Equal(0))
				})
			})
		})
	})
})

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

func (*spyQuerier) LabelValues(name string) ([]string, prom_storage.Warnings, error) {
	panic("implement me")
}

func (*spyQuerier) LabelNames() ([]string, prom_storage.Warnings, error) {
	panic("implement me")
}

func (*spyQuerier) Close() error {
	panic("implement me")
}
