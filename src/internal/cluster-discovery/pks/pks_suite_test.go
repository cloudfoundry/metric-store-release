package pks_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestPks(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Pks Suite")
}
