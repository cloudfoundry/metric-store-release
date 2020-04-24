package kubernetes

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
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

type CertificateClient interface {
	GenerateCSR() (csrPEM []byte, key *ecdsa.PrivateKey, err error)
	RequestCertificate(csrData []byte, privateKey interface{}) (req *certificates.CertificateSigningRequest, err error)
	UpdateApproval(certificateSigningRequest *certificates.CertificateSigningRequest) (result *certificates.CertificateSigningRequest, err error)
	Get(options v1.GetOptions) (*certificates.CertificateSigningRequest, error)
}

type csrClient struct {
	api                   typedCertificates.CertificateSigningRequestInterface
	metricStoreCommonName string
}

func NewCSRClient(client typedCertificates.CertificateSigningRequestInterface) *csrClient {
	return &csrClient{
		api:                   client,
		metricStoreCommonName: "metricstore" + uuid.New().String(), // TODO this is pretty terrible, fix this somehow
	}
}

func (client *csrClient) RequestCertificate(csrData []byte, privateKey interface{}) (req *certificates.CertificateSigningRequest, err error) {
	usages := []certificates.KeyUsage{certificates.UsageClientAuth}
	return csr.RequestCertificate(client.api, csrData, client.metricStoreCommonName, usages, privateKey)
}

func (client *csrClient) UpdateApproval(certificateSigningRequest *certificates.CertificateSigningRequest) (result *certificates.CertificateSigningRequest, err error) {
	certificateSigningRequest.Status.Conditions = append(certificateSigningRequest.Status.Conditions, certificates.CertificateSigningRequestCondition{Type: certificates.CertificateApproved})
	return client.api.UpdateApproval(certificateSigningRequest)
}

func (client *csrClient) Get(options v1.GetOptions) (*certificates.CertificateSigningRequest, error) {
	return client.api.Get(client.metricStoreCommonName, options)
}

func (client *csrClient) GenerateCSR() ([]byte, *ecdsa.PrivateKey, error) {
	template := &x509.CertificateRequest{
		// TODO make sure we don't need these other fields
		Subject: pkix.Name{
			CommonName: client.metricStoreCommonName,
			//Organization: []string{"metric-store"},
		},
		//EmailAddresses: []string{"metric-store@vmware.com"},
	}
	privateKey, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to generate a new private key: %v", err)
	}

	csrPEM, err := cert.MakeCSRFromTemplate(privateKey, template)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to create a csr from the private key: %v", err)
	}
	return csrPEM, privateKey, nil
}
