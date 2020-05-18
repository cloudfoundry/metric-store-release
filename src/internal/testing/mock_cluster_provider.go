package testing

import (
	"github.com/cloudfoundry/metric-store-release/src/internal/cluster-discovery/pks"
)

type MockClusterProvider struct {
	Clusters []pks.Cluster
}

func (m MockClusterProvider) GetClusters(authHeader string) ([]pks.Cluster, error) {
	return m.Clusters, nil
}
