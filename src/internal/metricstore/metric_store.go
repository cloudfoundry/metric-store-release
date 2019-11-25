package metricstore

import (
	"context"
	"crypto/md5"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"sync/atomic"
	"time"

	"github.com/cloudfoundry/metric-store-release/src/internal/api"
	"github.com/cloudfoundry/metric-store-release/src/internal/debug"
	"github.com/cloudfoundry/metric-store-release/src/internal/logger"
	"github.com/cloudfoundry/metric-store-release/src/pkg/ingressclient"
	"github.com/cloudfoundry/metric-store-release/src/pkg/leanstreams"
	"go.uber.org/zap"

	"github.com/cloudfoundry/metric-store-release/src/internal/metrics"
	"github.com/cloudfoundry/metric-store-release/src/internal/storage"
	"github.com/cloudfoundry/metric-store-release/src/internal/version"
	"github.com/cloudfoundry/metric-store-release/src/pkg/persistence/transform"
	"github.com/cloudfoundry/metric-store-release/src/pkg/rpc"
	shared_tls "github.com/cloudfoundry/metric-store-release/src/pkg/tls"
	"github.com/niubaoshu/gotiny"

	config_util "github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/discovery"
	sd_config "github.com/prometheus/prometheus/discovery/config"
	"github.com/prometheus/prometheus/discovery/targetgroup"
	"github.com/prometheus/prometheus/notifier"
	prom_labels "github.com/prometheus/prometheus/pkg/labels"
	"github.com/prometheus/prometheus/promql"
	"github.com/prometheus/prometheus/rules"
	prom_storage "github.com/prometheus/prometheus/storage"
)

const (
	COMMON_NAME                         = "metric-store"
	MAX_BATCH_SIZE_IN_BYTES             = 32 * 1024
	MAX_INTERNODE_PAYLOAD_SIZE_IN_BYTES = 2 * MAX_BATCH_SIZE_IN_BYTES
	DEFAULT_EVALUATION_INTERVAL         = (1 * time.Minute)
)

var (
	SHA = "dev"
)

// MetricStore is a persisted store for Loggregator metrics (gauges, timers,
// counters).
type MetricStore struct {
	log *logger.Logger

	lis               net.Listener
	server            *http.Server
	ingressListener   *leanstreams.TCPListener
	internodeListener *leanstreams.TCPListener
	// internodeConns    chan *leanstreams.TCPClient

	ingressTLSConfig   *tls.Config
	internodeTLSConfig *tls.Config
	egressTLSConfig    *config_util.TLSConfig
	metrics            debug.MetricRegistrar
	closing            int64

	localStore        prom_storage.Storage
	ruleManager       *rules.Manager
	notifierManager   *notifier.Manager
	replicationFactor uint
	queryTimeout      time.Duration

	addr               string
	ingressAddr        string
	internodeAddr      string
	extAddr            string
	handoffStoragePath string

	nodeIndex int
	// nodeAddrs are the addresses of all the nodes (including the current
	// node). The index corresponds with the nodeIndex. It defaults to a
	// single bogus address so the node will not attempt to route data
	// externally and instead will store all of it.
	nodeAddrs []string
	// internodeAddrs are the addresses of all the internodes (including the current
	// node). The index corresponds with the nodeIndex. It defaults to a
	// single bogus address so the node will not attempt to route data
	// externally and instead will store all of it.
	internodeAddrs []string

	alertmanagerAddr string
	rulesPath        string

	replicatedStorage prom_storage.Storage
}

func New(localStore prom_storage.Storage, ingressTLSConfig, internodeTLSConfig *tls.Config, egressTLSConfig *config_util.TLSConfig, opts ...MetricStoreOption) *MetricStore {
	store := &MetricStore{
		log:     logger.NewNop(),
		metrics: &debug.NullRegistrar{},

		localStore:        localStore,
		replicationFactor: 1,
		queryTimeout:      10 * time.Second,

		addr:               ":8080",
		ingressAddr:        ":8090",
		internodeAddr:      ":8091",
		handoffStoragePath: "/tmp/metric-store/handoff",
		ingressTLSConfig:   ingressTLSConfig,
		internodeTLSConfig: internodeTLSConfig,
		egressTLSConfig:    egressTLSConfig,
	}

	for _, o := range opts {
		o(store)
	}

	// TODO: bring this back
	// store.internodeConns = make(chan *leanstreams.TCPClient, len(store.internodeAddrs))

	if len(store.nodeAddrs) == 0 {
		store.nodeAddrs = []string{store.addr}
	}

	return store
}

// MetricStoreOption configures a MetricStore.
type MetricStoreOption func(*MetricStore)

// WithLogger returns a MetricStoreOption that configures the logger used for
// the MetricStore. Defaults to silent logger.
func WithLogger(l *logger.Logger) MetricStoreOption {
	return func(store *MetricStore) {
		store.log = l
	}
}

// WithAddr configures the address to listen for API egress requests. It defaults to
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

// WithInternodeAddr configures the address to listen for internode writes.
// It defaults to :8091.
func WithInternodeAddr(internodeAddr string) MetricStoreOption {
	return func(store *MetricStore) {
		store.internodeAddr = internodeAddr
	}
}

// WithAlertmanagerAddr configures the address where an alertmanager is.
func WithAlertmanagerAddr(alertmanagerAddr string) MetricStoreOption {
	return func(store *MetricStore) {
		store.alertmanagerAddr = alertmanagerAddr
	}
}

// WithClustered enables the MetricStore to route data to peer nodes. It hashes
// each point by Name and SourceId and routes data that does not belong on the node
// to the correct node. NodeAddrs is a slice of node addresses where the slice
// index corresponds to the NodeIndex. The current node's address is included.
// The default is standalone mode where the MetricStore will store all the data
// and forward none of it.
func WithClustered(nodeIndex int, nodeAddrs, internodeAddrs []string) MetricStoreOption {
	// TODO: error check that these slices are the same length
	return func(store *MetricStore) {
		store.nodeIndex = nodeIndex
		store.nodeAddrs = nodeAddrs
		store.internodeAddrs = internodeAddrs
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
func WithMetrics(metrics debug.MetricRegistrar) MetricStoreOption {
	return func(store *MetricStore) {
		store.metrics = metrics
	}
}

// WithReplicationFactor sets the number of nodes that hold a copy of all
// data.
func WithReplicationFactor(replicationFactor uint) MetricStoreOption {
	return func(store *MetricStore) {
		store.replicationFactor = replicationFactor
	}
}

// WithHandoffStoragePath sets the base path for storing remote writes that are
// buffered due to errors.
func WithHandoffStoragePath(handoffStoragePath string) MetricStoreOption {
	return func(store *MetricStore) {
		store.handoffStoragePath = handoffStoragePath
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

func EngineQueryFunc(engine *promql.Engine, q prom_storage.Queryable) rules.QueryFunc {
	return func(ctx context.Context, qs string, t time.Time) (promql.Vector, error) {
		vector, err := rules.EngineQueryFunc(engine, q)(ctx, qs, t)
		if err != nil {
			return nil, err
		}

		samples := []promql.Sample{}
		for _, sample := range vector {
			samples = append(samples, promql.Sample{
				Point: promql.Point{
					T: transform.MillisecondsToNanoseconds(sample.T),
					V: sample.V,
				},
				Metric: sample.Metric,
			})
		}

		return promql.Vector(samples), nil
	}
}

// Start starts the MetricStore. It has an internal go-routine that it creates
// and therefore does not block.
func (store *MetricStore) Start() {
	promql.SetDefaultEvaluationInterval(DEFAULT_EVALUATION_INTERVAL)

	store.replicatedStorage = storage.NewReplicatedStorage(
		store.localStore,
		store.nodeIndex,
		store.nodeAddrs,
		store.internodeAddrs,
		store.replicationFactor,
		store.internodeTLSConfig,
		store.egressTLSConfig,
		storage.WithReplicatedLogger(store.log),
		storage.WithReplicatedHandoffStoragePath(store.handoffStoragePath),
		storage.WithReplicatedMetrics(store.metrics),
	)

	engineOpts := promql.EngineOpts{
		MaxConcurrent: 10,
		MaxSamples:    1e6,
		Timeout:       store.queryTimeout,
		Logger:        store.log,
		Reg:           store.metrics.Registerer(),
	}
	queryEngine := promql.NewEngine(engineOpts)

	options := &notifier.Options{
		QueueCapacity: 10,
		Registerer:    store.metrics.Registerer(),
	}
	store.notifierManager = notifier.NewManager(options, store.log)

	store.ruleManager = rules.NewManager(&rules.ManagerOptions{
		Appendable:  store.replicatedStorage,
		TSDB:        store.replicatedStorage,
		QueryFunc:   EngineQueryFunc(queryEngine, store.replicatedStorage),
		NotifyFunc:  sendAlerts(store.notifierManager),
		Context:     context.Background(),
		ExternalURL: &url.URL{},
		Logger:      store.log,
		Registerer:  store.metrics.Registerer(),
	})

	store.setupRouting(queryEngine)
	store.setupDirtyListener()
	store.setupSanitizedListener()

	go store.configureAlertManager()
	go store.processRules(queryEngine)
}

func (store *MetricStore) setupRouting(promQLEngine *promql.Engine) {
	egressAddr, err := net.ResolveTCPAddr("tcp", store.addr)
	if err != nil {
		store.log.Fatal("failed to resolve egress address", err)
	}

	insecureConnection, err := net.ListenTCP("tcp", egressAddr)
	if err != nil {
		store.log.Fatal("failed to listen", err)
	}

	tlsConfig, err := shared_tls.NewMutualTLSConfig(
		store.egressTLSConfig.CAFile,
		store.egressTLSConfig.CertFile,
		store.egressTLSConfig.KeyFile,
		store.egressTLSConfig.ServerName,
	)
	if err != nil {
		store.log.Fatal("failed to convert TLS config", err)
	}

	secureConnection := tls.NewListener(insecureConnection, tlsConfig)
	store.lis = secureConnection
	if store.extAddr == "" {
		store.extAddr = store.lis.Addr().String()
	}

	promAPI := api.NewPromAPI(
		promQLEngine,
		store.notifierManager,
		store.ruleManager,
		store.log,
	)

	apiV1 := promAPI.RouterForStorage(store.replicatedStorage)
	apiPrivate := promAPI.RouterForStorage(store.localStore)

	mux := http.NewServeMux()
	mux.Handle("/api/v1/", http.StripPrefix("/api/v1", apiV1))
	// TODO: extract private as constant
	mux.Handle("/private/api/v1/", http.StripPrefix("/private/api/v1", apiPrivate))

	mux.HandleFunc("/health", store.apiHealth)

	store.server = &http.Server{
		Handler:     mux,
		ErrorLog:    store.log.StdLog("egress"),
		ReadTimeout: store.queryTimeout,
	}

	go func() {
		store.log.Info("registration of egress reverse proxy is scheduled")
		if err := store.server.Serve(secureConnection); err != nil && atomic.LoadInt64(&store.closing) == 0 {
			store.log.Fatal("failed to serve http egress server", err)
		}
	}()
}

func (store *MetricStore) processRules(queryEngine *promql.Engine) {
	var rules []string

	if store.rulesPath != "" {
		rules = append(rules, store.rulesPath)
	}

	store.ruleManager.Update(5*time.Second, rules, nil)
	store.ruleManager.Run()
}

func (store *MetricStore) configureAlertManager() {
	if store.alertmanagerAddr == "" {
		return
	}

	discoveryManagerNotify := discovery.NewManager(
		context.Background(),
		store.log,
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
					APIVersion: config.AlertmanagerAPIVersionV2,
				},
			},
		},
	}

	if err := store.notifierManager.ApplyConfig(cfg); err != nil {
		store.log.Fatal("error Applying the config", err)
	}

	discoveredConfig := make(map[string]sd_config.ServiceDiscoveryConfig)
	for _, v := range cfg.AlertingConfig.AlertmanagerConfigs {
		// AlertmanagerConfigs doesn't hold an unique identifier so we use the config hash as the identifier.
		b, err := json.Marshal(v)
		if err != nil {
			store.log.Fatal("error parsing alertmanager config", err)
		}
		discoveredConfig[fmt.Sprintf("%x", md5.Sum(b))] = v.ServiceDiscoveryConfig
	}
	discoveryManagerNotify.ApplyConfig(discoveredConfig)
}

func (store *MetricStore) setupDirtyListener() {
	appender, _ := store.replicatedStorage.Appender()

	queuePoints := func(payload []byte) error {
		batch := rpc.Batch{}
		gotiny.Unmarshal(payload, &batch)

		// figure out which nodes this point belongs to and add it there
		points := batch.Points
		var ingressPointsTotal uint64

		for _, point := range points {
			if !transform.IsValidFloat(point.Value) {
				store.log.Debug("skipping point with invalid value", zap.Float64("value", point.Value))
				continue
			}

			labels := make(map[string]string)
			for label, value := range point.Labels {
				labels[transform.SanitizeLabelName(label)] = value
			}
			sanitizedName := transform.SanitizeMetricName(point.Name)
			labels[prom_labels.MetricName] = sanitizedName

			_, err := appender.Add(
				prom_labels.FromMap(labels),
				point.Timestamp,
				point.Value,
			)
			if err != nil {
				continue
			}

			ingressPointsTotal++
		}
		err := appender.Commit()
		if err != nil {
			return err
		}
		store.metrics.Add(debug.MetricStoreIngressPointsTotal, float64(ingressPointsTotal))

		return nil
	}

	cfg := leanstreams.TCPListenerConfig{
		MaxMessageSize: ingressclient.MAX_INGRESS_PAYLOAD_SIZE_IN_BYTES,
		Callback:       queuePoints,
		Address:        store.ingressAddr,
		TLSConfig:      store.ingressTLSConfig,
	}
	btl, err := leanstreams.ListenTCP(cfg)
	store.ingressListener = btl
	if err != nil {
		store.log.Fatal("failed to listen on ingress port", err)
	}

	err = btl.StartListeningAsync()
	if err != nil {
		store.log.Fatal("failed to start async listening on ingress port", err)
	}
}

// TODO - skip if no remote nodes?
func (store *MetricStore) setupSanitizedListener() {
	appender, _ := store.localStore.Appender()

	writePoints := func(payload []byte) error {
		// TODO: queue in diode
		batch := rpc.Batch{}
		gotiny.Unmarshal(payload, &batch)

		points := batch.Points
		var collectedPointsTotal uint64

		for _, point := range points {
			_, err := appender.Add(
				transform.ConvertLabels(point),
				point.Timestamp,
				point.Value,
			)
			if err != nil {
				continue
			}

			collectedPointsTotal++
		}
		err := appender.Commit()
		if err != nil {
			return err
		}
		store.metrics.Add(metrics.MetricStoreCollectedPointsTotal, float64(collectedPointsTotal))

		return nil
	}

	cfg := leanstreams.TCPListenerConfig{
		MaxMessageSize: MAX_INTERNODE_PAYLOAD_SIZE_IN_BYTES,
		Callback:       writePoints,
		Address:        store.internodeAddr,
		TLSConfig:      store.internodeTLSConfig,
	}
	btl, err := leanstreams.ListenTCP(cfg)
	store.internodeListener = btl
	if err != nil {
		store.log.Fatal("failed to listen on internode port", err)
	}

	err = btl.StartListeningAsync()
	if err != nil {
		store.log.Fatal("failed to start async listening on internode port", err)
	}
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

// Close will shutdown the servers
func (store *MetricStore) Close() error {
	atomic.AddInt64(&store.closing, 1)
	store.server.Shutdown(context.Background())

	// TODO: need to close remote connections
	// store.replicatedStorage.Close()
	store.ingressListener.Close()
	store.internodeListener.Close()
	return nil
}

func (store *MetricStore) apiHealth(w http.ResponseWriter, req *http.Request) {
	type healthInfo struct {
		Version string `json:"version"`
		Sha     string `json:"sha"`
	}

	responseData := healthInfo{
		Version: version.VERSION,
		Sha:     SHA,
	}

	responseBytes, err := json.Marshal(responseData)
	if err != nil {
		store.log.Error("Failed to marshal health check response", err)
	}

	w.Write(responseBytes)
	return
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
