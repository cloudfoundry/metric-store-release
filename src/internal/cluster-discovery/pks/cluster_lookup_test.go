package pks

import (
	"errors"
	"github.com/cloudfoundry/metric-store-release/src/pkg/logger"
	"github.com/cloudfoundry/metric-store-release/src/pkg/pks"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"go.uber.org/atomic"
)

var clusterLookup *ClusterLookup
var client *mockClient

type mockClient struct {
	errorsOnGetClusters                            bool
	errorsOnGetCredentialsClusterName              string
	errorsGetCredentialsReturnsBadCAforClusterName string
	GetCredentialsCallCount                        atomic.Int32
	clusters                                       map[string]*pks.Credentials
}

func NewMockClient(clusters map[string]*pks.Credentials) *mockClient {
	return &mockClient{clusters: clusters}
}

func (m *mockClient) GetClusters(authorization string) ([]string, error) {
	if m.errorsOnGetClusters {
		return nil, errors.New("get clusters error")
	}

	var keys []string
	for key := range m.clusters {
		keys = append(keys, key)
	}
	return keys, nil
}

func (m *mockClient) GetCredentials(clusterName string, authorization string) (*pks.Credentials, error) {
	m.GetCredentialsCallCount.Inc()
	if m.errorsOnGetCredentialsClusterName == clusterName {
		return nil, errors.New("get clusters error")
	}

	return m.clusters[clusterName], nil
}

var _ = Describe("Cluster Lookup", func() {

	BeforeEach(func() {
		caData := "c29tZS1jYQ=="
		cluster1 := &pks.Credentials{
			CaData:    caData,
			UserToken: "some-token",
			Server:    "https://cluster1.com:9090",
		}

		cluster2 := &pks.Credentials{
			CaData:    caData,
			UserToken: "some-token",
			Server:    "https://cluster2.com:9090",
		}
		client = NewMockClient(map[string]*pks.Credentials{"cluster1": cluster1, "cluster2": cluster2})
		clusterLookup = &ClusterLookup{
			pksClient: client,
			log:       logger.NewNop(),
		}
	})

	Describe("Handles Errors", func() {
		It("returns an error when it fails to look up the pks clusters", func() {
			client.errorsOnGetClusters = true
			_, err := clusterLookup.GetClusters("some-header")
			Expect(err).To(HaveOccurred())
		})

		It("ignores a cluster when it fails to get credentials for a cluster", func() {
			client.errorsOnGetCredentialsClusterName = "cluster1"
			clusters, err := clusterLookup.GetClusters("some-header")
			Expect(err).ToNot(HaveOccurred())
			Expect(clusters).To(HaveLen(1))
		})

		It("ignores a cluster when it fails to decode the CA", func() {
			client.clusters["cluster1"].CaData = "not base64 encoded"
			clusters, err := clusterLookup.GetClusters("some-header")
			Expect(err).ToNot(HaveOccurred())
			Expect(clusters).To(HaveLen(1))
		})

		It("ignores a cluster when it fails to parse the URL", func() {
			client.clusters["cluster1"].Server = "not a real url"
			clusters, err := clusterLookup.GetClusters("some-header")
			Expect(err).ToNot(HaveOccurred())
			Expect(clusters).To(HaveLen(1))
		})
	})

})
