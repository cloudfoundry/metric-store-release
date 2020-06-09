package metric_store_test

import (
	"log"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestMetricStoreAcceptance(t *testing.T) {
	log.SetOutput(GinkgoWriter)

	RegisterFailHandler(Fail)
	RunSpecs(t, "Metric Store Acceptance Suite")
}
