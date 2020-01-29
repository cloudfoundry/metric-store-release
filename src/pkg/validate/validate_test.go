package validate_test

import (
	. "github.com/cloudfoundry/metric-store-release/src/pkg/validate"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Validate Package", func() {
	Describe("AlertmanagerUrl()", func() {
		It("returns nil for valid urls", func() {
			Expect(AlertmanagerUrl("example.com:80")).To(Succeed())
			Expect(AlertmanagerUrl("127.0.0.1:1234")).To(Succeed())
			Expect(AlertmanagerUrl("example.com")).To(Succeed())
			Expect(AlertmanagerUrl("127.0.0.1")).To(Succeed())
			Expect(AlertmanagerUrl("")).To(Succeed())
		})

		It("returns an error for invalid urls", func() {
			Expect(AlertmanagerUrl("http://127.0.0.1:1234")).NotTo(Succeed())
			Expect(AlertmanagerUrl("foobar....")).NotTo(Succeed())
			Expect(AlertmanagerUrl(":1234")).NotTo(Succeed())
			Expect(AlertmanagerUrl("127.0.0.1:1234/foobar")).NotTo(Succeed())
		})
	})
})
