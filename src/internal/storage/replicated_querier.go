package storage

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sort"
	"strings"
	"time"

	"github.com/cloudfoundry/metric-store-release/src/pkg/logger"
	config_util "github.com/prometheus/common/config"

	"github.com/cloudfoundry/metric-store-release/src/internal/routing"
	"github.com/prometheus/prometheus/pkg/labels"
	prom_storage "github.com/prometheus/prometheus/storage"
)

type ReplicatedQuerier struct {
	ctx context.Context

	store prom_storage.Storage

	routingTable Routing
	localIndex   int

	querierFactory QuerierFactory
	queryTimeout   time.Duration
	log            *logger.Logger
}

type QuerierFactory interface {
	Build(ctx context.Context, nodeIndexes ...int) []prom_storage.Querier
}

func NewReplicatedQuerierFactory(localStore prom_storage.Storage,
	localIndex int, nodeAddrs []string, egressTLSConfig *config_util.TLSConfig,
	log *logger.Logger) *ReplicatedQuerierFactory {
	return &ReplicatedQuerierFactory{
		localStore:      localStore,
		localIndex:      localIndex,
		nodeAddrs:       nodeAddrs,
		egressTLSConfig: egressTLSConfig,
		log:             log,
	}
}

type ReplicatedQuerierFactory struct {
	localStore      prom_storage.Storage
	localIndex      int
	nodeAddrs       []string
	egressTLSConfig *config_util.TLSConfig
	log             *logger.Logger
}

func (factory *ReplicatedQuerierFactory) Build(ctx context.Context, nodeIndexes ...int) []prom_storage.Querier {
	var queriers []prom_storage.Querier

	if nodeIndexes == nil {
		nodeIndexes = listIndexes(factory.nodeAddrs)
	}

	for _, i := range nodeIndexes {
		querier, err := factory.createQuerier(i, ctx)

		if querier == nil || err != nil {
			factory.log.Error("Could not create querier", err)
		} else {
			queriers = append(queriers, querier)
		}
	}
	return queriers
}

func (factory *ReplicatedQuerierFactory) createQuerier(i int, ctx context.Context) (prom_storage.Querier, error) {
	if i == factory.localIndex {
		return factory.localStore.Querier(ctx, 0, 0)
	} else {
		return NewRemoteQuerier(ctx, i, factory.nodeAddrs[i], factory.egressTLSConfig, factory.log)
	}
}

func listIndexes(nodeAddrs []string) []int {
	var nodeIndexes []int
	for i := range nodeAddrs {
		nodeIndexes = append(nodeIndexes, i)
	}
	return nodeIndexes
}

func NewReplicatedQuerier(ctx context.Context, localStore prom_storage.Storage, localIndex int, factory QuerierFactory,
	queryTimeout time.Duration, routingTable Routing, log *logger.Logger) *ReplicatedQuerier {
	return &ReplicatedQuerier{
		ctx:            ctx,
		store:          localStore,
		localIndex:     localIndex,
		querierFactory: factory,
		queryTimeout:   queryTimeout,
		routingTable:   routingTable,
		log:            log,
	}
}

func (r *ReplicatedQuerier) Select(params *prom_storage.SelectParams, matchers ...*labels.Matcher) (prom_storage.SeriesSet, prom_storage.Warnings, error) {
	ctx, cancel := context.WithTimeout(r.ctx, r.queryTimeout)
	defer cancel()

	metricName, err := r.extractMetricName(matchers)
	if err != nil {
		return nil, nil, err
	}

	if r.routingTable.IsLocal(metricName) {
		localQuerier, err := r.store.Querier(ctx, 0, 0)
		if err != nil {
			return nil, nil, err
		}
		return localQuerier.Select(params, matchers...)
	}

	return r.queryWithRetries(ctx, r.routingTable.Lookup(metricName), params, matchers...)
}

func (r *ReplicatedQuerier) SelectSorted(params *prom_storage.SelectParams, matchers ...*labels.Matcher) (prom_storage.SeriesSet, prom_storage.Warnings, error) {
	return r.Select(params, matchers...)
}

func (r *ReplicatedQuerier) extractMetricName(matchers []*labels.Matcher) (string, error) {
	for _, matcher := range matchers {
		if matcher.Name == labels.MetricName {
			if matcher.Type != labels.MatchEqual {
				return "", errors.New("only strict equality is supported for metric names")
			}
			return matcher.Value, nil
		}
	}
	return "", errors.New("no metric name present")
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
	queriers := r.querierFactory.Build(ctx, nodes...)

	var result prom_storage.SeriesSet
	var warnings prom_storage.Warnings
	var err error

	for _, remoteQuerier := range queriers {
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

	for _, querier := range r.querierFactory.Build(r.ctx) {
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

	for _, querier := range r.querierFactory.Build(r.ctx) {
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
