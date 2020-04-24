package pks

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/cloudfoundry/metric-store-release/src/pkg/logger"
	"go.uber.org/zap"
	"io/ioutil"
	"net/http"
)

// Client handles interaction with the PKS API
type Client struct {
	url        string
	httpClient *http.Client
	log        *logger.Logger
}

func NewClient(addr string, httpClient *http.Client, log *logger.Logger) *Client {
	//TODO use an interface for HTTPClient
	return &Client{
		url:        addr,
		httpClient: httpClient,
		log:        log,
	}
}

func panicIfError(err error) {
	if err != nil {
		panic(err)
	}
}
func (client *Client) GetClusters(authorization string) ([]string, error) {
	url := fmt.Sprintf("%s/v1/clusters", client.url)
	client.log.Debug("cluster request", logger.String("url", url))
	pksRequest, err := http.NewRequest(http.MethodGet, url, bytes.NewBuffer([]byte{}))
	if err != nil {
		panic(err)
	}
	pksRequest.Header.Add("Authorization", authorization)

	responseBody, err := client.doRequest(pksRequest, http.StatusOK)
	panicIfError(err)

	var pksClusters []pksClustersResponse
	err = json.Unmarshal(responseBody, &pksClusters)
	panicIfError(err)

	var clusterNames []string
	for _, response := range pksClusters {
		clusterNames = append(clusterNames, response.Name)
	}
	return clusterNames, nil
}

func (client *Client) GetCredentials(clusterName string, authorization string) (*pksCredentialsResponse, error) {
	pksRequest, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/v1/clusters/%s/binds", client.url, clusterName), bytes.NewBuffer([]byte{}))
	panicIfError(err)
	pksRequest.Header.Add("Authorization", authorization)
	pksRequest.Header.Add("Content-Type", "application/json")
	pksRequest.Header.Add("Media-Type", "application/json")

	responseBody, err := client.doRequest(pksRequest, http.StatusCreated)
	panicIfError(err)

	pksCredentials := &pksCredentialsResponse{}
	client.log.Debug("Credential Response", zap.ByteString("body", responseBody))
	err = json.Unmarshal(responseBody, pksCredentials)
	panicIfError(err)

	return pksCredentials, nil
}

func (client *Client) doRequest(req *http.Request, expectedStatus int) ([]byte, error) {
	resp, err := client.httpClient.Do(req)
	panicIfError(err)

	if resp.StatusCode != expectedStatus {
		return nil, fmt.Errorf("unexpected response code %d", resp.StatusCode)
	}

	defer resp.Body.Close()
	return ioutil.ReadAll(resp.Body)
}

type pksClustersResponse struct {
	Name string `json:"name"`
}

type pksCredentialsResponse struct {
	Clusters []clusters `json:"clusters"`
	Users    []users    `json:"users"`
}

type clusters struct {
	Name    string  `json:"name"`
	Cluster cluster `json:"cluster"`
}

type cluster struct {
	CertificateAuthorityData string `json:"certificate-authority-data"`
	Server                   string `json:"server"`
}

type users struct {
	Name string `json:"name"`
	User user   `json:"user"`
}

type user struct {
	Token string `json:"token"`
}
