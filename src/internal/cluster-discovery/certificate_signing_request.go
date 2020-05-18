package cluster_discovery

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"github.com/cloudfoundry/metric-store-release/src/internal/cluster-discovery/kubernetes"
	certificates "k8s.io/api/certificates/v1beta1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"time"
)

type CertificateSigningRequest struct {
	kubernetesCSRClient kubernetes.CertificateSigningRequestClient
	timeout             time.Duration
	retryInterval       time.Duration
}

type CSROption func(csr *CertificateSigningRequest)

func NewCertificateSigningRequest(client kubernetes.CertificateSigningRequestClient, options ...CSROption) *CertificateSigningRequest {
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

// TODO if we have a large number of clusters, does blocking on certificate
//      approval make discovery unacceptably slow?
func (csr *CertificateSigningRequest) RequestScraperCertificate() ([]byte, []byte, error) {
	//TODO DELETE CSR so that it doesn't error
	//you can still log into the cluster with the generated cert
	//after the csr is deleted

	csrPEM, privateKey, err := csr.kubernetesCSRClient.Generate()
	if err != nil {
		return nil, nil, err
	}

	signingRequest, err := csr.kubernetesCSRClient.Submit(csrPEM, privateKey)
	if err != nil {
		return nil, nil, err
	}

	signingRequest, err = csr.kubernetesCSRClient.Approve(signingRequest)
	if err != nil {
		return nil, nil, err
	}

	signingRequest, err = csr.getUntilApproved()
	if err != nil {
		return nil, nil, err
	}

	marshalledKey, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to encode private key: %v", err)
	}

	keyBlock := &pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: marshalledKey,
	}
	keyBytes := pem.EncodeToMemory(keyBlock)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to encode private key: %v", err)
	}

	return signingRequest.Status.Certificate, keyBytes, nil
}

func (csr *CertificateSigningRequest) getUntilApproved() (*certificates.CertificateSigningRequest, error) {
	ctx, _ := context.WithTimeout(context.Background(), csr.timeout)
	for {
		select {
		case <-ctx.Done():
			return nil, errors.New("retrieving certificate signing request timed out")
		default:
			signingRequest, err := csr.kubernetesCSRClient.Get(v1.GetOptions{})
			if err != nil {
				return nil, err
			}
			if signingRequest.Status.Certificate != nil {
				return signingRequest, nil
			}
			time.Sleep(csr.retryInterval)
		}
	}
}
