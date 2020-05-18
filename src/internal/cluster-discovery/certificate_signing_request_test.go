package cluster_discovery_test

import (
	"github.com/cloudfoundry/metric-store-release/src/internal/cluster-discovery"
	"github.com/cloudfoundry/metric-store-release/src/internal/testing"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"time"
)

var _ = Describe("CertificateSigningRequest", func() {
	Describe("Generates a CSR", func() {
		It("returns the certificate and private key", func() {
			mockClient := testing.NewMockCSRClient()
			csr := cluster_discovery.NewCertificateSigningRequest(mockClient,
				cluster_discovery.WithCertificateSigningRequestTimeout(time.Millisecond))
			cert, key, err := csr.RequestScraperCertificate()

			Expect(err).ToNot(HaveOccurred())
			Expect(key).To(Equal(mockClient.PrivateKeyInPEMForm()))
			Expect(string(cert)).To(Equal("signed-certificate"))
		})

		Describe("bubble up errors on external calls", func() {
			It("handles certificate request error", func() {
				mockClient := testing.NewMockCSRClient()
				mockClient.NextCreateCSRIsError = true
				csr := cluster_discovery.NewCertificateSigningRequest(mockClient,
					cluster_discovery.WithCertificateSigningRequestTimeout(time.Millisecond))
				_, _, err := csr.RequestScraperCertificate()

				Expect(err).To(HaveOccurred())
			})

			It("handles certificate approval error", func() {
				mockClient := testing.NewMockCSRClient()
				mockClient.NextUpdateApprovalIsError = true
				csr := cluster_discovery.NewCertificateSigningRequest(mockClient,
					cluster_discovery.WithCertificateSigningRequestTimeout(time.Millisecond))
				_, _, err := csr.RequestScraperCertificate()

				Expect(err).To(HaveOccurred())
			})

			It("handles get approved signing request error", func() {
				mockClient := testing.NewMockCSRClient()
				mockClient.NextGetApprovalIsError = true
				csr := cluster_discovery.NewCertificateSigningRequest(mockClient,
					cluster_discovery.WithCertificateSigningRequestTimeout(time.Millisecond))
				_, _, err := csr.RequestScraperCertificate()

				Expect(err).To(HaveOccurred())
			})
		})
	})

})
