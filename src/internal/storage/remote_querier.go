package storage

import (
	"context"
	"math"
	"time"

	"github.com/cloudfoundry/metric-store-release/src/internal/api"
	shared_api "github.com/cloudfoundry/metric-store-release/src/internal/api"
	shared_tls "github.com/cloudfoundry/metric-store-release/src/internal/tls"
	"github.com/cloudfoundry/metric-store-release/src/pkg/logger"
	prom_api_client "github.com/prometheus/client_golang/api/prometheus/v1"
	config_util "github.com/prometheus/common/config"
	"github.com/prometheus/prometheus/model/labels"
	prom_storage "github.com/prometheus/prometheus/storage"
	"github.com/prometheus/prometheus/storage/remote"
)

type RemoteQuerier struct {
	ctx           context.Context
	index         int
	addr          string
	publicClient  remote.ReadClient
	privateClient prom_api_client.API
	log           *logger.Logger
	minTime       time.Time
	maxTime       time.Time
}

func NewRemoteQuerier(
	ctx context.Context,
	index int,
	addr string,
	egressTLSConfig *config_util.TLSConfig,
	logger *logger.Logger,
) (prom_storage.Querier, error) {
	// TODO - remote query timeout should probably not be a hardcoded value?
	publicClient, err := api.NewPromReadClient(
		index,
		addr,
		30*time.Second,
		egressTLSConfig,
	)
	if err != nil {
		return nil, err
	}

	privateTLSConfig, err := shared_tls.NewMutualTLSClientConfig(
		egressTLSConfig.CAFile,
		egressTLSConfig.CertFile,
		egressTLSConfig.KeyFile,
		egressTLSConfig.ServerName,
	)
	if err != nil {
		return nil, err
	}

	privateClient, err := shared_api.NewPromHTTPClient(
		addr,
		"/private",
		privateTLSConfig,
	)
	if err != nil {
		return nil, err
	}
	minTime := time.Unix(math.MinInt64/1000+62135596801, 0).UTC()
	maxTime := time.Unix(math.MaxInt64/1000-62135596801, 999999999).UTC()

	querier := &RemoteQuerier{
		ctx:           ctx,
		index:         index,
		addr:          addr,
		publicClient:  publicClient,
		privateClient: privateClient,
		log:           logger,
		minTime:       minTime,
		maxTime:       maxTime,
	}
	return querier, nil
}

func (r *RemoteQuerier) Select(sortSeries bool, params *prom_storage.SelectHints, matchers ...*labels.Matcher) prom_storage.SeriesSet {
	query, err := remote.ToQuery(0, 0, matchers, params)
	if err != nil {
		return nil
	}

	res, err := r.publicClient.Read(r.ctx, query)
	if err != nil {
		return nil
	}
	return remote.FromQueryResult(sortSeries, res)
}

func (r *RemoteQuerier) LabelValues(name string, matchers ...*labels.Matcher) ([]string,
	prom_storage.Warnings, error) {
	var results []string

	result := make([]string, len(matchers))
	for i, matcher := range matchers {
		result[i] = matcher.String()
	}

	labelValuesResult, _, err := r.privateClient.LabelValues(r.ctx, name, result, r.minTime,
		r.maxTime)
	if err != nil {
		return nil, nil, err
	}

	for _, labelValue := range labelValuesResult {
		results = append(results, string(labelValue))
	}

	return results, nil, nil
}

func (r *RemoteQuerier) LabelNames(matchers ...*labels.Matcher) ([]string, prom_storage.Warnings, error) {

	result := make([]string, len(matchers))
	for i, matcher := range matchers {
		result[i] = matcher.String()
	}

	res, _, err := r.privateClient.LabelNames(r.ctx, result, r.minTime, r.maxTime)
	return res, nil, err
}

func (r *RemoteQuerier) Close() error {
	return nil
}
