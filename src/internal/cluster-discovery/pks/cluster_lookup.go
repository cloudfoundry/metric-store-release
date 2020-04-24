package pks

import (
	"crypto/tls"
	"encoding/base64"
	"fmt"
	kubernetes2 "github.com/cloudfoundry/metric-store-release/src/internal/cluster-discovery/kubernetes"
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

type ClusterLookup struct {
	pksClient *pks.Client
	log       *logger.Logger
}

type Cluster struct {
	Name      string
	CaData    string
	UserToken string
	Host      string
	APIClient kubernetes2.CertificateClient
}

func (lookup *ClusterLookup) GetClusters(authHeader string) ([]Cluster, error) {
	pksClusters, err := lookup.pksClient.GetClusters(authHeader)
	if err != nil {
		panic(err)
	}

	lookup.log.Debug("clusters", logger.String("json", fmt.Sprintf("%#v", pksClusters)))

	var clusters []Cluster
	for _, clusterName := range pksClusters {
		credentials, err := lookup.pksClient.GetCredentials(clusterName, authHeader)
		if err != nil {
			panic(err)
		}

		lookup.log.Debug("credentials", logger.String("json", fmt.Sprintf("%#v", credentials)))

		caData := credentials.Clusters[0].Cluster.CertificateAuthorityData
		userToken := credentials.Users[0].User.Token
		u, err := url.Parse(credentials.Clusters[0].Cluster.Server)
		if err != nil {
			panic(err)
		}

		certificateAuthority, err := base64.StdEncoding.DecodeString(caData)
		if err != nil {
			panic(err)

		}

		//TODO add test with no port given
		hostname, _, err := net.SplitHostPort(u.Host)
		if err != nil {
			panic(err)

		}

		tlsClientConfig := rest.TLSClientConfig{
			CAData:     certificateAuthority,
			ServerName: hostname,
		}

		clientSet, err := kubernetes.NewForConfig(&rest.Config{
			Host:            u.Host,
			BearerToken:     userToken,
			TLSClientConfig: tlsClientConfig,
		})
		if err != nil {
			panic(err)

		}
		apiClient := clientSet.CertificatesV1beta1().CertificateSigningRequests()

		clusters = append(clusters, Cluster{
			Name:      clusterName,
			CaData:    caData,
			UserToken: userToken,
			Host:      u.Host,
			APIClient: kubernetes2.NewCSRClient(apiClient),
		})
	}
	return clusters, nil
}
