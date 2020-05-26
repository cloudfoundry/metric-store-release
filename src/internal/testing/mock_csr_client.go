package testing

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"go.uber.org/atomic"
	"k8s.io/api/certificates/v1beta1"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"sync"

	. "github.com/onsi/gomega"
)

type MockCSRClient struct {
	sync.Mutex
	GeneratedCSRs atomic.Int32
	DeletedCSRs   atomic.Int32
	Key           *rsa.PrivateKey

	NextCreateCSRIsError      bool
	NextUpdateApprovalIsError bool
	NextGetApprovalIsError    bool
	NextDeleteIsError         bool
	ValidExistingScrapeJobs   bool
	TestConnectivityCalls []TestConnectivityArgs
}

type TestConnectivityArgs struct {
	Url string
	CaCertPath string
	CertPath string
	KeyPath string
	ServerName string
}

func NewMockCSRClient() *MockCSRClient {
	privateKey, err := rsa.GenerateKey(rand.Reader, 256)
	Expect(err).To(Not(HaveOccurred()))
	mockClient := &MockCSRClient{
		Key: privateKey,
	}
	return mockClient
}

func (mock *MockCSRClient) Generate() (csrPEM []byte, key *rsa.PrivateKey, err error) {
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
	mock.GeneratedCSRs.Inc()
	return &v1beta1.CertificateSigningRequest{
		Status: v1beta1.CertificateSigningRequestStatus{
			Conditions:  []v1beta1.CertificateSigningRequestCondition{{Type: v1beta1.CertificateApproved}},
			Certificate: []byte("signed-certificate"),
		},
	}, nil
}

func (mock *MockCSRClient) Delete() error {
	if mock.NextDeleteIsError {
		return fmt.Errorf("Could not delete")
	}
	mock.DeletedCSRs.Inc()
	return nil
}

func (mock *MockCSRClient) TestConnectivity(url string, caCertPath, certPath, keyPath, serverName string ) bool {
	mock.Lock()
	mock.TestConnectivityCalls = append(mock.TestConnectivityCalls, TestConnectivityArgs{
		Url:               url,
		CaCertPath: caCertPath,
		CertPath:          certPath,
		KeyPath:           keyPath,
		ServerName:        serverName,
	})
	mock.Unlock()
	return mock.ValidExistingScrapeJobs
}

func (mock *MockCSRClient) PrivateKey() []byte {
	keyBytes, err := x509.MarshalPKCS8PrivateKey(mock.Key)
	Expect(err).ToNot(HaveOccurred())
	return keyBytes
}

func (mock *MockCSRClient) PrivateKeyInPEMForm() []byte {
	marshalledKey, err := x509.MarshalPKCS8PrivateKey(mock.Key)
	Expect(err).ToNot(HaveOccurred())
	keyBlock := &pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: marshalledKey,
	}
	return pem.EncodeToMemory(keyBlock)
}
