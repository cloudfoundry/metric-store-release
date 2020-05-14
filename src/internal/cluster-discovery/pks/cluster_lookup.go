package pks

import (
	"crypto/tls"
	"encoding/base64"
	"fmt"
	cd_kubernetes "github.com/cloudfoundry/metric-store-release/src/internal/cluster-discovery/kubernetes"
	"github.com/cloudfoundry/metric-store-release/src/pkg/logger"
	"net"
	"net/http"
	"net/url"

	"github.com/cloudfoundry/metric-store-release/src/pkg/pks"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func NewClusterLookup(addr string, tlsConfig *tls.Config, log *logger.Logger) *ClusterLookup {
	client := pks.NewClient(addr, &http.Client{Transport: &http.Transport{TLSClientConfig: tlsConfig}}, log)
	return &ClusterLookup{
		pksClient: client,
		log:       log,
	}
}

type Client interface {
	GetClusters(authorization string) ([]string, error)
	GetCredentials(clusterName string, authorization string) (*pks.Credentials, error)
}

type ClusterLookup struct {
	pksClient Client
	log       *logger.Logger
}

type Cluster struct {
	Name      string
	CaData    string
	UserToken string
	Addr      string
	APIClient cd_kubernetes.CertificateSigningRequestClient
}

func (lookup *ClusterLookup) GetClusters(authHeader string) ([]Cluster, error) {
	pksClusters, err := lookup.pksClient.GetClusters(authHeader)
	if err != nil {
		return nil, err
	}

	lookup.log.Debug("clusters", logger.String("json", fmt.Sprintf("%#v", pksClusters)))

	var clusters []Cluster
	for _, clusterName := range pksClusters {
		credentials, err := lookup.pksClient.GetCredentials(clusterName, authHeader)
		if err != nil {
			lookup.log.Error("Could not retrieve credentials for cluster "+clusterName+". skipping.", err)
			continue
		}

		lookup.log.Debug("credentials", logger.String("json", fmt.Sprintf("%#v", credentials)))

		certificateAuthority, err := base64.StdEncoding.DecodeString(credentials.CaData)
		if err != nil {
			lookup.log.Error("Could not decode CA for cluster "+clusterName+". skipping.", err)
			continue
		}

		u, err := url.Parse(credentials.Server)
		if err != nil {
			lookup.log.Error("Could not parse the URL for cluster "+clusterName+". skipping.", err)
			continue
		}

		hostname, _, err := net.SplitHostPort(u.Host)
		if err != nil {
			lookup.log.Error("Could not parse the host and port for cluster "+clusterName+". skipping.", err)
			continue
		}

		tlsClientConfig := rest.TLSClientConfig{
			CAData:     certificateAuthority,
			ServerName: hostname,
		}

		clientSet, err := kubernetes.NewForConfig(&rest.Config{
			Host:            u.Host,
			BearerToken:     credentials.UserToken,
			TLSClientConfig: tlsClientConfig,
		})
		if err != nil {
			lookup.log.Error("Could not build kubernetes config for cluster "+clusterName+". skipping.", err)
			continue
		}
		apiClient := clientSet.CertificatesV1beta1().CertificateSigningRequests()

		clusters = append(clusters, Cluster{
			Name:      clusterName,
			CaData:    credentials.CaData,
			UserToken: credentials.UserToken,
			Addr:      u.Host,
			APIClient: cd_kubernetes.NewCSRClient(apiClient),
		})
	}
	return clusters, nil
}
