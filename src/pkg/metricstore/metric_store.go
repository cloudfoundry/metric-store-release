package metricstore

import (
	"context"
	"crypto/md5"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"net"
	"net/http"
	"net/url"
	"os"
	"sync/atomic"
	"time"

	"github.com/cloudfoundry/metric-store-release/src/pkg/api"
	"github.com/cloudfoundry/metric-store-release/src/pkg/leanstreams"
	"github.com/cloudfoundry/metric-store-release/src/pkg/metrics"
	"github.com/cloudfoundry/metric-store-release/src/pkg/persistence/transform"
	"github.com/cloudfoundry/metric-store-release/src/pkg/rpc"
	promLog "github.com/go-kit/kit/log"
	"github.com/niubaoshu/gotiny"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/discovery"
	sd_config "github.com/prometheus/prometheus/discovery/config"
	"github.com/prometheus/prometheus/discovery/targetgroup"
	"github.com/prometheus/prometheus/notifier"
	"github.com/prometheus/prometheus/pkg/labels"
	"github.com/prometheus/prometheus/promql"
	"github.com/prometheus/prometheus/rules"
	"github.com/prometheus/prometheus/storage"
)

const (
	COMMON_NAME = "metric-store"
	MAX_HASH    = math.MaxUint64
)

// MetricStore is a persisted store for Loggregator metrics (gauges, timers,
// counters).
type MetricStore struct {
	log *log.Logger

	lis             net.Listener
	server          *http.Server
	ingressListener *leanstreams.TCPListener

	egressTLSConfig  *tls.Config
	ingressTLSConfig *tls.Config
	metrics          metrics.Initializer
	closing          int64

	persistentStore storage.Storage
	ruleManager     *rules.Manager
	notifierManager *notifier.Manager
	queryTimeout    time.Duration

	incIngress func(uint64)

	addr        string
	ingressAddr string
	extAddr     string

	alertmanagerAddr string
	rulesPath        string
}

func New(persistentStore storage.Storage, egressTLSConfig, ingressTLSConfig *tls.Config, opts ...MetricStoreOption) *MetricStore {
	store := &MetricStore{
		log:     log.New(ioutil.Discard, "", 0),
		metrics: metrics.NullMetrics{},

		persistentStore:  persistentStore,
		queryTimeout:     10 * time.Second,
		addr:             ":8080",
		ingressAddr:      ":8090",
		egressTLSConfig:  egressTLSConfig,
		ingressTLSConfig: ingressTLSConfig,
	}

	for _, o := range opts {
		o(store)
	}

	store.incIngress = store.metrics.NewCounter("metric_store_ingress")

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

// WithAlertmanagerAddr configures the address where an alertmanager is.
func WithAlertmanagerAddr(alertmanagerAddr string) MetricStoreOption {
	return func(store *MetricStore) {
		store.alertmanagerAddr = alertmanagerAddr
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

// WithRulesPath sets the path where configuration for alerting rules can be
// found
func WithRulesPath(rulesPath string) MetricStoreOption {
	return func(store *MetricStore) {
		store.rulesPath = rulesPath
	}
}

// Start starts the MetricStore. It has an internal go-routine that it creates
// and therefore does not block.
func (store *MetricStore) Start() {
	engineOpts := promql.EngineOpts{
		MaxConcurrent: 10,
		MaxSamples:    1e6,
		Timeout:       store.queryTimeout,
	}
	queryEngine := promql.NewEngine(engineOpts)

	store.setupRouting(queryEngine)

	options := &notifier.Options{
		QueueCapacity: 10,
	}
	store.notifierManager = notifier.NewManager(options, promLog.NewNopLogger())

	go store.configureAlertManager()
	go store.processRules(queryEngine)
}

func (store *MetricStore) processRules(queryEngine *promql.Engine) {
	if store.rulesPath == "" {
		return
	}

	store.ruleManager = rules.NewManager(&rules.ManagerOptions{
		Appendable:  store.persistentStore,
		TSDB:        store.persistentStore,
		QueryFunc:   rules.EngineQueryFunc(queryEngine, store.persistentStore),
		NotifyFunc:  sendAlerts(store.notifierManager),
		Context:     context.Background(),
		ExternalURL: &url.URL{},
		Logger:      promLog.NewLogfmtLogger(promLog.NewSyncWriter(os.Stderr)),
	})

	store.ruleManager.Update(5*time.Second, []string{store.rulesPath}, nil)
	store.ruleManager.Run()
}

func (store *MetricStore) configureAlertManager() {
	if store.alertmanagerAddr == "" {
		return
	}

	discoveryManagerNotify := discovery.NewManager(
		context.Background(),
		promLog.NewNopLogger(),
		discovery.Name("notify"),
	)
	go discoveryManagerNotify.Run()
	go store.notifierManager.Run(discoveryManagerNotify.SyncCh())

	cfg := &config.Config{
		AlertingConfig: config.AlertingConfig{
			AlertmanagerConfigs: []*config.AlertmanagerConfig{
				{
					ServiceDiscoveryConfig: sd_config.ServiceDiscoveryConfig{
						StaticConfigs: []*targetgroup.Group{
							{
								Targets: []model.LabelSet{
									{
										"__address__": model.LabelValue(store.alertmanagerAddr),
									},
								},
							},
						},
					},
					Scheme:     "http",
					Timeout:    10000000000,
					APIVersion: config.AlertmanagerAPIVersionV1,
				},
			},
		},
	}

	if err := store.notifierManager.ApplyConfig(cfg); err != nil {
		store.log.Fatalf("Error Applying the config: %v", err)
	}

	discoveredConfig := make(map[string]sd_config.ServiceDiscoveryConfig)
	for _, v := range cfg.AlertingConfig.AlertmanagerConfigs {
		// AlertmanagerConfigs doesn't hold an unique identifier so we use the config hash as the identifier.
		b, err := json.Marshal(v)
		if err != nil {
			store.log.Fatalf("Error parsing alertmanager config")
		}
		discoveredConfig[fmt.Sprintf("%x", md5.Sum(b))] = v.ServiceDiscoveryConfig
	}
	discoveryManagerNotify.ApplyConfig(discoveredConfig)
}

func (store *MetricStore) setupRouting(promQLEngine *promql.Engine) {
	// Ingress Setup
	writeBinary := func(payload []byte) error {
		batch := rpc.Batch{}

		gotiny.Unmarshal(payload, &batch)

		appender, _ := store.persistentStore.Appender()
		var totalPointsWritten uint64
		for _, point := range batch.Points {
			sanitizedLabels := make(map[string]string)
			sanitizedLabels["__name__"] = transform.SanitizeMetricName(point.Name)

			for label, value := range point.Labels {
				sanitizedLabels[transform.SanitizeLabelName(label)] = value
			}

			_, err := appender.Add(labels.FromMap(sanitizedLabels), point.Timestamp, point.Value)
			if err != nil {
				continue
			}
			totalPointsWritten++
		}

		err := appender.Commit()
		if err != nil {
			return err
		}
		store.incIngress(totalPointsWritten)

		return nil
	}

	cfg := leanstreams.TCPListenerConfig{
		MaxMessageSize: 65536,
		Callback:       writeBinary,
		Address:        store.ingressAddr,
		TLSConfig:      store.ingressTLSConfig,
	}
	ingressConnection, err := leanstreams.ListenTCP(cfg)
	store.ingressListener = ingressConnection
	if err != nil {
		store.log.Fatalf("failed to listen on ingress port: %v", err)
	}

	err = ingressConnection.StartListeningAsync()
	if err != nil {
		store.log.Fatalf("failed to start async listening on ingress port: %v", err)
	}

	// Egress Setup
	egressAddr, err := net.ResolveTCPAddr("tcp", store.addr)
	if err != nil {
		store.log.Fatalf("failed to resolve egress address: %v", err)
	}

	insecureConnection, err := net.ListenTCP("tcp", egressAddr)
	if err != nil {
		store.log.Fatalf("failed to listen: %v", err)
	}

	secureConnection := tls.NewListener(insecureConnection, store.egressTLSConfig)

	store.lis = secureConnection
	store.log.Printf("listening on %s...", store.Addr())

	promAPI := api.NewPromAPI(promQLEngine)
	apiV1 := promAPI.RouterForStorage(store.persistentStore)

	mux := http.NewServeMux()
	mux.Handle("/api/v1/", http.StripPrefix("/api/v1", apiV1))

	store.server = &http.Server{
		Handler:     mux,
		ErrorLog:    store.log,
		ReadTimeout: store.queryTimeout,
	}

	go func() {
		if err := store.server.Serve(secureConnection); err != nil && atomic.LoadInt64(&store.closing) == 0 {
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
	store.server.Shutdown(context.Background())
	store.ingressListener.Close()
	return nil
}

func sendAlerts(s *notifier.Manager) rules.NotifyFunc {
	return func(ctx context.Context, expr string, alerts ...*rules.Alert) {
		var res []*notifier.Alert

		for _, alert := range alerts {
			a := &notifier.Alert{
				StartsAt:    alert.FiredAt,
				Labels:      alert.Labels,
				Annotations: alert.Annotations,
			}
			if !alert.ResolvedAt.IsZero() {
				a.EndsAt = alert.ResolvedAt
			} else {
				a.EndsAt = alert.ValidUntil
			}
			res = append(res, a)
		}

		if len(alerts) > 0 {
			s.Send(res...)
		}
	}
}
