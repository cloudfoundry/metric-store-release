package storage_test

import (
	"context"
	"time"

	"github.com/cloudfoundry/metric-store-release/src/internal/storage"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("A ticker for exponential backoffs", func() {
	var expectTickerTiming = func(ticker chan struct{}, minWait time.Duration, maxWait time.Duration) {
		startTime := time.Now()
		Eventually(ticker, time.Second, time.Millisecond).Should(Receive())
		endTime := time.Now()

		Expect(endTime.Sub(startTime)).To(BeNumerically(">", minWait))
		Expect(endTime.Sub(startTime)).To(BeNumerically("<", maxWait))
	}

	It("ticks on first interval", func() {
		ticker, _ := storage.NewExponentialTicker(storage.TickerConfig{
			BaseDelay: 10 * time.Millisecond,
		})
		expectTickerTiming(ticker, 10*time.Millisecond, 20*time.Millisecond)
	})

	It("ticks exponentially", func() {
		ticker, _ := storage.NewExponentialTicker(storage.TickerConfig{
			BaseDelay:  10 * time.Millisecond,
			Multiplier: 2,
		})
		expectTickerTiming(ticker, 10*time.Millisecond, 20*time.Millisecond)
		expectTickerTiming(ticker, 20*time.Millisecond, 40*time.Millisecond)
		expectTickerTiming(ticker, 40*time.Millisecond, 80*time.Millisecond)
	})

	It("caps at max", func() {
		ticker, _ := storage.NewExponentialTicker(storage.TickerConfig{
			BaseDelay:  10 * time.Millisecond,
			Multiplier: 10,
			MaxDelay:   10 * time.Millisecond,
		})

		expectTickerTiming(ticker, 10*time.Millisecond, 20*time.Millisecond)
		expectTickerTiming(ticker, 10*time.Millisecond, 20*time.Millisecond)
		expectTickerTiming(ticker, 10*time.Millisecond, 20*time.Millisecond)
	})

	It("default inputs", func() {
		// default multiplier of 2, delay of 10ms, no max
		ticker, _ := storage.NewExponentialTicker(storage.TickerConfig{})

		expectTickerTiming(ticker, 10*time.Millisecond, 20*time.Millisecond)
		expectTickerTiming(ticker, 20*time.Millisecond, 40*time.Millisecond)
		expectTickerTiming(ticker, 40*time.Millisecond, 80*time.Millisecond)
	})

	It("validate inputs", func() {
		Expect(func() {
			storage.NewExponentialTicker(storage.TickerConfig{
				BaseDelay: -10 * time.Millisecond,
			})
		}).To(Panic())
		Expect(func() {
			storage.NewExponentialTicker(storage.TickerConfig{
				Multiplier: -1,
			})
		}).To(Panic())
		Expect(func() {
			storage.NewExponentialTicker(storage.TickerConfig{
				MaxDelay: -999 * time.Hour,
			})
		}).To(Panic())
	})

	It("respects context", func() {
		ctx, cancel := context.WithCancel(context.Background())
		ticker, _ := storage.NewExponentialTicker(storage.TickerConfig{
			MaxDelay: 10 * time.Millisecond,
			Context:  ctx,
		})
		Eventually(ticker, time.Second, time.Millisecond).Should(Receive())
		cancel()
		Consistently(ticker).ShouldNot(Receive())
	})

	It("respects cancel", func() {
		ticker, cancel := storage.NewExponentialTicker(storage.TickerConfig{
			MaxDelay: 10 * time.Millisecond,
		})
		Eventually(ticker, time.Second, time.Millisecond).Should(Receive())
		cancel()
		Consistently(ticker).ShouldNot(Receive())
	})
})
