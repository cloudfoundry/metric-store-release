package app

import (
	"io/ioutil"
	"net/http"

	"github.com/cloudfoundry/metric-store-release/src/internal/blackbox"
	"github.com/cloudfoundry/metric-store-release/src/pkg/logger"
	"github.com/cloudfoundry/metric-store-release/src/internal/testing"
	"github.com/cloudfoundry/metric-store-release/src/pkg/tls"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Blackbox App", func() {
	var (
		bb *BlackboxApp
	)

	BeforeEach(func() {
		bb = NewBlackboxApp(&blackbox.Config{
			TLS: tls.TLS{
				CAPath:   testing.Cert("metric-store-ca.crt"),
				CertPath: testing.Cert("metric-store.crt"),
				KeyPath:  testing.Cert("metric-store.key"),
			},
		}, logger.NewTestLogger(GinkgoWriter))
		go bb.Run()
		Eventually(bb.DebugAddr).ShouldNot(BeEmpty())
	})

	AfterEach(func() {
		defer bb.Stop()
	})

	It("serves metrics on a metrics endpoint", func() {
		var body string
		fn := func() string {
			resp, err := http.Get("http://" + bb.DebugAddr() + "/metrics")
			if err != nil {
				return ""
			}
			defer resp.Body.Close()

			bytes, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				return ""
			}

			body = string(bytes)

			return body
		}
		Eventually(fn).ShouldNot(BeEmpty())
		Expect(body).To(ContainSubstring("blackbox_http_reliability"))
		Expect(body).To(ContainSubstring("go_threads"))
	})
})
