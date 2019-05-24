package metricstore

import (
	"crypto/tls"
	"io/ioutil"
	"log"
	"math"
	"net"
	"sync/atomic"
	"time"

	"google.golang.org/grpc"

	"github.com/cloudfoundry/metric-store-release/src/pkg/leanstreams"
	"github.com/cloudfoundry/metric-store-release/src/pkg/local"
	"github.com/cloudfoundry/metric-store-release/src/pkg/metrics"
	"github.com/cloudfoundry/metric-store-release/src/pkg/persistence/transform"
	"github.com/cloudfoundry/metric-store-release/src/pkg/query"
	rpc "github.com/cloudfoundry/metric-store-release/src/pkg/rpc/metricstore_v1"
	"github.com/gogo/protobuf/proto"
	"github.com/prometheus/prometheus/pkg/labels"
	"github.com/prometheus/prometheus/storage"
)

const MAX_HASH = math.MaxUint64

type RetentionConfig struct {
	RetentionPeriod       time.Duration
	ExpiryFrequency       time.Duration
	DiskFreePercentTarget float64
}

type PersistentStore interface {
	Put(points []*rpc.Point)
	Get(*storage.SelectParams, ...*labels.Matcher) (storage.SeriesSet, error)
	DeleteOlderThan(cutoff time.Time)
	DeleteOldest()
	EmitStorageDurationMetric()
	Close()
	Labels() (*rpc.PromQL_LabelsQueryResult, error)
	LabelValues(*rpc.PromQL_LabelValuesQueryRequest) (*rpc.PromQL_LabelValuesQueryResult, error)
}

type diskFreeReporter func() float64

// MetricStore is a persisted store for Loggregator metrics (gauges, timers,
// counters).
type MetricStore struct {
	log *log.Logger

	lis             net.Listener
	server          *grpc.Server
	ingressListener *leanstreams.TCPListener

	serverOpts []grpc.ServerOption
	metrics    metrics.Initializer
	closing    int64

	persistentStore     PersistentStore
	diskFreeReporter    diskFreeReporter
	expiryConfig        RetentionConfig
	queryTimeout        time.Duration
	metricsEmitDuration time.Duration

	addr        string
	ingressAddr string
	extAddr     string
	tlsConfig   *tls.Config
}

func New(persistentStore PersistentStore, diskFreeReporter diskFreeReporter, tlsConfig *tls.Config, opts ...MetricStoreOption) *MetricStore {
	store := &MetricStore{
		log:     log.New(ioutil.Discard, "", 0),
		metrics: metrics.NullMetrics{},

		persistentStore:  persistentStore,
		diskFreeReporter: diskFreeReporter,
		expiryConfig: RetentionConfig{
			ExpiryFrequency:       time.Duration(math.MaxInt64),
			RetentionPeriod:       time.Duration(math.MaxInt64),
			DiskFreePercentTarget: 0,
		},
		queryTimeout:        10 * time.Second,
		metricsEmitDuration: 10 * time.Minute,
		addr:                ":8080",
		ingressAddr:         ":8090",
		tlsConfig:           tlsConfig,
	}

	for _, o := range opts {
		o(store)
	}

	return store
}

// MetricStoreOption configures a MetricStore.
type MetricStoreOption func(*MetricStore)

// WithLogger returns a MetricStoreOption that configures the logger used for
// the MetricStore. Defaults to silent logger.
func WithLogger(l *log.Logger) MetricStoreOption {
	return func(store *MetricStore) {
		store.log = l
	}
}

// WithAddr configures the address to listen for gRPC requests. It defaults to
// :8080.
func WithAddr(addr string) MetricStoreOption {
	return func(store *MetricStore) {
		store.addr = addr
	}
}

// WithIngressAddr configures the address to listen for ingress. It defaults to
// :8090.
func WithIngressAddr(ingressAddr string) MetricStoreOption {
	return func(store *MetricStore) {
		store.ingressAddr = ingressAddr
	}
}

// WithServerOpts configures the gRPC server options. It defaults to an
// empty list.
func WithServerOpts(opts ...grpc.ServerOption) MetricStoreOption {
	return func(store *MetricStore) {
		store.serverOpts = opts
	}
}

// WithExternalAddr returns a MetricStoreOption that sets address that peer
// nodes will refer to the given node as. This is required when the set
// address won't match what peers will refer to the node as (e.g. :0).
// Defaults to the resulting address from the listener.
func WithExternalAddr(addr string) MetricStoreOption {
	return func(store *MetricStore) {
		store.extAddr = addr
	}
}

// WithRetentionConfig sets the frequency of automated cleanup that expires old
// data.
func WithRetentionConfig(config RetentionConfig) MetricStoreOption {
	return func(store *MetricStore) {
		store.expiryConfig = config
	}
}

// WithMetrics returns a MetricStoreOption that configures the metrics for the
// MetricStore. It will add metrics to the given map.
func WithMetrics(m metrics.Initializer) MetricStoreOption {
	return func(store *MetricStore) {
		store.metrics = m
	}
}

// WithQueryTimeout sets the maximum duration of a PromQL query.
func WithQueryTimeout(queryTimeout time.Duration) MetricStoreOption {
	return func(store *MetricStore) {
		store.queryTimeout = queryTimeout
	}
}

// WithMetricsEmitDuration sets the duration to which periodic metrics are
// emitted
func WithMetricsEmitDuration(metricsEmitDuration time.Duration) MetricStoreOption {
	return func(store *MetricStore) {
		store.metricsEmitDuration = metricsEmitDuration
	}
}

// Start starts the MetricStore. It has an internal go-routine that it creates
// and therefore does not block.
func (store *MetricStore) Start() {
	go store.periodicExpiry(store.persistentStore)
	go store.periodicMetrics(store.persistentStore)
	store.setupRouting(store.persistentStore)
}

func (store *MetricStore) periodicExpiry(ps PersistentStore) {
	store.deleteExpiredData(ps)
	for range time.Tick(store.expiryConfig.ExpiryFrequency) {
		store.deleteExpiredData(ps)
	}
}

func (store *MetricStore) periodicMetrics(ps PersistentStore) {
	ps.EmitStorageDurationMetric()
	for range time.Tick(store.metricsEmitDuration) {
		ps.EmitStorageDurationMetric()
	}
}

func (store *MetricStore) deleteExpiredData(ps PersistentStore) {
	cutoff := time.Now().Add(-store.expiryConfig.RetentionPeriod)
	store.log.Printf("expiring data older than %s", cutoff.Format(time.RFC3339))
	ps.DeleteOlderThan(cutoff)

	diskFree := store.diskFreeReporter()
	if diskFree < store.expiryConfig.DiskFreePercentTarget {
		store.log.Printf("expiring data due to disk free space of %.0f%%", diskFree)
		ps.DeleteOldest()
	}
}

func (store *MetricStore) setupRouting(s PersistentStore) {
	// gRPC
	lis, err := net.Listen("tcp", store.addr)
	if err != nil {
		store.log.Fatalf("failed to listen: %v", err)
	}
	store.lis = lis
	store.log.Printf("listening on %s...", store.Addr())

	if store.extAddr == "" {
		store.extAddr = store.lis.Addr().String()
	}

	localStoreReader := local.NewLocalStoreReader(s)

	writeBinary := func(payload []byte) error {
		r := &rpc.SendRequest{}
		var points []*rpc.Point

		err := proto.Unmarshal(payload, r)
		if err != nil {
			return err
		}

		for _, point := range r.Batch.Points {
			if !transform.IsValidFloat(point.Value) {
				continue
			}

			point.Name = transform.SanitizeMetricName(point.GetName())

			sanitizedLabels := make(map[string]string)
			for label, value := range point.GetLabels() {
				sanitizedLabels[transform.SanitizeLabelName(label)] = value
			}
			if len(sanitizedLabels) > 0 {
				point.Labels = sanitizedLabels
			}

			points = append(points, point)
		}

		s.Put(points)

		return nil
	}

	cfg := leanstreams.TCPListenerConfig{
		MaxMessageSize: 65536,
		Callback:       writeBinary,
		Address:        store.ingressAddr,
		TLSConfig:      store.tlsConfig,
	}
	btl, err := leanstreams.ListenTCP(cfg)
	store.ingressListener = btl
	if err != nil {
		store.log.Fatalf("failed to listen on ingress port: %v", err)
	}

	err = btl.StartListeningAsync()
	if err != nil {
		store.log.Fatalf("failed to start async listening on ingress port: %v", err)
	}

	queryEngine := query.NewEngine(
		store.metrics,
		query.WithQueryTimeout(store.queryTimeout),
		query.WithLogger(store.log),
	)
	egressReverseProxy := local.NewEgressReverseProxy(
		localStoreReader,
		queryEngine,
		local.WithLogger(store.log),
	)
	store.server = grpc.NewServer(store.serverOpts...)

	go func() {
		rpc.RegisterPromQLAPIServer(store.server, egressReverseProxy)
		if err := store.server.Serve(lis); err != nil && atomic.LoadInt64(&store.closing) == 0 {
			store.log.Fatalf("failed to serve gRPC ingress server: %s %#v", err, err)
		}
	}()
}

// Addr returns the address that the MetricStore is listening on. This is only
// valid after Start has been invoked.
func (store *MetricStore) Addr() string {
	return store.lis.Addr().String()
}

// IngressAddr returns the address that the MetricStore is listening on for ingress.
// This is only valid after Start has been invoked.
func (store *MetricStore) IngressAddr() string {
	return store.ingressListener.Address
}

// Close will shutdown the gRPC server
func (store *MetricStore) Close() error {
	atomic.AddInt64(&store.closing, 1)
	store.server.GracefulStop()
	store.ingressListener.Close()
	return nil
}
