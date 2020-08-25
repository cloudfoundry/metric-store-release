package ticker

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("A ticker for exponential backoffs", func() {
	It("ticks exponentially", func() {
		config := TickerConfig{
			BaseDelay:  10 * time.Millisecond,
			Multiplier: 2,
		}

		delay := config.BaseDelay

		delay = calculateNextDelay(delay, config)
		Expect(delay).To(Equal(20 * time.Millisecond))

		delay = calculateNextDelay(delay, config)
		Expect(delay).To(Equal(40 * time.Millisecond))

		delay = calculateNextDelay(delay, config)
		Expect(delay).To(Equal(80 * time.Millisecond))
	})

	It("caps at max", func() {
		config := TickerConfig{
			BaseDelay:  10 * time.Millisecond,
			Multiplier: 10,
			MaxDelay:   10 * time.Millisecond,
		}

		delay := config.BaseDelay

		delay = calculateNextDelay(delay, config)
		Expect(delay).To(Equal(10 * time.Millisecond))

		delay = calculateNextDelay(delay, config)
		Expect(delay).To(Equal(10 * time.Millisecond))

		delay = calculateNextDelay(delay, config)
		Expect(delay).To(Equal(10 * time.Millisecond))
	})

	It("default inputs", func() {
		// default multiplier of 2, delay of 100ms, no max
		cfg := TickerConfig{}
		cfg = validateConfig(cfg)

		Expect(cfg.BaseDelay).To(Equal(100 * time.Millisecond))
		Expect(cfg.Multiplier).To(Equal(2.0))
		Expect(cfg.MaxDelay).To(Equal(0 * time.Millisecond))
	})

	It("validate inputs", func() {
		Expect(func() {
			NewExponentialTicker(TickerConfig{
				BaseDelay: -10 * time.Millisecond,
			})
		}).To(Panic())
		Expect(func() {
			NewExponentialTicker(TickerConfig{
				Multiplier: -1,
			})
		}).To(Panic())
		Expect(func() {
			NewExponentialTicker(TickerConfig{
				MaxDelay: -999 * time.Hour,
			})
		}).To(Panic())
	})

	It("respects context", func() {
		ctx, cancel := context.WithCancel(context.Background())
		ticker, _ := NewExponentialTicker(TickerConfig{
			MaxDelay: 10 * time.Millisecond,
			Context:  ctx,
		})
		Eventually(ticker, time.Second, time.Millisecond).Should(Receive())
		cancel()
		Consistently(ticker).ShouldNot(Receive())
	})

	It("respects cancel", func() {
		ticker, cancel := NewExponentialTicker(TickerConfig{
			MaxDelay: 10 * time.Millisecond,
		})
		Eventually(ticker, time.Second, time.Millisecond).Should(Receive())
		cancel()
		Consistently(ticker).ShouldNot(Receive())
	})
})
