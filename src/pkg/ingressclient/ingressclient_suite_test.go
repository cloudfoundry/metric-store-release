package ingressclient_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestIngressClient(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "IngressClient Suite")
}
