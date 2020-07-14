package metric_store

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/gob"
	"encoding/json"
	"github.com/prometheus/prometheus/pkg/logging"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"go.uber.org/zap"

	"github.com/cloudfoundry/metric-store-release/src/internal/api"
	"github.com/cloudfoundry/metric-store-release/src/internal/metrics"
	"github.com/cloudfoundry/metric-store-release/src/internal/rules"
	"github.com/cloudfoundry/metric-store-release/src/internal/scraping"
	"github.com/cloudfoundry/metric-store-release/src/internal/storage"
	shared_tls "github.com/cloudfoundry/metric-store-release/src/internal/tls"
	"github.com/cloudfoundry/metric-store-release/src/internal/version"
	"github.com/cloudfoundry/metric-store-release/src/pkg/ingressclient"
	"github.com/cloudfoundry/metric-store-release/src/pkg/leanstreams"
	"github.com/cloudfoundry/metric-store-release/src/pkg/logger"
	"github.com/cloudfoundry/metric-store-release/src/pkg/persistence/transform"
	"github.com/cloudfoundry/metric-store-release/src/pkg/rpc"

	config_util "github.com/prometheus/common/config"
	prom_labels "github.com/prometheus/prometheus/pkg/labels"
	"github.com/prometheus/prometheus/promql"
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
	egressServer      *http.Server
	ingressListener   *leanstreams.TCPListener
	internodeListener *leanstreams.TCPListener

	ingressTLSConfig         *tls.Config
	internodeTLSServerConfig *tls.Config
	internodeTLSClientConfig *tls.Config
	egressTLSConfig          *config_util.TLSConfig
	metrics                  metrics.Registrar
	closing                  int64

	localStore        prom_storage.Storage
	promRuleManagers  rules.RuleManagers
	replicationFactor uint

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

	scraper     *scraping.Scraper
	storagePath string

	replicatedStorage prom_storage.Storage

	// Query Engine Parameters
	queryTimeout         time.Duration
	activeQueryLogPath   string
	logQueries           bool
	maxConcurrentQueries int
}

func New(localStore prom_storage.Storage, storagePath string, ingressTLSConfig, internodeTLSServerConfig, internodeTLSClientConfig *tls.Config,
	egressTLSConfig *config_util.TLSConfig, opts ...MetricStoreOption) *MetricStore {
	store := &MetricStore{
		log:     logger.NewNop(),
		metrics: &metrics.NullRegistrar{},

		localStore:        localStore,
		replicationFactor: 1,
		queryTimeout:      10 * time.Second,

		addr:                     ":8080",
		ingressAddr:              ":8090",
		internodeAddr:            ":8091",
		handoffStoragePath:       "/tmp/metric-store/handoff",
		ingressTLSConfig:         ingressTLSConfig,
		internodeTLSServerConfig: internodeTLSServerConfig,
		internodeTLSClientConfig: internodeTLSClientConfig,
		egressTLSConfig:          egressTLSConfig,
		storagePath:              storagePath,

		activeQueryLogPath:   filepath.Join(storagePath, "activequeries"),
		maxConcurrentQueries: 20,
		logQueries:           false,
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

// WithMetrics returns a MetricStoreOption that configures the metrics for the
// MetricStore. It will add metrics to the given map.
func WithMetrics(metrics metrics.Registrar) MetricStoreOption {
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

func WithScraper(scraper *scraping.Scraper) MetricStoreOption {
	return func(store *MetricStore) {
		store.scraper = scraper
	}
}

/////////////////////////////////////////////////
// Query Engine Options

func WithConcurrentQueryLimit(maxQueries int) MetricStoreOption {
	return func(store *MetricStore) {
		store.maxConcurrentQueries = maxQueries
	}
}
func WithQueryLogging(doLog bool) MetricStoreOption {
	return func(store *MetricStore) {
		store.logQueries = doLog
	}
}

// WithQueryTimeout sets the maximum duration of a PromQL query.
func WithQueryTimeout(queryTimeout time.Duration) MetricStoreOption {
	return func(store *MetricStore) {
		store.queryTimeout = queryTimeout
	}
}

// End Query Engine Options
////////////////////////////////////////////////////

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
		store.internodeTLSClientConfig,
		store.egressTLSConfig,
		store.queryTimeout,
		storage.WithReplicatedLogger(store.log),
		storage.WithReplicatedHandoffStoragePath(store.handoffStoragePath),
		storage.WithReplicatedMetrics(store.metrics),
	)

	queryEngine := store.createEngine()

	store.promRuleManagers = rules.NewRuleManagers(
		store.replicatedStorage,
		queryEngine,
		time.Duration(promql.DefaultEvaluationInterval)*time.Millisecond,
		store.log,
		store.metrics,
		store.queryTimeout,
	)

	store.setupIngressListener()
	store.setupDistributionListener()

	if store.scraper != nil {
		store.scraper.Run(storage.NewScrapeStorage(store.replicatedStorage))
	}

	store.setupRouting(queryEngine)
	go store.loadRules(queryEngine)
}

func (store *MetricStore) createEngine() *promql.Engine {
	engineOpts := promql.EngineOpts{
		MaxSamples:         20e6,
		Timeout:            store.queryTimeout,
		Logger:             store.log,
		Reg:                store.metrics.Registerer(),
		ActiveQueryTracker: promql.NewActiveQueryTracker(store.activeQueryLogPath, store.maxConcurrentQueries, store.log),
		LookbackDelta:      5 * time.Minute,
	}
	queryEngine := promql.NewEngine(engineOpts)

	if store.logQueries {
		// Is there a way to not have to hardcode this path?
		queryLogger, err := logging.NewJSONFileLogger("/var/vcap/sys/log/metric-store/query.log")
		if err != nil {
			store.log.Error("Unable to create query log", err)
		} else {
			queryEngine.SetQueryLogger(queryLogger)
		}
	}
	return queryEngine
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

	tlsServerConfig, err := shared_tls.NewMutualTLSServerConfig(
		store.egressTLSConfig.CAFile,
		store.egressTLSConfig.CertFile,
		store.egressTLSConfig.KeyFile,
	)
	if err != nil {
		store.log.Fatal("failed to convert TLS egressServer config", err)
	}

	secureConnection := tls.NewListener(insecureConnection, tlsServerConfig)
	store.lis = secureConnection
	if store.extAddr == "" {
		store.extAddr = store.lis.Addr().String()
	}

	rulesStoragePath := filepath.Join(store.storagePath, "rule_managers")
	err = os.Mkdir(rulesStoragePath, os.ModePerm)
	if err != nil && !os.IsExist(err) {
		store.log.Fatal("failed to create rules storage dir", err)
	}
	localRuleManager := rules.NewLocalRuleManager(
		rulesStoragePath,
		store.promRuleManagers,
	)

	// TODO is this necessary?
	tlsClientConfig, err := shared_tls.NewMutualTLSClientConfig(
		store.egressTLSConfig.CAFile,
		store.egressTLSConfig.CertFile,
		store.egressTLSConfig.KeyFile,
		store.egressTLSConfig.ServerName,
	)
	if err != nil {
		store.log.Fatal("failed to convert TLS client config", err)
	}

	replicatedRuleManager := rules.NewReplicatedRuleManager(
		localRuleManager,
		store.nodeIndex,
		store.nodeAddrs,
		store.replicationFactor,
		tlsClientConfig,
	)

	promAPI := api.NewPromAPI(
		promQLEngine,
		store.log,
	)

	apiV1 := promAPI.RouterForStorage(store.replicatedStorage, replicatedRuleManager)
	apiPrivate := promAPI.RouterForStorage(store.localStore, localRuleManager)

	rulesAPI := api.NewRulesAPI(replicatedRuleManager, store.log)
	rulesAPIRouter := rulesAPI.Router()

	reloadAPI := api.NewReloadAPI(store.scraper.LoadConfigs, store.log)

	localRulesAPI := api.NewRulesAPI(localRuleManager, store.log)
	localRulesAPIRouter := localRulesAPI.Router()

	mux := http.NewServeMux()
	mux.Handle("/api/v1/", http.StripPrefix("/api/v1", apiV1))
	// TODO: extract private as constant
	mux.Handle("/private/api/v1/", http.StripPrefix("/private/api/v1", apiPrivate))
	mux.Handle("/rules/", http.StripPrefix("/rules", rulesAPIRouter))
	mux.Handle("/private/rules/", http.StripPrefix("/private/rules", localRulesAPIRouter))
	mux.Handle("/~/", http.StripPrefix("/~", reloadAPI.Router()))

	mux.HandleFunc("/health", store.apiHealth)

	store.egressServer = &http.Server{
		Handler:     mux,
		ErrorLog:    store.log.StdLog("egress"),
		ReadTimeout: store.queryTimeout,
	}

	go func() {
		store.log.Info("registration of egress reverse proxy is scheduled")
		if err := store.egressServer.Serve(secureConnection); err != nil && atomic.LoadInt64(&store.closing) == 0 {
			store.log.Fatal("failed to serve http egress Server", err)
		}
	}()
}

func (store *MetricStore) loadRules(promQLEngine *promql.Engine) {
	rulesDir := filepath.Join(store.storagePath, "rule_managers")
	directories, err := ioutil.ReadDir(rulesDir)
	if err != nil {
		store.log.Error("no rules are available", err)
		return
	}

	ruleManagerFile := rules.NewRuleManagerFiles(rulesDir)

	for _, directory := range directories {
		// TODO: skip files
		// if !directory.IsDir() {
		// 	continue
		// }

		managerId := directory.Name()
		promRulesFile, alertManagers, err := ruleManagerFile.Load(managerId)
		if err != nil {
			store.log.Error("could not parse rule file", err, logger.String("file", managerId))
			continue
		}

		_ = store.promRuleManagers.Create(managerId, promRulesFile, alertManagers)
	}
}

// ingress is for a points not necessarily stored on this node
func (store *MetricStore) setupIngressListener() {
	appender := store.replicatedStorage.Appender()

	queuePoints := func(payload []byte) error {
		network := bytes.NewBuffer(payload)
		dec := gob.NewDecoder(network)
		batch := rpc.Batch{}
		err := dec.Decode(&batch)
		if err != nil {
			store.log.Error("gob decode error", err)
			return err
		}

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
		err = appender.Commit()
		if err != nil {
			return err
		}
		store.metrics.Add(metrics.MetricStoreIngressPointsTotal, float64(ingressPointsTotal))

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
// collected is a point from another node for storage
func (store *MetricStore) setupDistributionListener() {
	appender := store.localStore.Appender()

	writePoints := func(payload []byte) error {
		// TODO: queue in diode
		network := bytes.NewBuffer(payload)
		dec := gob.NewDecoder(network)
		batch := rpc.Batch{}
		err := dec.Decode(&batch)
		if err != nil {
			store.log.Error("gob decode error", err)
		}

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
		err = appender.Commit()
		if err != nil {
			return err
		}
		store.metrics.Add(metrics.MetricStoreCollectedPointsTotal, float64(collectedPointsTotal)) // TODO: better name?

		return nil
	}

	cfg := leanstreams.TCPListenerConfig{
		MaxMessageSize: MAX_INTERNODE_PAYLOAD_SIZE_IN_BYTES,
		Callback:       writePoints,
		Address:        store.internodeAddr,
		TLSConfig:      store.internodeTLSServerConfig,
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
	_ = store.egressServer.Shutdown(context.Background())
	// if store.discoveryAgent != nil {
	// 	store.discoveryAgent.Stop()
	// }
	// if store.scrapeManager != nil {
	// 	store.scrapeManager.Stop()
	// }
	_ = store.promRuleManagers.DeleteAll()
	_ = store.replicatedStorage.Close()
	store.ingressListener.Close()
	store.internodeListener.Close()
	return nil
}

func (store *MetricStore) apiHealth(w http.ResponseWriter, _ *http.Request) {
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

	_, _ = w.Write(responseBytes)
	return
}
