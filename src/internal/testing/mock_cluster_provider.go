package testing

import (
	"github.com/cloudfoundry/metric-store-release/src/internal/cluster-discovery/kubernetes"
	"github.com/cloudfoundry/metric-store-release/src/internal/cluster-discovery/pks"
)

type MockClusterProvider struct {
	CertClient kubernetes.CertificateSigningRequestClient
}

func (m MockClusterProvider) GetClusters(authHeader string) ([]pks.Cluster, error) {
	return []pks.Cluster{
		{
			Name:      "cluster1",
			CaData:    "certdata",
			UserToken: "bearer thingie",
			Addr:      "somehost:12345",
			APIClient: m.CertClient,
		},
	}, nil
}
