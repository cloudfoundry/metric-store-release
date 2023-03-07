package storage

import (
	"context"
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

	querier := &RemoteQuerier{
		ctx:           ctx,
		index:         index,
		addr:          addr,
		publicClient:  publicClient,
		privateClient: privateClient,
		log:           logger,
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

	minTime, maxTime, _ := DefaultTimeRangeAndMatches()

	result := make([]string, len(matchers))
	for i, matcher := range matchers {
		result[i] = matcher.String()
	}

	labelValuesResult, _, err := r.privateClient.LabelValues(r.ctx, name, result, minTime,
		maxTime)
	if err != nil {
		return nil, nil, err
	}

	for _, labelValue := range labelValuesResult {
		results = append(results, string(labelValue))
	}

	return results, nil, nil
}

func (r *RemoteQuerier) LabelNames(matchers ...*labels.Matcher) ([]string, prom_storage.Warnings, error) {
	minTime, maxTime, _ := DefaultTimeRangeAndMatches()

	result := make([]string, len(matchers))
	for i, matcher := range matchers {
		result[i] = matcher.String()
	}

	res, _, err := r.privateClient.LabelNames(r.ctx, result, minTime, maxTime)
	return res, nil, err
}

func (r *RemoteQuerier) Close() error {
	return nil
}
