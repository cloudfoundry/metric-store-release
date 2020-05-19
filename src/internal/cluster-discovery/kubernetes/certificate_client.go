package kubernetes

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
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
}

type csrClient struct {
	api                   typedCertificates.CertificateSigningRequestInterface
	metricStoreCommonName string
	name                  string
}

func NewCSRClient(client typedCertificates.CertificateSigningRequestInterface) *csrClient {
	return &csrClient{
		api:                   client,
		metricStoreCommonName: "metricstore",
		name:                  "metricstore" + uuid.New().String(), // TODO this is pretty terrible, fix this somehow
	}
}

func (client *csrClient) Submit(csrData []byte, privateKey interface{}) (req *certificates.CertificateSigningRequest, err error) {
	usages := []certificates.KeyUsage{certificates.UsageClientAuth}
	return csr.RequestCertificate(client.api, csrData, client.name, usages, privateKey)
}

func (client *csrClient) Approve(certificateSigningRequest *certificates.CertificateSigningRequest) (result *certificates.CertificateSigningRequest, err error) {
	certificateSigningRequest.Status.Conditions = append(certificateSigningRequest.Status.Conditions, certificates.CertificateSigningRequestCondition{Type: certificates.CertificateApproved})
	return client.api.UpdateApproval(certificateSigningRequest)
}

func (client *csrClient) Get(options v1.GetOptions) (*certificates.CertificateSigningRequest, error) {
	return client.api.Get(client.name, options)
}

func (client *csrClient) Delete() error {
	return client.api.Delete(client.name, &v1.DeleteOptions{})
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
