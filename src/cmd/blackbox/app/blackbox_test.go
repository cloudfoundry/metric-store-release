package app_test

import (
	"io/ioutil"
	"net/http"

	"github.com/cloudfoundry/metric-store-release/src/cmd/blackbox/app"
	"github.com/cloudfoundry/metric-store-release/src/internal/logger"
	"github.com/cloudfoundry/metric-store-release/src/internal/testing"
	"github.com/cloudfoundry/metric-store-release/src/pkg/tls"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Blackbox App", func() {
	var (
		blackbox *app.BlackboxApp
	)

	BeforeEach(func() {
		blackbox = app.NewBlackboxApp(&app.Config{
			TLS: tls.TLS{
				CAPath:   testing.Cert("metric-store-ca.crt"),
				CertPath: testing.Cert("metric-store.crt"),
				KeyPath:  testing.Cert("metric-store.key"),
			},
		}, logger.NewTestLogger())
		go blackbox.Run()
		Eventually(blackbox.DebugAddr).ShouldNot(BeEmpty())
	})

	AfterEach(func() {
		defer blackbox.Stop()
	})

	It("serves metrics on a metrics endpoint", func() {
		var body string
		fn := func() string {
			resp, err := http.Get("http://" + blackbox.DebugAddr() + "/metrics")
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
