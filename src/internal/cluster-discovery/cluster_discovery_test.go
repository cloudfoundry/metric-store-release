package cluster_discovery_test

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	cluster_discovery "github.com/cloudfoundry/metric-store-release/src/internal/cluster-discovery"
	"github.com/cloudfoundry/metric-store-release/src/internal/cluster-discovery/kubernetes"
	"github.com/cloudfoundry/metric-store-release/src/internal/cluster-discovery/pks"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"github.com/prometheus/common/model"
	prometheusConfig "github.com/prometheus/prometheus/config"
	kubernetesDiscovery "github.com/prometheus/prometheus/discovery/kubernetes"
	"github.com/prometheus/prometheus/pkg/relabel"
	"go.uber.org/atomic"
	"gopkg.in/yaml.v2"
	certificates "k8s.io/api/certificates/v1beta1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"time"
)

type certificateMock struct {
	generatedCSRs     atomic.Int32
	certificateString string
	pending           bool
	privateKey        *ecdsa.PrivateKey
}

func (mock *certificateMock) GenerateCSR() (csrPEM []byte, key *ecdsa.PrivateKey, err error) {
	mock.generatedCSRs.Inc()
	return []byte(mock.certificateString), mock.privateKey, nil
}

func (mock *certificateMock) RequestCertificate(csrData []byte, privateKey interface{}) (req *certificates.CertificateSigningRequest, err error) {
	return nil, nil
}

func (mock *certificateMock) UpdateApproval(_ *certificates.CertificateSigningRequest) (result *certificates.CertificateSigningRequest, err error) {
	return nil, nil
}

func (mock *certificateMock) Get(options v1.GetOptions) (*certificates.CertificateSigningRequest, error) {
	if mock.pending {
		return &certificates.CertificateSigningRequest{
			Status: certificates.CertificateSigningRequestStatus{
				Conditions:  []certificates.CertificateSigningRequestCondition{},
				Certificate: nil,
			},
		}, nil
	}

	return &certificates.CertificateSigningRequest{
		Status: certificates.CertificateSigningRequestStatus{
			Conditions:  []certificates.CertificateSigningRequestCondition{{Type: certificates.CertificateApproved}},
			Certificate: []byte("signed-certificate"),
		},
	}, nil
}

func (mock *certificateMock) PrivateKey() []byte {
	keyBytes, err := x509.MarshalECPrivateKey(mock.privateKey)
	Expect(err).ToNot(HaveOccurred())
	return keyBytes
}

type storeSpy struct {
	certs              map[string][]byte
	caCerts            map[string][]byte
	privateKeys        map[string][]byte
	scrapeConfig       []byte
	loadedScrapeConfig []byte
}

func (spy *storeSpy) SaveCert(clusterName string, certData []byte) error {
	if spy.certs == nil {
		spy.certs = map[string][]byte{}
	}
	spy.certs[clusterName] = certData
	return nil
}
func (spy *storeSpy) SaveCA(clusterName string, certData []byte) error {
	if spy.caCerts == nil {
		spy.caCerts = map[string][]byte{}
	}
	spy.caCerts[clusterName] = certData
	return nil
}
func (spy *storeSpy) SavePrivateKey(clusterName string, keyData []byte) error {
	if spy.privateKeys == nil {
		spy.privateKeys = map[string][]byte{}
	}
	spy.privateKeys[clusterName] = keyData
	return nil
}

func (spy *storeSpy) Path() string {
	return ""
}
func (spy *storeSpy) PrivateKeyPath(string) string {
	return "/tmp/scraper/private.key"
}
func (spy *storeSpy) CAPath(clusterName string) string {
	return "/tmp/scraper/" + clusterName + "/ca.pem"
}
func (spy *storeSpy) CertPath(clusterName string) string {
	return "/tmp/scraper/" + clusterName + "/cert.pem"
}

func (spy *storeSpy) SaveScrapeConfig(config []byte) error {
	spy.scrapeConfig = config
	return nil
}
func (spy *storeSpy) LoadScrapeConfig() ([]byte, error) {
	return spy.loadedScrapeConfig, nil
}

var _ = Describe("Cluster Discovery", func() {
	type testContext struct {
		certificateStore  storeSpy
		certificateClient certificateMock
	}

	var setup = func() *testContext {
		privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		Expect(err).To(Not(HaveOccurred()))

		tc := &testContext{
			certificateClient: certificateMock{
				certificateString: "scraperCertData",
				privateKey:        privateKey,
			},
		}
		return tc
	}

	var runScrape = func(tc *testContext) prometheusConfig.Config {
		mockAuth := &mockAuthClient{}
		discovery := cluster_discovery.New(&tc.certificateStore,
			&mockClusterProvider{certClient: &tc.certificateClient},
			mockAuth)
		discovery.UpdateScrapeConfig()

		var expected prometheusConfig.Config

		Expect(yaml.NewDecoder(bytes.NewReader(tc.certificateStore.scrapeConfig)).Decode(&expected)).To(Succeed())
		return expected
	}

	Describe("Start", func() {
		It("runs repeatedly", func() {
			mockAuth := &mockAuthClient{}
			certificateStore := &storeSpy{}
			privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
			Expect(err).To(Not(HaveOccurred()))
			certificateClient := &certificateMock{certificateString: "scraperCertData", privateKey: privateKey}
			discovery := cluster_discovery.New(certificateStore, &mockClusterProvider{certClient: certificateClient}, mockAuth,
				cluster_discovery.WithRefreshInterval(time.Millisecond),
			)

			discovery.Start()

			Eventually(mockAuth.calls.Load).Should(BeNumerically(">", 1))
		})

		It("stops", func() {
			mockAuth := &mockAuthClient{}
			certificateStore := &storeSpy{}
			privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
			Expect(err).To(Not(HaveOccurred()))
			certificateClient := &certificateMock{certificateString: "scraperCertData", privateKey: privateKey}
			discovery := cluster_discovery.New(certificateStore, &mockClusterProvider{certClient: certificateClient}, mockAuth,
				cluster_discovery.WithRefreshInterval(time.Millisecond),
			)

			discovery.Start()
			Eventually(mockAuth.calls.Load).Should(BeNumerically(">", 1))
			discovery.Stop()
			runsAtStop := mockAuth.calls.Load()
			Consistently(mockAuth.calls.Load).Should(BeNumerically("<=", runsAtStop+1))
		})
	})

	Describe("ScrapeConfig for node in a cluster", func() {
		var getJobConfig = func(jobName string, config prometheusConfig.Config) *prometheusConfig.ScrapeConfig {
			Expect(config.ScrapeConfigs).ToNot(BeEmpty())
			for _, scrapeConfig := range config.ScrapeConfigs {
				if scrapeConfig.JobName == jobName {
					return scrapeConfig
				}
			}
			Fail("No scrape config found for name" + jobName)
			return nil
		}

		It("creates a base ScrapeConfig", func() {
			scrapeConfig := getJobConfig("cluster1-kubernetes-nodes", runScrape(setup()))

			sdConfig := scrapeConfig.ServiceDiscoveryConfig.KubernetesSDConfigs[0]
			Expect(sdConfig.Role).To(Equal(kubernetesDiscovery.Role("node")))
			Expect(sdConfig.APIServer.String()).To(Equal("//somehost:12345"))
		})

		It("creates a ScrapeConfig for nodes in a cluster", func() {
			scrapeConfig := getJobConfig("cluster1-kubernetes-nodes", runScrape(setup()))

			Expect(scrapeConfig.RelabelConfigs).To(HaveLen(3))
			Expect(scrapeConfig.RelabelConfigs).To(MatchAllElements(
				func(element interface{}) string {
					return element.(*relabel.Config).TargetLabel
				},
				Elements{
					"": PointTo(MatchFields(IgnoreExtras, Fields{
						"Regex":  Equal(relabel.MustNewRegexp("__meta_kubernetes_node_label_(.+)")),
						"Action": Equal(relabel.LabelMap),
					})),
					"__address__": PointTo(MatchFields(IgnoreExtras, Fields{
						"Replacement": Equal("somehost:12345"),
					})),
					"__metrics_path__": PointTo(MatchFields(IgnoreExtras, Fields{
						"SourceLabels": Equal(model.LabelNames{
							"__meta_kubernetes_node_name",
						}),
						"Regex":       Equal(relabel.MustNewRegexp("(.+)")),
						"Replacement": Equal("/api/v1/nodes/$1/proxy/metrics"),
					})),
				}),
			)
		})

		It("configures TLS", func() {
			clusterName := "cluster1"
			jobName := clusterName + "-kubernetes-nodes"
			tc := setup()
			scrapeConfig := getJobConfig(jobName, runScrape(tc))

			Expect(string(tc.certificateStore.certs["cluster1"])).To(Equal("signed-certificate"))
			Expect(tc.certificateStore.caCerts["cluster1"]).To(Equal([]byte("certdata")))
			Expect(tc.certificateStore.privateKeys["cluster1"]).To(Equal(tc.certificateClient.PrivateKey()))

			tlsConfig := scrapeConfig.HTTPClientConfig.TLSConfig
			Expect(tlsConfig.KeyFile).To(Equal("/tmp/scraper/private.key"))
			Expect(tlsConfig.CAFile).To(Equal("/tmp/scraper/cluster1/ca.pem"))
			Expect(tlsConfig.CertFile).To(Equal("/tmp/scraper/cluster1/cert.pem"))

			tlsConfig = scrapeConfig.ServiceDiscoveryConfig.KubernetesSDConfigs[0].HTTPClientConfig.TLSConfig
			Expect(tlsConfig.KeyFile).To(Equal("/tmp/scraper/private.key"))
			Expect(tlsConfig.CAFile).To(Equal("/tmp/scraper/cluster1/ca.pem"))
			Expect(tlsConfig.CertFile).To(Equal("/tmp/scraper/cluster1/cert.pem"))
		})
	})
})

type mockClusterProvider struct {
	certClient kubernetes.CertificateClient
}

func (m mockClusterProvider) GetClusters(authHeader string) ([]pks.Cluster, error) {
	return []pks.Cluster{
		{
			Name:      "cluster1",
			CaData:    "certdata",
			UserToken: "bearer thingie",
			Host:      "somehost:12345",
			APIClient: m.certClient,
		},
	}, nil
}

type mockAuthClient struct {
	calls atomic.Int32
}

func (m *mockAuthClient) GetAuthHeader() (string, error) {
	m.calls.Inc()
	return "bearer stuff", nil
}

var _ = `---
- job_name: "cluster1"
  metrics_path: "/metrics"
  scheme: "https"
  tls_config:
    ca_file: "/tmp/cluster1_ca.pem"
    cert_file: "/tmp/cluster1_cert.pem"
    key_file: "/tmp/cluster1_cert.key"
    insecure_skip_verify: false
  static_configs:
  - targets:
    - "localhost:$k8sApiPort"
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
    regex: "default;kubernetes;https"
- job_name: "cluster1-kubernetes-nodes"
  kubernetes_sd_configs:
  - role: "node"
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
  - action: "labelmap"
    regex: "__meta_kubernetes_node_label_(.+)"
  - target_label: "__address__"
    replacement: "localhost:$k8sApiPort"
  - source_labels:
    - "__meta_kubernetes_node_name"
    regex: "(.+)"
    target_label: "__metrics_path__"
    replacement: "/api/v1/nodes/$1/proxy/metrics"
- job_name: "cluster1-kubernetes-cadvisor"
  kubernetes_sd_configs:
  - role: "node"
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
  - action: "labelmap"
    regex: "__meta_kubernetes_node_label_(.+)"
  - target_label: "__address__"
    replacement: "localhost:$k8sApiPort"
  - source_labels:
    - "__meta_kubernetes_node_name"
    regex: "(.+)"
    target_label: "__metrics_path__"
    replacement: "/api/v1/nodes/$1/proxy/metrics/cadvisor"
- job_name: "cluster1-kube-state-metrics"
  kubernetes_sd_configs:
  - role: "pod"
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
    - "__meta_kubernetes_pod_container_name"
    - "__meta_kubernetes_pod_container_port_name"
    action: "keep"
    regex: "(pks-system;kube-state-metrics;http-metrics|telemetry)"
  - target_label: "__address__"
    replacement: "localhost:$k8sApiPort"
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
- job_name: "cluster1-kubernetes-coredns"
  kubernetes_sd_configs:
  - role: "pod"
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
    - "__meta_kubernetes_pod_container_name"
    action: "keep"
    regex: "coredns"
  - target_label: "__address__"
    replacement: "localhost:$k8sApiPort"
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
    target_label: "instance"`
