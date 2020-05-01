package cluster_discovery_test

import (
	"github.com/cloudfoundry/metric-store-release/src/internal/cluster-discovery"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"time"
)

var _ = Describe("CertificateSigningRequest", func() {
	Describe("Generates a CSR", func() {
		It("returns the certificate and private key", func() {
			mockClient := newMockCSRClient()
			csr := cluster_discovery.NewCertificateSigningRequest(mockClient,
				cluster_discovery.WithCertificateSigningRequestTimeout(time.Millisecond))
			cert, key, err := csr.RequestScraperCertificate()

			Expect(err).ToNot(HaveOccurred())
			Expect(key).To(Equal(mockClient.PrivateKey()))
			Expect(string(cert)).To(Equal("signed-certificate"))
		})

		Describe("bubble up errors on external calls", func() {
			It("handles certificate request error", func() {
				mockClient := newMockCSRClient()
				mockClient.nextCreateCSRIsError = true
				csr := cluster_discovery.NewCertificateSigningRequest(mockClient,
					cluster_discovery.WithCertificateSigningRequestTimeout(time.Millisecond))
				_, _, err := csr.RequestScraperCertificate()

				Expect(err).To(HaveOccurred())
			})

			It("handles certificate approval error", func() {
				mockClient := newMockCSRClient()
				mockClient.nextUpdateApprovalIsError = true
				csr := cluster_discovery.NewCertificateSigningRequest(mockClient,
					cluster_discovery.WithCertificateSigningRequestTimeout(time.Millisecond))
				_, _, err := csr.RequestScraperCertificate()

				Expect(err).To(HaveOccurred())
			})

			It("handles get approved signing request error", func() {
				mockClient := newMockCSRClient()
				mockClient.nextGetApprovalIsError = true
				csr := cluster_discovery.NewCertificateSigningRequest(mockClient,
					cluster_discovery.WithCertificateSigningRequestTimeout(time.Millisecond))
				_, _, err := csr.RequestScraperCertificate()

				Expect(err).To(HaveOccurred())
			})
		})
	})

})
