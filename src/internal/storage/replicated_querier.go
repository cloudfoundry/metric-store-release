package storage

import (
	"context"
	"errors"
	"fmt"
	"github.com/cloudfoundry/metric-store-release/src/pkg/logger"
	"net"
	"sort"
	"strings"
	"time"

	"github.com/cloudfoundry/metric-store-release/src/internal/routing"
	"github.com/prometheus/prometheus/pkg/labels"
	prom_storage "github.com/prometheus/prometheus/storage"
)

type ReplicatedQuerier struct {
	ctx        context.Context
	store      prom_storage.Storage
	lookup     routing.Lookup
	localIndex int
	queriers   []prom_storage.Querier
	log        *logger.Logger
}

func NewReplicatedQuerier(
	ctx context.Context,
	localStore prom_storage.Storage,
	localIndex int,
	queriers []prom_storage.Querier,
	lookup routing.Lookup,
	log *logger.Logger,
) *ReplicatedQuerier {
	return &ReplicatedQuerier{
		ctx:        ctx,
		store:      localStore,
		localIndex: localIndex,
		queriers:   queriers,
		lookup:     lookup,
		log:        log,
	}
}

func (r *ReplicatedQuerier) Select(params *prom_storage.SelectParams, matchers ...*labels.Matcher) (prom_storage.SeriesSet, prom_storage.Warnings, error) {
	clients := routing.NewClients(r.lookup, r.localIndex)

	var metricName string
	for _, matcher := range matchers {
		if matcher.Name == labels.MetricName {
			if matcher.Type != labels.MatchEqual {
				return nil, nil, errors.New("only strict equality is supported for metric names")
			}
			metricName = matcher.Value
		}
	}
	// TODO: no metric name, return an error?
	nodesWithMetric, metricContainedLocally := clients.MetricDistribution(metricName)

	if metricContainedLocally {
		localQuerier, err := r.store.Querier(r.ctx, 0, 0)
		if err != nil {
			return nil, nil, err
		}
		return localQuerier.Select(params, matchers...)
	}

	return r.queryWithRetries(nodesWithMetric, params, matchers...)
}

func (r *ReplicatedQuerier) queryWithRetries(nodes []int, params *prom_storage.SelectParams, matchers ...*labels.Matcher) (prom_storage.SeriesSet, prom_storage.Warnings, error) {
	result, warnings, err := r.queryWithNodeFailover(nodes, params, matchers...)

	if isConnectionError(err) {
		return r.retryQueryWithBackoff(nodes, params, matchers...)
	} else {
		return result, warnings, err
	}
}

func (r *ReplicatedQuerier) retryQueryWithBackoff(nodes []int, params *prom_storage.SelectParams, matchers ...*labels.Matcher) (prom_storage.SeriesSet, prom_storage.Warnings, error) {
	r.log.Info("unable to contact nodes to read. attempting retries",
		logger.String("nodes", fmt.Sprintf("%v", nodes)))

	ticker, stop := NewExponentialTicker(TickerConfig{Context: r.ctx, MaxDelay: 30 * time.Second})
	defer stop()

	for {
		select {
		case <-ticker:
			result, warnings, err := r.queryWithNodeFailover(nodes, params, matchers...)
			if err == nil {
				r.log.Info("read retry successful")
				return result, warnings, nil
			} else if !isConnectionError(err) {
				r.log.Info("retry stopped on error", logger.Error(err))
				return nil, warnings, err
			}
		}
	}
}

func (r *ReplicatedQuerier) queryWithNodeFailover(nodes []int, params *prom_storage.SelectParams, matchers ...*labels.Matcher) (prom_storage.SeriesSet, prom_storage.Warnings, error) {
	routing.Shuffle(nodes)

	var result prom_storage.SeriesSet
	var warnings prom_storage.Warnings
	var err error
	for _, index := range nodes {
		remoteQuerier := r.queriers[index]
		if remoteQuerier == nil {
			continue
		}

		result, warnings, err = remoteQuerier.Select(params, matchers...)
		if !isConnectionError(err) {
			return result, warnings, err
		}
	}

	if err == nil {
		err = fmt.Errorf("metric does not exist on available nodes (%v)", nodes)
	}
	return nil, nil, err
}

func isConnectionError(err error) bool {
	var opError *net.OpError

	// necessary until promclient upgrades from pkg/errors
	for err != nil {
		cause, ok := err.(interface{ Cause() error })
		if !ok {
			break
		}
		err = cause.Cause()
	}
	return errors.As(err, &opError)
}

func (r *ReplicatedQuerier) LabelNames() ([]string, prom_storage.Warnings, error) {
	labelNamesMap := make(map[string]struct{})

	for _, querier := range r.queriers {
		labelNames, _, _ := querier.LabelNames()
		for _, labelName := range labelNames {
			labelNamesMap[labelName] = struct{}{}
		}
	}

	allLabelNames := make([]string, 0, len(labelNamesMap))
	for name := range labelNamesMap {
		allLabelNames = append(allLabelNames, name)
	}
	sort.Strings(allLabelNames)

	return allLabelNames, nil, nil
}

func (r *ReplicatedQuerier) LabelValues(name string) ([]string, prom_storage.Warnings, error) {
	var results [][]string

	for _, querier := range r.queriers {
		labelValues, _, _ := querier.LabelValues(name)
		results = append(results, labelValues)
	}

	return mergeStringSlices(results), nil, nil
}

func mergeStringSlices(ss [][]string) []string {
	switch len(ss) {
	case 0:
		return nil
	case 1:
		return ss[0]
	case 2:
		return mergeTwoStringSlices(ss[0], ss[1])
	default:
		halfway := len(ss) / 2
		return mergeTwoStringSlices(
			mergeStringSlices(ss[:halfway]),
			mergeStringSlices(ss[halfway:]),
		)
	}
}

func mergeTwoStringSlices(a, b []string) []string {
	i, j := 0, 0
	result := make([]string, 0, len(a)+len(b))
	for i < len(a) && j < len(b) {
		switch strings.Compare(a[i], b[j]) {
		case 0:
			result = append(result, a[i])
			i++
			j++
		case -1:
			result = append(result, a[i])
			i++
		case 1:
			result = append(result, b[j])
			j++
		}
	}
	result = append(result, a[i:]...)
	result = append(result, b[j:]...)
	return result
}

func (r *ReplicatedQuerier) Close() error {
	return nil
}
