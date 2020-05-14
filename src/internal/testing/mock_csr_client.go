package testing

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"fmt"
	"go.uber.org/atomic"
	"k8s.io/api/certificates/v1beta1"
	"k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/onsi/gomega"
)

type MockCSRClient struct {
	GeneratedCSRs atomic.Int32
	Key           *ecdsa.PrivateKey

	NextCreateCSRIsError      bool
	NextUpdateApprovalIsError bool
	NextGetApprovalIsError    bool
}

func NewMockCSRClient() *MockCSRClient {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	Expect(err).To(Not(HaveOccurred()))
	mockClient := &MockCSRClient{
		Key: privateKey,
	}
	return mockClient
}

func (mock *MockCSRClient) Generate() (csrPEM []byte, key *ecdsa.PrivateKey, err error) {
	mock.GeneratedCSRs.Inc()
	return []byte{}, mock.Key, nil
}

func (mock *MockCSRClient) Submit(csrData []byte, privateKey interface{}) (req *v1beta1.CertificateSigningRequest, err error) {
	if mock.NextCreateCSRIsError {
		return nil, fmt.Errorf("Server Unavailable")
	}
	return nil, nil
}

func (mock *MockCSRClient) Approve(_ *v1beta1.CertificateSigningRequest) (result *v1beta1.CertificateSigningRequest, err error) {
	if mock.NextUpdateApprovalIsError {
		return nil, fmt.Errorf("Server Unavailable")
	}
	return nil, nil
}

func (mock *MockCSRClient) Get(options v1.GetOptions) (*v1beta1.CertificateSigningRequest, error) {
	if mock.NextGetApprovalIsError {
		return nil, fmt.Errorf("Server Unavailable")
	}

	return &v1beta1.CertificateSigningRequest{
		Status: v1beta1.CertificateSigningRequestStatus{
			Conditions:  []v1beta1.CertificateSigningRequestCondition{{Type: v1beta1.CertificateApproved}},
			Certificate: []byte("signed-certificate"),
		},
	}, nil
}

func (mock *MockCSRClient) PrivateKey() []byte {
	keyBytes, err := x509.MarshalECPrivateKey(mock.Key)
	Expect(err).ToNot(HaveOccurred())
	return keyBytes
}
