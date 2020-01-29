package auth_test

import (
	"errors"

	"github.com/cloudfoundry/metric-store-release/src/pkg/logger"
	"github.com/cloudfoundry/metric-store-release/src/pkg/auth"

	"github.com/cloudfoundry/metric-store-release/src/internal/testing"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("DefaultAccessLogger", func() {
	var (
		writer       *spyWriter
		accessLogger *auth.DefaultAccessLogger
	)

	BeforeEach(func() {
		writer = &spyWriter{}
		accessLogger = auth.NewAccessLogger(writer, logger.NewTestLogger(GinkgoWriter))
	})

	It("logs Access", func() {
		req, err := testing.NewServerRequest("GET", "some.url.com/foo", nil)
		Expect(err).ToNot(HaveOccurred())

		Expect(accessLogger.LogAccess(req, "1.1.1.1", "1")).To(Succeed())
		prefix := "CEF:0|cloud_foundry|metric_store|1.0|GET some.url.com/foo|GET some.url.com/foo|0|"
		Expect(writer.message).To(HavePrefix(prefix))
	})

	It("includes details about the access", func() {
		req, err := testing.NewServerRequest("GET", "http://some.url.com/foo", nil)
		Expect(err).ToNot(HaveOccurred())
		req.RemoteAddr = "127.0.0.1:4567"

		Expect(accessLogger.LogAccess(req, "1.1.1.1", "1")).To(Succeed())
		Expect(writer.message).To(ContainSubstring("src=127.0.0.1 spt=4567"))
	})

	It("uses X-Forwarded-For if it exists", func() {
		req, err := testing.NewServerRequest("GET", "http://some.url.com/foo", nil)
		Expect(err).ToNot(HaveOccurred())
		req.RemoteAddr = "127.0.0.1:4567"
		req.Header.Set("X-Forwarded-For", "50.60.70.80:1234")

		Expect(accessLogger.LogAccess(req, "1.1.1.1", "1")).To(Succeed())
		Expect(writer.message).To(ContainSubstring("src=50.60.70.80 spt=1234"))
	})

	It("writes multiple log lines", func() {
		req, err := testing.NewServerRequest("GET", "http://some.url.com/foo", nil)
		Expect(err).ToNot(HaveOccurred())
		req.RemoteAddr = "127.0.0.1:4567"

		Expect(accessLogger.LogAccess(req, "1.1.1.1", "1")).To(Succeed())
		expected := "src=127.0.0.1 spt=4567"
		Expect(writer.message).To(ContainSubstring(expected))
		Expect(writer.message).To(HaveSuffix("\n"))

		req.Header.Set("X-Forwarded-For", "50.60.70.80:1234")

		Expect(accessLogger.LogAccess(req, "1.1.1.1", "1")).To(Succeed())
		expected = "src=50.60.70.80 spt=1234"
		Expect(writer.message).To(ContainSubstring(expected))
		Expect(writer.message).To(HaveSuffix("\n"))
	})

	It("returns an error", func() {
		writer = &spyWriter{}
		writer.err = errors.New("boom")
		accessLogger = auth.NewAccessLogger(writer, logger.NewTestLogger(GinkgoWriter))

		req, err := testing.NewServerRequest("GET", "http://some.url.com/foo", nil)
		Expect(err).ToNot(HaveOccurred())
		req.RemoteAddr = "127.0.0.1:4567"
		Expect(accessLogger.LogAccess(req, "1.1.1.1", "1")).ToNot(Succeed())
	})
})

type spyWriter struct {
	message []byte
	err     error
}

func (s *spyWriter) Write(message []byte) (sent int, err error) {
	s.message = message
	return 0, s.err
}
