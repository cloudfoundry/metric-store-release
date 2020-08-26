package ticker_test

import (
	"time"

	. "github.com/cloudfoundry/metric-store-release/src/internal/ticker"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("A ticker for exponential backoffs", func() {
	It("default inputs", func() {
		cfg := &Config{}
		cfg.Validate()

		Expect(cfg.BaseDelay).To(Equal(100 * time.Millisecond))
		Expect(cfg.Multiplier).To(Equal(2.0))
		Expect(cfg.MaxDelay).To(Equal(0 * time.Millisecond))
	})

	It("validates inputs", func() {
		Expect(func() {
			cfg := &Config{BaseDelay: -10 * time.Millisecond}
			cfg.Validate()
		}).To(Panic())
		Expect(func() {
			cfg := &Config{Multiplier: -1}
			cfg.Validate()
		}).To(Panic())
		Expect(func() {
			cfg := &Config{MaxDelay: -999 * time.Hour}
			cfg.Validate()
		}).To(Panic())
	})
})
