package cluster_discovery

import (
	"bytes"
	"fmt"
	"github.com/cloudfoundry/metric-store-release/src/internal/cluster-discovery/pks"
	"github.com/cloudfoundry/metric-store-release/src/internal/debug"
	"github.com/cloudfoundry/metric-store-release/src/pkg/logger"
	prometheusConfig "github.com/prometheus/prometheus/config"
	"gopkg.in/yaml.v2"
	"net/http"
	"net/url"
	"text/template"
	"time"
)

// ClusterDiscovery queries the PKS API and generates a Prometheus scrape config file
// for all the available clusters.
type ClusterDiscovery struct {
	log     *logger.Logger
	metrics debug.MetricRegistrar

	auth                  authorizationProvider
	topology              topologyProvider
	store                 scrapeStore
	done                  chan bool
	refreshInterval       time.Duration
	metricStoreClient     *http.Client
	metricStoreAPIAddress string
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
	metricStoreAPIAddress string,
	metricStoreClient *http.Client,
	options ...WithOption,
) *ClusterDiscovery {
	discovery := &ClusterDiscovery{
		topology:              topologyProvider,
		auth:                  authClient,
		store:                 scrapeConfigStore,
		log:                   logger.NewNop(),
		metrics:               &debug.NullRegistrar{},
		done:                  make(chan bool, 1),
		refreshInterval:       time.Minute, //TODO expose this in the bosh release
		metricStoreAPIAddress: metricStoreAPIAddress,
		metricStoreClient:     metricStoreClient,
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

//TODO CHECK IF EXISTING SCRAPE CONFIG WORKS BEFORE GENERATING NEW ONE FOR A CLUSTER

// Start runs the discovery server and periodically writes an updated prometheus
// config file for each of the available PKS clusters.
func (discovery *ClusterDiscovery) Start() {
	go discovery.Run()
}

func (discovery *ClusterDiscovery) Run() {
	discovery.UpdateScrapeConfig()

	t := time.NewTicker(discovery.refreshInterval)
	for {
		select {
		case <-discovery.done:
			t.Stop()
			return
		case <-t.C:
			//TODO log number of new scrape configs created
			//maybe number of current configs as well
			discovery.UpdateScrapeConfig()
		}
	}
}

func (discovery *ClusterDiscovery) UpdateScrapeConfig() {
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

	combinedConfig := bytes.NewBufferString("scrape_configs:\n")
	for _, cluster := range clusters {
		scrapeConfigs, _ := discovery.getScrapeConfigsForCluster(&cluster)
		combinedConfig.WriteString(scrapeConfigs)
	}

	err = discovery.store.SaveScrapeConfig(combinedConfig.Bytes())
	if err != nil {
		discovery.log.Error("writing updated scrape config", err)
	}

	//TODO only reload metric-store config if we have new scrape configs added
	err = discovery.reloadMetricStoreConfiguration()
	if err != nil {
		discovery.log.Error("reloading metric store configuration", err)
	}
}

// Stop shuts down the ClusterDiscovery server, leaving the scrape config file in place.
func (discovery *ClusterDiscovery) Stop() {
	close(discovery.done)
}

func (discovery *ClusterDiscovery) getScrapeConfigsForCluster(cluster *pks.Cluster) (string, error) {
	err := discovery.saveCerts(cluster)
	if err != nil {
		return "", err
	}

	return discovery.populateScrapeTemplate(cluster)
}

func (discovery *ClusterDiscovery) saveCerts(cluster *pks.Cluster) error {
	// TODO talk to Bob
	//existing := discovery.loadConfigForCluster(cluster.Name)
	//if existing != nil {
	//	return existing
	//}
	err := discovery.store.SaveCA(cluster.Name, cluster.CaData)
	if err != nil {
		return err
	}

	certificateSigningRequest := NewCertificateSigningRequest(cluster.APIClient, WithCertificateSigningRequestLogger(discovery.log))
	clientCert, clientKey, err := certificateSigningRequest.RequestScraperCertificate()
	if err != nil {
		return err
	}

	err = discovery.store.SaveCert(cluster.Name, clientCert)
	if err != nil {
		return err
	}

	err = discovery.store.SavePrivateKey(cluster.Name, clientKey)
	if err != nil {
		return err
	}
	return nil
}

func (discovery *ClusterDiscovery) populateScrapeTemplate(cluster *pks.Cluster) (string, error) {
	var vars = ScrapeTemplate{
		ClusterName: cluster.Name,
		CAPath:      discovery.store.CAPath(cluster.Name),
		CertPath:    discovery.store.CertPath(cluster.Name),
		KeyPath:     discovery.store.PrivateKeyPath(cluster.Name),
		K8sApiAddr:  cluster.Addr,
		ServerName:  cluster.ServerName,
		MasterIps:   cluster.MasterIps,

		SkipSsl: true,
	}

	template, err := template.New("clusterConfig").Parse(scrapeTemplate)
	if err != nil {
		discovery.log.Error("unable to parse scrape config template", err)
		return "", err
	}

	var buffer bytes.Buffer
	err = template.Execute(&buffer, vars)
	if err != nil {
		discovery.log.Error("unable to populate scrape config template", err)
		return "", err
	}

	return buffer.String(), nil
}

func (discovery *ClusterDiscovery) loadConfigForCluster(_ string) []*prometheusConfig.ScrapeConfig {
	//do i have an existing scrap config?
	//can i connect to the k8s api using the tls config and get a 200 OK
	// maybe, check that cluster.Addr and config.apiServer are the same
	//if i get a 200 OK then add the existing scrape configs to config.ScrapeConfigs
	raw, err := discovery.store.LoadScrapeConfig()
	if err != nil {
		return nil
	}

	var cfg prometheusConfig.Config
	err = yaml.NewDecoder(bytes.NewReader(raw)).Decode(&cfg)
	return nil
}

func (discovery *ClusterDiscovery) reloadMetricStoreConfiguration() error {
	u := url.URL{
		Scheme: "https",
		Host:   discovery.metricStoreAPIAddress,
		Path:   "/~/reload",
	}
	resp, err := discovery.metricStoreClient.Post(u.String(), "application/json", nil)

	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d %s", resp.StatusCode, resp.Status)
	}
	return nil
}

type ScrapeTemplate struct {
	ClusterName string
	CAPath      string
	CertPath    string
	KeyPath     string
	SkipSsl     bool
	K8sApiAddr  string
	ServerName  string
	MasterIps   []string
}

var scrapeTemplate = `
- job_name: "{{.ClusterName}}-telegraf"
  metrics_path: "/metrics"
  scheme: "http"
  tls_config:
    insecure_skip_verify: {{ .SkipSsl }}
  relabel_configs:
  - target_label: "cluster"
    replacement: "{{.ClusterName}}"
  static_configs:
  - targets:
{{range $node := .MasterIps}}    - "{{$node}}:10200"
{{end}}
- job_name: "{{.ClusterName}}-kube-controller-manager"
  metrics_path: "/metrics"
  scheme: "http"
  tls_config:
    insecure_skip_verify: {{ .SkipSsl }}
  relabel_configs:
  - target_label: "cluster"
    replacement: "{{.ClusterName}}"
  static_configs:
  - targets:
{{range $node := .MasterIps}}    - "{{$node}}:10252"
{{end}}
- job_name: "{{.ClusterName}}-kube-scheduler"
  metrics_path: "/metrics"
  scheme: "http"
  tls_config:
    insecure_skip_verify: {{ .SkipSsl }}
  relabel_configs:
  - target_label: "cluster"
    replacement: "{{.ClusterName}}"
  static_configs:
  - targets:
{{range $node := .MasterIps}}    - "{{$node}}:10251"
{{end}}
- job_name: "{{.ClusterName}}-kubernetes-apiservers"
  metrics_path: "/metrics"
  scheme: "https"
  tls_config:
    ca_file: "{{.CAPath}}"
    cert_file: "{{.CertPath}}"
    key_file: "{{.KeyPath}}"
    insecure_skip_verify: {{ .SkipSsl }}
    server_name: "{{.ServerName}}"
  relabel_configs:
  - target_label: "cluster"
    replacement: "{{.ClusterName}}"
  static_configs:
  - targets:
    - {{.K8sApiAddr}}
- job_name: "{{.ClusterName}}-kubernetes-nodes"
  kubernetes_sd_configs:
  - role: "node"
    api_server: "https://{{.K8sApiAddr}}"
    tls_config:
      ca_file: "{{.CAPath}}"
      cert_file: "{{.CertPath}}"
      key_file: "{{.KeyPath}}"
      insecure_skip_verify: {{ .SkipSsl }}
      server_name: "{{.ServerName}}"
  scheme: "https"
  tls_config:
    ca_file: "{{.CAPath}}"
    cert_file: "{{.CertPath}}"
    key_file: "{{.KeyPath}}"
    insecure_skip_verify: {{ .SkipSsl }}
    server_name: "{{.ServerName}}"
  relabel_configs:
  - target_label: "cluster"
    replacement: "{{.ClusterName}}"
  - action: "labelmap"
    regex: "__meta_kubernetes_node_label_(.+)"
  - target_label: "__address__"
    replacement: "{{.K8sApiAddr}}"
  - source_labels:
    - "__meta_kubernetes_node_name"
    regex: "(.+)"
    target_label: "__metrics_path__"
    replacement: "/api/v1/nodes/$1/proxy/metrics"
- job_name: "{{.ClusterName}}-kubernetes-cadvisor"
  kubernetes_sd_configs:
  - role: "node"
    api_server: "https://{{.K8sApiAddr}}"
    tls_config:
      ca_file: "{{.CAPath}}"
      cert_file: "{{.CertPath}}"
      key_file: "{{.KeyPath}}"
      insecure_skip_verify: {{ .SkipSsl }}
      server_name: "{{.ServerName}}"
  scheme: "https"
  tls_config:
    ca_file: "{{.CAPath}}"
    cert_file: "{{.CertPath}}"
    key_file: "{{.KeyPath}}"
    insecure_skip_verify: {{ .SkipSsl }}
    server_name: "{{.ServerName}}"
  relabel_configs:
  - target_label: "cluster"
    replacement: "{{.ClusterName}}"
  - action: "labelmap"
    regex: "__meta_kubernetes_node_label_(.+)"
  - target_label: "__address__"
    replacement: "{{.K8sApiAddr}}"
  - source_labels:
    - "__meta_kubernetes_node_name"
    regex: "(.+)"
    target_label: "__metrics_path__"
    replacement: "/api/v1/nodes/$1/proxy/metrics/cadvisor"
- job_name: "{{.ClusterName}}-kube-state-metrics"
  kubernetes_sd_configs:
  - role: "pod"
    api_server: "https://{{.K8sApiAddr}}"
    tls_config:
      ca_file: "{{.CAPath}}"
      cert_file: "{{.CertPath}}"
      key_file: "{{.KeyPath}}"
      insecure_skip_verify: {{ .SkipSsl }}
      server_name: "{{.ServerName}}"
  scheme: "https"
  tls_config:
    ca_file: "{{.CAPath}}"
    cert_file: "{{.CertPath}}"
    key_file: "{{.KeyPath}}"
    insecure_skip_verify: {{ .SkipSsl }}
    server_name: "{{.ServerName}}"
  relabel_configs:
  - target_label: "cluster"
    replacement: "{{.ClusterName}}"
  - source_labels:
    - "__meta_kubernetes_namespace"
    - "__meta_kubernetes_pod_container_name"
    - "__meta_kubernetes_pod_container_port_name"
    action: "keep"
    regex: "(pks-system;kube-state-metrics;http-metrics|telemetry)"
  - target_label: "__address__"
    replacement: "{{.K8sApiAddr}}"
  - source_labels:
    - "__meta_kubernetes_namespace"
    - "__meta_kubernetes_pod_name"
    - "__meta_kubernetes_pod_container_port_number"
    action: "replace"
    regex: "(.+);(.+);(\\d+)"
    target_label: "__metrics_path__"
    replacement: "/api/v1/namespaces/$1/pods/$2:$3/proxy/metrics"
  - action: "labelmap"
    regex: "__meta_kubernetes_service_label_(.+)"
  - source_labels:
    - "__meta_kubernetes_namespace"
    action: "replace"
    target_label: "kubernetes_namespace"
  - source_labels:
    - "__meta_kubernetes_service_name"
    action: "replace"
    target_label: "kubernetes_name"
  - source_labels:
    - "__meta_kubernetes_pod_name"
    - "__meta_kubernetes_pod_container_port_number"
    action: "replace"
    regex: "(.+);(\\d+)"
    target_label: "instance"
    replacement: "$1:$2"
- job_name: "{{.ClusterName}}-kubernetes-coredns"
  kubernetes_sd_configs:
  - role: "pod"
    api_server: "https://{{.K8sApiAddr}}"
    tls_config:
      ca_file: "{{.CAPath}}"
      cert_file: "{{.CertPath}}"
      key_file: "{{.KeyPath}}"
      insecure_skip_verify: {{ .SkipSsl }}
      server_name: "{{.ServerName}}"
  scheme: "https"
  tls_config:
    ca_file: "{{.CAPath}}"
    cert_file: "{{.CertPath}}"
    key_file: "{{.KeyPath}}"
    insecure_skip_verify: {{ .SkipSsl }}
    server_name: "{{.ServerName}}"
  relabel_configs:
  - target_label: "cluster"
    replacement: "{{.ClusterName}}"
  - source_labels:
    - "__meta_kubernetes_pod_container_name"
    action: "keep"
    regex: "coredns"
  - target_label: "__address__"
    replacement: "{{.K8sApiAddr}}"
  - source_labels:
    - "__meta_kubernetes_namespace"
    - "__meta_kubernetes_pod_name"
    - "__meta_kubernetes_service_annotation_prometheus_io_port"
    action: "replace"
    regex: "(.+);(.+);(\\d+)"
    target_label: "__metrics_path__"
    replacement: "/api/v1/namespaces/$1/pods/$2:$3/proxy/metrics"
  - action: "labelmap"
    regex: "__meta_kubernetes_service_label_(.+)"
  - source_labels:
    - "__meta_kubernetes_namespace"
    action: "replace"
    target_label: "kubernetes_namespace"
  - source_labels:
    - "__meta_kubernetes_service_name"
    action: "replace"
    target_label: "kubernetes_name"
  - source_labels:
    - "__meta_kubernetes_pod_name"
    action: "replace"
    target_label: "instance"
`
