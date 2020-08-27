package ticker_test

import (
	"time"

	. "github.com/cloudfoundry/metric-store-release/src/internal/ticker"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("A delay for exponential backoffs", func() {
	It("validates the config", func() {
		invalidConfig := &Config{BaseDelay: -1}

		Expect(func() {
			NewExponentialDelay(invalidConfig)
		}).To(Panic())
	})

	It("steps exponentially", func() {
		config := &Config{
			BaseDelay:  10 * time.Millisecond,
			Multiplier: 2,
		}

		delay := NewExponentialDelay(config)

		Expect(delay.Step()).To(Equal(10 * time.Millisecond))
		Expect(delay.Step()).To(Equal(20 * time.Millisecond))
		Expect(delay.Step()).To(Equal(40 * time.Millisecond))
		Expect(delay.Step()).To(Equal(80 * time.Millisecond))
	})

	It("does not race when multi-threaded", func() {
		delay := NewExponentialDelay(&Config{})
		for i := 0; i < 100; i++ {
			go delay.Reset()
		}
		for i := 0; i < 100; i++ {
			go delay.Step()
		}
	})

	It("caps duration at max", func() {
		config := &Config{
			BaseDelay:  10 * time.Millisecond,
			Multiplier: 10,
			MaxDelay:   10 * time.Millisecond,
		}

		delay := NewExponentialDelay(config)

		Expect(delay.Step()).To(Equal(10 * time.Millisecond))
		Expect(delay.Step()).To(Equal(10 * time.Millisecond))
		Expect(delay.Step()).To(Equal(10 * time.Millisecond))
		Expect(delay.Step()).To(Equal(10 * time.Millisecond))
	})

	It("resets", func() {
		config := &Config{
			BaseDelay:  10 * time.Millisecond,
			Multiplier: 2,
		}

		delay := NewExponentialDelay(config)

		Expect(delay.Step()).To(Equal(10 * time.Millisecond))
		Expect(delay.Step()).To(Equal(20 * time.Millisecond))

		delay.Reset()
		Expect(delay.Step()).To(Equal(10 * time.Millisecond))
	})
})
