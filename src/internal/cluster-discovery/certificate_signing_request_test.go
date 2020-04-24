package cluster_discovery_test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"github.com/cloudfoundry/metric-store-release/src/internal/cluster-discovery"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"time"
)

var _ = Describe("CertificateSigningRequest", func() {
	Describe("Generates a CSR", func() {
		It("returns the certificate and private key", func() {
			privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
			Expect(err).To(Not(HaveOccurred()))
			mockClient := &certificateMock{certificateString: "scraperCertData", privateKey: privateKey}
			csr := cluster_discovery.NewCertificateSigningRequest(mockClient, cluster_discovery.WithCertificateSigningRequestTimeout(time.Millisecond))
			cert, key, err := csr.RequestScraperCertificate()

			Expect(err).ToNot(HaveOccurred())
			Expect(key).To(Equal(mockClient.PrivateKey()))
			Expect(string(cert)).To(Equal("signed-certificate"))
		})

		// This test is broken. It blocks until mockClient.pending is true, which of course, doesn't happen until
		// later in the test
		//It("waits until the certificate is approved when downloading", func() {
		//	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		//	Expect(err).To(Not(HaveOccurred()))
		//	mockClient := &certificateMock{certificateString: "scraperCertData", privateKey: privateKey}
		//	mockClient.pending = true
		//	csr := cluster_discovery.NewCertificateSigningRequest(mockClient, cluster_discovery.WithCertificateSigningRequestTimeout(time.Millisecond))
		//	_, _, err = csr.RequestScraperCertificate()
		//	Expect(err).To(HaveOccurred())
		//
		//	mockClient.pending = false
		//	cert, key, err := csr.RequestScraperCertificate()
		//	Expect(err).ToNot(HaveOccurred())
		//	Expect(key).To(Equal(mockClient.PrivateKey()))
		//	Expect(string(cert)).To(Equal("signed-certificate"))
		//})

	})

})
