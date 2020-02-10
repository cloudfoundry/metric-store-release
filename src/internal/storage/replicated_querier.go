package storage

import (
	"context"
	"errors"
	"fmt"
	"github.com/cloudfoundry/metric-store-release/src/pkg/logger"
	config_util "github.com/prometheus/common/config"
	"net"
	"sort"
	"strings"
	"time"

	"github.com/cloudfoundry/metric-store-release/src/internal/routing"
	"github.com/prometheus/prometheus/pkg/labels"
	prom_storage "github.com/prometheus/prometheus/storage"
)

type ReplicatedQuerier struct {
	ctx          context.Context
	store        prom_storage.Storage
	lookup       routing.Lookup
	localIndex   int
	factory      QuerierFactory
	queryTimeout time.Duration
	log          *logger.Logger
}

type QuerierFactory func(ctx context.Context) []prom_storage.Querier

func ReplicatedQuerierFactory(localStore prom_storage.Storage, localIndex int, nodeAddrs []string, egressTLSConfig *config_util.TLSConfig, log *logger.Logger) QuerierFactory {
	return func(ctx context.Context) []prom_storage.Querier {
		queriers := make([]prom_storage.Querier, len(nodeAddrs))

		for i, addr := range nodeAddrs {
			if i != localIndex {
				remoteQuerier, err := NewRemoteQuerier(
					ctx,
					i,
					addr,
					egressTLSConfig,
					log,
				)

				if err != nil {
					log.Error("Could not create remote querier", err)
					continue
				}
				queriers[i] = remoteQuerier

				continue
			}

			localQuerier, _ := localStore.Querier(ctx, 0, 0)
			queriers[i] = localQuerier
		}
		return queriers
	}
}

func NewReplicatedQuerier(ctx context.Context, localStore prom_storage.Storage, localIndex int, factory QuerierFactory,
	queryTimeout time.Duration, lookup routing.Lookup, log *logger.Logger, ) *ReplicatedQuerier {
	return &ReplicatedQuerier{
		ctx:        ctx,
		store:      localStore,
		localIndex: localIndex,
		factory:   factory,
		queryTimeout: queryTimeout,
		lookup:     lookup,
		log:        log,
	}
}

func (r *ReplicatedQuerier) Select(params *prom_storage.SelectParams, matchers ...*labels.Matcher) (prom_storage.SeriesSet, prom_storage.Warnings, error) {
	ctx, cancel := context.WithTimeout(r.ctx, r.queryTimeout)
	defer cancel()

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
		localQuerier, err := r.store.Querier(ctx, 0, 0)
		if err != nil {
			return nil, nil, err
		}
		return localQuerier.Select(params, matchers...)
	}

	return r.queryWithRetries(ctx, nodesWithMetric, params, matchers...)
}

func (r *ReplicatedQuerier) queryWithRetries(ctx context.Context, nodes []int, params *prom_storage.SelectParams, matchers ...*labels.Matcher) (prom_storage.SeriesSet, prom_storage.Warnings, error) {
	result, warnings, err := r.queryWithNodeFailover(ctx, nodes, params, matchers...)

	if isConnectionError(err) {
		return r.retryQueryWithBackoff(ctx, nodes, params, matchers...)
	} else {
		return result, warnings, err
	}
}

func (r *ReplicatedQuerier) retryQueryWithBackoff(ctx context.Context, nodes []int, params *prom_storage.SelectParams, matchers ...*labels.Matcher) (prom_storage.SeriesSet, prom_storage.Warnings, error) {
	r.log.Info("unable to contact nodes to read. attempting retries",
		logger.String("nodes", fmt.Sprintf("%v", nodes)))

	ticker, stop := NewExponentialTicker(TickerConfig{Context: ctx, MaxDelay: 30 * time.Second})
	defer stop()

	for {
		select {
		case <-ticker:
			result, warnings, err := r.queryWithNodeFailover(ctx, nodes, params, matchers...)
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

func (r *ReplicatedQuerier) queryWithNodeFailover(ctx context.Context, nodes []int, params *prom_storage.SelectParams, matchers ...*labels.Matcher) (prom_storage.SeriesSet, prom_storage.Warnings, error) {
	routing.Shuffle(nodes)
	queriers := r.factory(ctx)

	var result prom_storage.SeriesSet
	var warnings prom_storage.Warnings
	var err error
	for _, index := range nodes {
		remoteQuerier := queriers[index]
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

	for _, querier := range r.factory(r.ctx) {
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

	for _, querier := range r.factory(r.ctx) {
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
