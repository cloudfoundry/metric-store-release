package cluster_discovery

import (
	"bytes"
	"github.com/cloudfoundry/metric-store-release/src/internal/cluster-discovery/pks"
	"github.com/cloudfoundry/metric-store-release/src/internal/debug"
	"github.com/cloudfoundry/metric-store-release/src/pkg/logger"
	prometheusCommonConfig "github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	prometheusConfig "github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/discovery/config"
	kubernetesDiscovery "github.com/prometheus/prometheus/discovery/kubernetes"
	"github.com/prometheus/prometheus/pkg/relabel"
	"go.uber.org/zap"
	"gopkg.in/yaml.v2"
	"net/url"
	"time"
)

// ClusterDiscovery queries the PKS API and generates a Prometheus scrape config file
// for all the available clusters.
type ClusterDiscovery struct {
	log     *logger.Logger
	metrics debug.MetricRegistrar

	auth            authorizationProvider
	topology        topologyProvider
	store           scrapeStore
	done            chan bool
	refreshInterval time.Duration
}

type authorizationProvider interface {
	GetAuthHeader() (string, error)
}

type scrapeStore interface {
	SaveCA(name string, data []byte) error
	SaveCert(name string, clientCert []byte) error
	SavePrivateKey(name string, data []byte) error
	SaveScrapeConfig(contents []byte) error

	Path() string
	CAPath(clusterName string) string
	CertPath(clusterName string) string
	PrivateKeyPath(clusterName string) string

	LoadScrapeConfig() ([]byte, error)
}

type topologyProvider interface {
	GetClusters(authHeader string) ([]pks.Cluster, error)
}

// New returns a configured ClusterDiscovery
func New(
	scrapeConfigStore scrapeStore,
	topologyProvider topologyProvider,
	authClient authorizationProvider,
	options ...WithOption,
) *ClusterDiscovery {
	discovery := &ClusterDiscovery{
		topology:        topologyProvider,
		auth:            authClient,
		store:           scrapeConfigStore,
		log:             logger.NewNop(),
		metrics:         &debug.NullRegistrar{},
		done:            make(chan bool, 1),
		refreshInterval: time.Minute,
	}

	for _, option := range options {
		option(discovery)
	}
	return discovery
}

type WithOption func(discovery *ClusterDiscovery)

func WithRefreshInterval(interval time.Duration) WithOption {
	return func(discovery *ClusterDiscovery) {
		discovery.refreshInterval = interval
	}
}

func WithLogger(log *logger.Logger) WithOption {
	return func(discovery *ClusterDiscovery) {
		discovery.log = log
	}
}

func WithMetrics(metrics debug.MetricRegistrar) WithOption {
	return func(discovery *ClusterDiscovery) {
		discovery.metrics = metrics
	}
}

//TODO DELETE CERTS/KEYS WHEN SCRAPE NO LONGER WORKS
//TODO CHECK IF EXISTING SCRAPE CONFIG WORKS BEFORE GENERATING NEW ONE FOR A CLUSTER

// Start runs the discovery server and periodically writes an updated prometheus
// config file for each of the available PKS clusters.
func (discovery *ClusterDiscovery) Start() {
	go discovery.run()
}

func (discovery *ClusterDiscovery) run() {
	discovery.updateScrapeConfig()

	t := time.NewTicker(discovery.refreshInterval)
	for {
		select {
		case <-discovery.done:
			t.Stop()
			return
		case <-t.C:
			discovery.updateScrapeConfig()
		}
	}
}

func (discovery *ClusterDiscovery) updateScrapeConfig() {
	authHeader, err := discovery.auth.GetAuthHeader()
	if err != nil {
		discovery.log.Error("getting auth header", err)
		return
	}

	clusters, err := discovery.topology.GetClusters(authHeader)
	if err != nil {
		discovery.log.Error("getting cluster list", err)
		return
	}

	scrapeConfig := &prometheusConfig.Config{}
	for _, cluster := range clusters {
		scrapeConfig.ScrapeConfigs = append(scrapeConfig.ScrapeConfigs, discovery.getScrapeConfigsForCluster(&cluster)...)
	}

	contents, err := yaml.Marshal(scrapeConfig)
	if err != nil {
		discovery.log.Debug("attempting to marshal", zap.Any("scrape config", scrapeConfig))
		discovery.log.Error("converting scrape config to yaml", err)
		return
	}

	// TODO scrape configs need to be combined with other configs
	// from various places (scrape metric store itself, etc),
	// so this shouldn't expect to create a full prom config
	err = discovery.store.SaveScrapeConfig(contents)
	if err != nil {
		discovery.log.Error("writing updated scrape config", err)
	}
}

// Stop shuts down the ClusterDiscovery server, leaving the scrape config file in place.
func (discovery *ClusterDiscovery) Stop() {
	close(discovery.done)
}

func (discovery *ClusterDiscovery) getScrapeConfigsForCluster(cluster *pks.Cluster) []*prometheusConfig.ScrapeConfig {
	// TODO talk to Bob
	//existing := discovery.loadConfigForCluster(cluster.Name)
	//if existing != nil {
	//	return existing
	//}

	_ = discovery.store.SaveCA(cluster.Name, []byte(cluster.CaData))

	certificateSigningRequest := NewCertificateSigningRequest(cluster.APIClient)
	clientCert, clientKey, err := certificateSigningRequest.RequestScraperCertificate()
	panicIfError(err) // TODO fix me

	err = discovery.store.SaveCert(cluster.Name, clientCert)
	panicIfError(err) // TODO fix me

	err = discovery.store.SavePrivateKey(cluster.Name, clientKey)
	panicIfError(err) // TODO fix me

	return []*prometheusConfig.ScrapeConfig{
		kubernetesNodesScrapeConfig(cluster, discovery.store),
		//kubernetesAPIServersScrapeConfig(cluster, discovery.store),
	}
}

func (discovery *ClusterDiscovery) loadConfigForCluster(_ string) []*prometheusConfig.ScrapeConfig {
	//do i have an existing scrap config?
	//can i connect to the k8s api using the tls config and get a 200 OK
	// maybe, check that cluster.Host and config.apiServer are the same
	//if i get a 200 OK then add the existing scrape configs to config.ScrapeConfigs
	raw, err := discovery.store.LoadScrapeConfig()
	if err != nil {
		return nil
	}

	var cfg prometheusConfig.Config
	err = yaml.NewDecoder(bytes.NewReader(raw)).Decode(&cfg)
	return nil
}

func kubernetesNodesScrapeConfig(cluster *pks.Cluster, store scrapeStore) *prometheusConfig.ScrapeConfig {
	return &prometheusConfig.ScrapeConfig{
		JobName: cluster.Name + "-kubernetes-nodes",
		ServiceDiscoveryConfig: config.ServiceDiscoveryConfig{
			KubernetesSDConfigs: []*kubernetesDiscovery.SDConfig{{
				APIServer: prometheusCommonConfig.URL{URL: &url.URL{Host: cluster.Host}},
				Role:      "node",
				HTTPClientConfig: prometheusCommonConfig.HTTPClientConfig{
					TLSConfig: prometheusCommonConfig.TLSConfig{
						CAFile:   store.CAPath(cluster.Name),
						CertFile: store.CertPath(cluster.Name),
						KeyFile:  store.PrivateKeyPath(cluster.Name),
					},
				},
			}},
		},
		Scheme: "https",
		HTTPClientConfig: prometheusCommonConfig.HTTPClientConfig{
			TLSConfig: prometheusCommonConfig.TLSConfig{
				CAFile:   store.CAPath(cluster.Name),
				CertFile: store.CertPath(cluster.Name),
				KeyFile:  store.PrivateKeyPath(cluster.Name),
			},
		},
		RelabelConfigs: []*relabel.Config{
			{
				Action: relabel.LabelMap,
				Regex:  relabel.MustNewRegexp("__meta_kubernetes_node_label_(.+)"),
			},
			{
				TargetLabel: "__address__",
				Replacement: cluster.Host,
			},
			{
				TargetLabel: "__metrics_path__",
				Replacement: "/api/v1/nodes/$1/proxy/metrics",
				Regex:       relabel.MustNewRegexp("(.+)"),
				SourceLabels: model.LabelNames{
					"__meta_kubernetes_node_name",
				},
			},
		},
	}
}

func kubernetesAPIServersScrapeConfig(cluster *pks.Cluster, store scrapeStore) *prometheusConfig.ScrapeConfig {
var _ = `
- job_name: "cluster1-kubernetes-apiservers"
  kubernetes_sd_configs:
  - role: "endpoints"
    api_server: "https://localhost:$k8sApiPort"
    tls_config:
      ca_file: "/tmp/cluster1_ca.pem"
      cert_file: "/tmp/cluster1_cert.pem"
      key_file: "/tmp/cluster1_cert.key"
      insecure_skip_verify: false
  scheme: "https"
  tls_config:
    ca_file: "/tmp/cluster1_ca.pem"
    cert_file: "/tmp/cluster1_cert.pem"
    key_file: "/tmp/cluster1_cert.key"
    insecure_skip_verify: false
  relabel_configs:
  - source_labels:
    - "__meta_kubernetes_namespace"
    - "__meta_kubernetes_service_name"
    - "__meta_kubernetes_endpoint_port_name"
    action: "keep"
    regex: "default;kubernetes;https"`
	return &prometheusConfig.ScrapeConfig{
		JobName: cluster.Name + "-kubernetes-apiservers",
		ServiceDiscoveryConfig: config.ServiceDiscoveryConfig{
			KubernetesSDConfigs: []*kubernetesDiscovery.SDConfig{{
				APIServer: prometheusCommonConfig.URL{URL: &url.URL{Host: cluster.Host}},
				Role:      "endpoints",
				HTTPClientConfig: prometheusCommonConfig.HTTPClientConfig{
					TLSConfig: prometheusCommonConfig.TLSConfig{
						CAFile:   store.CAPath(cluster.Name),
						CertFile: store.CertPath(cluster.Name),
						KeyFile:  store.PrivateKeyPath(cluster.Name),
					},
				},
			}},
		},
		Scheme: "https",
		HTTPClientConfig: prometheusCommonConfig.HTTPClientConfig{
			TLSConfig: prometheusCommonConfig.TLSConfig{
				CAFile:   store.CAPath(cluster.Name),
				CertFile: store.CertPath(cluster.Name),
				KeyFile:  store.PrivateKeyPath(cluster.Name),
			},
		},
		RelabelConfigs: []*relabel.Config{
			{
				SourceLabels: []model.LabelName{
					"__meta_kubernetes_namespace",
					"__meta_kubernetes_service_name",
					"__meta_kubernetes_endpoint_port_name",
				},
				Action: relabel.Keep,
				Regex:  relabel.MustNewRegexp("default;kubernetes;https"),
			},
			{
				TargetLabel: "__address__",
				Replacement: cluster.Host,
			},
			{
				TargetLabel: "__metrics_path__",
				Replacement: "/api/v1/nodes/$1/proxy/metrics",
				Regex:       relabel.MustNewRegexp("(.+)"),
				SourceLabels: model.LabelNames{
					"__meta_kubernetes_node_name",
				},
			},
		},
	}
}

func panicIfError(err error) {
	if err != nil {
		panic(err)
	}
}
