package blackbox_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestBlackbox(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Blackbox Suite")
}
