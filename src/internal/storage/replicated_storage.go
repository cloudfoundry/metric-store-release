package storage

import (
	// You will need to make sure this import exists for side effects:
	// _ "github.com/influxdata/influxdb/tsdb/engine"
	// the go linter in some instances removes it

	"context"
	"crypto/tls"
	"strconv"
	"time"

	"github.com/cloudfoundry/metric-store-release/src/internal/debug"
	"github.com/cloudfoundry/metric-store-release/src/internal/routing"
	"github.com/cloudfoundry/metric-store-release/src/pkg/logger"
	_ "github.com/influxdata/influxdb/tsdb/engine"
	config_util "github.com/prometheus/common/config"
	"github.com/prometheus/prometheus/storage"
	prom_storage "github.com/prometheus/prometheus/storage"
)

type ReplicatedStorage struct {
	log     *logger.Logger
	metrics debug.MetricRegistrar

	localIndex         int
	nodeAddrs          []string
	internodeAddrs     []string
	replicationFactor  uint
	appenders          []prom_storage.Appender
	handoffStoragePath string
	lookup             routing.Lookup
	localStore         prom_storage.Storage
	queryTimeout       time.Duration

	internodeTLSConfig *tls.Config
	egressTLSConfig    *config_util.TLSConfig
}

func NewReplicatedStorage(
	localStore prom_storage.Storage,
	localIndex int,
	nodeAddrs []string,
	internodeAddrs []string,
	replicationFactor uint,
	internodeTLSConfig *tls.Config,
	egressTLSConfig *config_util.TLSConfig,
	queryTimeout time.Duration,
	opts ...ReplicatedOption,
) prom_storage.Storage {
	storage := &ReplicatedStorage{
		log:                logger.NewNop(),
		metrics:            &debug.NullRegistrar{},
		localStore:         localStore,
		localIndex:         localIndex,
		nodeAddrs:          nodeAddrs,
		internodeAddrs:     internodeAddrs,
		replicationFactor:  replicationFactor,
		handoffStoragePath: "/tmp/metric-store/handoff",
		appenders:          make([]prom_storage.Appender, len(internodeAddrs)),
		internodeTLSConfig: internodeTLSConfig,
		egressTLSConfig:    egressTLSConfig,
		queryTimeout:       queryTimeout,
	}

	for _, opt := range opts {
		opt(storage)
	}

	routingTable, _ := routing.NewRoutingTable(storage.nodeAddrs, storage.replicationFactor)
	storage.lookup = routingTable.Lookup

	storage.createAppenders()

	return storage
}

type ReplicatedOption func(*ReplicatedStorage)

func WithReplicatedLogger(log *logger.Logger) ReplicatedOption {
	return func(s *ReplicatedStorage) {
		s.log = log
	}
}

func WithReplicatedHandoffStoragePath(handoffStoragePath string) ReplicatedOption {
	return func(s *ReplicatedStorage) {
		s.handoffStoragePath = handoffStoragePath
	}
}

func WithReplicatedMetrics(metrics debug.MetricRegistrar) ReplicatedOption {
	return func(s *ReplicatedStorage) {
		s.metrics = metrics
	}
}

func (r *ReplicatedStorage) createAppenders() error {
	for nodeIndex, addr := range r.internodeAddrs {
		if nodeIndex != r.localIndex {
			remoteAppender := NewRemoteAppender(
				strconv.Itoa(nodeIndex),
				addr,
				r.internodeTLSConfig,
				WithRemoteAppenderHandoffStoragePath(r.handoffStoragePath),
				WithRemoteAppenderLogger(r.log),
				WithRemoteAppenderMetrics(r.metrics),
			)
			r.appenders[nodeIndex] = remoteAppender

			continue
		}

		localAppender, err := r.localStore.Appender()
		if err != nil {
			return err
		}
		r.appenders[nodeIndex] = localAppender
	}

	return nil
}

func (r *ReplicatedStorage) Querier(ctx context.Context, _, _ int64) (storage.Querier, error) {
	factory := ReplicatedQuerierFactory(r.localStore, r.localIndex, r.nodeAddrs, r.egressTLSConfig, r.log)
	return NewReplicatedQuerier(
		ctx,
		r.localStore,
		r.localIndex,
		factory,
		r.queryTimeout,
		r.lookup,
		r.log,
	), nil
}

func (r *ReplicatedStorage) StartTime() (int64, error) {
	panic("not implemented")
}

func (r *ReplicatedStorage) Appender() (storage.Appender, error) {
	return NewReplicatedAppender(
		r.appenders,
		r.lookup,
		WithReplicatedAppenderLogger(r.log),
		WithReplicatedAppenderMetrics(r.metrics),
	), nil
}

func (r *ReplicatedStorage) Close() error {
	return nil
}
