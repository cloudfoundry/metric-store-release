package scraping_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestScraping(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Scraping Suite")
}
