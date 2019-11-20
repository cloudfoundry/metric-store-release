package handoff_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestHandoff(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Handoff Suite")
}
