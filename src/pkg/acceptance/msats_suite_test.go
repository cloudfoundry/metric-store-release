package acceptance_test

import (
	"testing"

	_ "github.com/cloudfoundry/metric-store-release/src/pkg/acceptance"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestMetricStoreAcceptanceTests(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "MetricStoreAcceptanceTests Suite")
}
