package storage

import (
	// You will need to make sure this import exists for side effects:
	// _ "github.com/influxdata/influxdb/tsdb/engine"
	// the go linter in some instances removes it

	"context"
	"crypto/tls"
	"fmt"
	"strconv"
	"time"

	"github.com/cloudfoundry/metric-store-release/src/internal/debug"
	"github.com/cloudfoundry/metric-store-release/src/internal/routing"
	"github.com/cloudfoundry/metric-store-release/src/pkg/leanstreams"
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
	routingTable       *routing.RoutingTable
	localStore         prom_storage.Storage
	queryTimeout       time.Duration

	internodeTLSConfig *tls.Config
	egressTLSConfig    *config_util.TLSConfig

	internodeConnections []*leanstreams.Connection
	replayerClosers      []chan struct{}
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
	store := &ReplicatedStorage{
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
		opt(store)
	}

	// TODO handle this error
	store.routingTable, _ = routing.NewRoutingTable(localIndex, store.nodeAddrs, store.replicationFactor)

	// TODO handle this error
	_ = store.createAppenders()

	return store
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
			connection := leanstreams.NewConnection(addr, r.internodeTLSConfig, MAX_INTERNODE_PAYLOAD_SIZE_IN_BYTES)
			r.internodeConnections = append(r.internodeConnections, connection)
			done := make(chan struct{})
			r.replayerClosers = append(r.replayerClosers, done)

			remoteAppender := NewRemoteAppender(
				strconv.Itoa(nodeIndex),
				connection,
				done,
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
	factory := NewReplicatedQuerierFactory(r.localStore, r.localIndex,
		r.nodeAddrs, r.egressTLSConfig, r.log)
	return NewReplicatedQuerier(
		ctx,
		r.localStore,
		r.localIndex,
		factory,
		r.queryTimeout,
		r.routingTable,
		r.log,
	), nil
}

func (r *ReplicatedStorage) StartTime() (int64, error) {
	panic("not implemented")
}

func (r *ReplicatedStorage) Appender() (storage.Appender, error) {
	return NewReplicatedAppender(
		r.appenders,
		r.routingTable.Lookup,
		WithReplicatedAppenderLogger(r.log),
		WithReplicatedAppenderMetrics(r.metrics),
	), nil
}

func (r *ReplicatedStorage) Close() error {
	for _, closer := range r.replayerClosers {
		close(closer)
	}

	var closeErrors []error
	for _, connection := range r.internodeConnections {
		err := connection.Close()
		if err != nil {
			closeErrors = append(closeErrors, err)
		}
	}

	if len(closeErrors) > 0 {
		return fmt.Errorf("failed to close %d of %d internode connections: %v", len(closeErrors), len(r.internodeConnections), closeErrors)
	}
	return nil
}
