package cluster_discovery

import (
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"github.com/cloudfoundry/metric-store-release/src/internal/cluster-discovery/kubernetes"
	certificates "k8s.io/api/certificates/v1beta1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"time"
)

type CertificateSigningRequest struct {
	kubernetesCSRClient kubernetes.CertificateClient
	timeout             time.Duration
	retryInterval       time.Duration
}

type CSROption func(csr *CertificateSigningRequest)

func NewCertificateSigningRequest(client kubernetes.CertificateClient, options ...CSROption) *CertificateSigningRequest {
	csr := &CertificateSigningRequest{kubernetesCSRClient: client, timeout: time.Minute, retryInterval: time.Second}

	for _, o := range options {
		o(csr)
	}
	return csr
}

func WithCertificateSigningRequestTimeout(t time.Duration) CSROption {
	return func(csr *CertificateSigningRequest) {
		csr.timeout = t
	}
}

func WithCertificateSigningRequestRetryInterval(t time.Duration) CSROption {
	return func(csr *CertificateSigningRequest) {
		csr.retryInterval = t
	}
}

func (csr *CertificateSigningRequest) RequestScraperCertificate() ([]byte, []byte, error) {
	//TODO DELETE CSR so that it doesn't error
	//you can still log into the cluster with the generated cert
	//after the csr is deleted

	csrPEM, privateKey, err := csr.kubernetesCSRClient.GenerateCSR()
	if err != nil {
		return nil, nil, err
	}

	signingRequest, err := csr.kubernetesCSRClient.RequestCertificate(csrPEM, privateKey)
	panicIfError(err)

	signingRequest, err = csr.kubernetesCSRClient.UpdateApproval(signingRequest)
	panicIfError(err)

	signingRequest, err = csr.get()
	panicIfError(err)
	keyBytes, err := x509.MarshalECPrivateKey(privateKey)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to encode private key: %v", err)
	}

	return signingRequest.Status.Certificate, keyBytes, nil
}

func (csr *CertificateSigningRequest) get() (*certificates.CertificateSigningRequest, error) {
	ctx, _ := context.WithTimeout(context.Background(), csr.timeout)
	for {
		select {
		case <-ctx.Done():
			return nil, errors.New("retrieving certificate signing request timed out")
		default:
			signingRequest, err := csr.kubernetesCSRClient.Get(v1.GetOptions{})
			if err != nil {
				//return nil, err
				panicIfError(err)
			}
			if signingRequest.Status.Certificate != nil {
				return signingRequest, nil
			}
			time.Sleep(csr.retryInterval)
		}
	}
}
