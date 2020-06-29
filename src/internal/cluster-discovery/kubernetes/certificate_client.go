package kubernetes

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"net/http"
	"time"

	sharedtls "github.com/cloudfoundry/metric-store-release/src/internal/tls"
	"github.com/cloudfoundry/metric-store-release/src/pkg/logger"
	"github.com/google/uuid"
	certificates "k8s.io/api/certificates/v1beta1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	typedCertificates "k8s.io/client-go/kubernetes/typed/certificates/v1beta1"
	"k8s.io/client-go/util/cert"
	"k8s.io/client-go/util/certificate/csr"
)

type CertificateSigningRequestClient interface {
	Generate() (csrPEM []byte, key *rsa.PrivateKey, err error)
	Submit(csrData []byte, privateKey interface{}) (req *certificates.CertificateSigningRequest, err error)
	Approve(certificateSigningRequest *certificates.CertificateSigningRequest) (result *certificates.CertificateSigningRequest, err error)
	Get(options v1.GetOptions) (*certificates.CertificateSigningRequest, error)
	Delete() error
	TestConnectivity(url string, caCertPath, certPath, keyPath, serverName string) bool
}

type csrClient struct {
	api                   typedCertificates.CertificateSigningRequestInterface
	metricStoreCommonName string
	name                  string
	timeout               time.Duration
	log                   *logger.Logger
}

func NewCSRClient(client typedCertificates.CertificateSigningRequestInterface, options ...WithOption) *csrClient {
	csrClient := &csrClient{
		api:                   client,
		metricStoreCommonName: "metricstore",
		name:                  uniqueCSRName(),
		timeout:               time.Minute,
		log:                   logger.NewNop(),
	}

	for _, option := range options {
		option(csrClient)
	}
	return csrClient
}

type WithOption func(client *csrClient)

func WithLogger(log *logger.Logger) WithOption {
	return func(client *csrClient) {
		client.log = log
	}
}

func WithTimeout(t time.Duration) WithOption {
	return func(client *csrClient) {
		client.timeout = t
	}
}

func uniqueCSRName() string {
	return "metricstore-" + uuid.New().String()
}

func (client *csrClient) Submit(csrData []byte, privateKey interface{}) (req *certificates.CertificateSigningRequest, err error) {
	usages := []certificates.KeyUsage{certificates.UsageClientAuth}
	return csr.RequestCertificate(client.api, csrData, client.name, "metric-store", usages, privateKey)
}

func (client *csrClient) Approve(certificateSigningRequest *certificates.CertificateSigningRequest) (result *certificates.CertificateSigningRequest, err error) {
	certificateSigningRequest.Status.Conditions = append(certificateSigningRequest.Status.Conditions, certificates.CertificateSigningRequestCondition{Type: certificates.CertificateApproved})

	ctx, _ := context.WithTimeout(context.Background(), client.timeout)
	return client.api.UpdateApproval(ctx, certificateSigningRequest, v1.UpdateOptions{})
}

func (client *csrClient) Get(options v1.GetOptions) (*certificates.CertificateSigningRequest, error) {
	ctx, _ := context.WithTimeout(context.Background(), client.timeout)
	return client.api.Get(ctx, client.name, options)
}

func (client *csrClient) Delete() error {
	ctx, _ := context.WithTimeout(context.Background(), client.timeout)
	return client.api.Delete(ctx, client.name, v1.DeleteOptions{})
}

func (client *csrClient) Generate() ([]byte, *rsa.PrivateKey, error) {
	template := &x509.CertificateRequest{
		// TODO make sure we don't need these other fields
		Subject: pkix.Name{
			CommonName: client.metricStoreCommonName,
			//Organization: []string{"metric-store"},
		},
		//EmailAddresses: []string{"metric-store@vmware.com"},
	}
	privateKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to generate a new private key: %v", err)
	}

	csrPEM, err := cert.MakeCSRFromTemplate(privateKey, template)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to create a csr from the private key: %v", err)
	}
	return csrPEM, privateKey, nil
}

func (client *csrClient) TestConnectivity(url string, caCertPath, certPath, keyPath, serverName string) bool {
	tlsConfig, err := sharedtls.NewMutualTLSClientConfig(caCertPath, certPath, keyPath, serverName)
	if err != nil {
		client.log.Error("creating tls config while testing existing scrape config for "+url, err)
		return false
	}

	httpClient := &http.Client{
		Timeout:   5 * time.Second, //TODO make this configurable?
		Transport: &http.Transport{TLSClientConfig: tlsConfig},
	}

	resp, err := httpClient.Get(url)

	if err != nil {
		client.log.Error("making request while testing existing scrape config for "+url, err)
		return false
	}
	return resp.StatusCode == http.StatusOK
}
