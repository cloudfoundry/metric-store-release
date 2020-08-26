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
		now := time.Now()
		Expect(delay.Ready(now)).To(BeFalse())

		now = now.Add(10 * time.Millisecond)
		Expect(delay.Ready(now)).To(BeTrue())
		now = now.Add(20 * time.Millisecond)
		Expect(delay.Ready(now)).To(BeTrue())
		now = now.Add(40 * time.Millisecond)
		Expect(delay.Ready(now)).To(BeTrue())
		now = now.Add(80 * time.Millisecond)
		Expect(delay.Ready(now)).To(BeTrue())
	})

	It("does not race when multi-threaded", func() {
		delay := NewExponentialDelay(&Config{})
		for i := 0; i < 100; i++ {
			go delay.Reset(time.Now())
		}
		for i := 0; i < 100; i++ {
			go delay.Ready(time.Now())
		}
	})

	It("caps duration at max", func() {
		config := &Config{
			BaseDelay:  10 * time.Millisecond,
			Multiplier: 10,
			MaxDelay:   10 * time.Millisecond,
		}

		delay := NewExponentialDelay(config)
		now := time.Now()

		now = now.Add(10 * time.Millisecond)
		Expect(delay.Ready(now)).To(BeTrue())
		now = now.Add(10 * time.Millisecond)
		Expect(delay.Ready(now)).To(BeTrue())
		now = now.Add(10 * time.Millisecond)
		Expect(delay.Ready(now)).To(BeTrue())
		now = now.Add(10 * time.Millisecond)
		Expect(delay.Ready(now)).To(BeTrue())
	})

	It("resets", func() {
		config := &Config{
			BaseDelay:  10 * time.Millisecond,
			Multiplier: 2,
		}

		delay := NewExponentialDelay(config)
		now := time.Now()

		now = now.Add(10 * time.Millisecond)
		Expect(delay.Ready(now)).To(BeTrue())
		now = now.Add(20 * time.Millisecond)
		Expect(delay.Ready(now)).To(BeTrue())

		delay.Reset(now)
		now = now.Add(10 * time.Millisecond)
		Expect(delay.Ready(now)).To(BeTrue())

		// i have the codecraft meetup until lunch
		// if you're interested, https://VMware.zoom.us/j/97902403110?pwd=Lzl6RmZWejd6Z1FYU2JjZXhvbTlRQT09
	})
})
