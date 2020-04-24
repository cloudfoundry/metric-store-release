package kubernetes_test

import (
	"github.com/cloudfoundry/metric-store-release/src/internal/cluster-discovery"
	"github.com/cloudfoundry/metric-store-release/src/internal/cluster-discovery/pks"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("kubernetes certificate client", func() {
	XIt("does something", func() {
		client := cluster_discovery.NewCertificateSigningRequest(pks.Cluster{}.APIClient)

		cert, _, err := client.RequestScraperCertificate()
		Expect(err).NotTo(HaveOccurred())

		Expect(cert).ToNot(BeNil())
	})
})