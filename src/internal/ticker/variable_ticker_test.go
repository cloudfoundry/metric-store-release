package ticker_test

import (
	"context"
	"sync"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/cloudfoundry/metric-store-release/src/internal/ticker"
)

type fakeDelay struct {
	duration     time.Duration
	nextDuration time.Duration

	sync.Mutex
}

func (d *fakeDelay) Reset() {
	d.Lock()
	defer d.Unlock()
	d.duration = d.nextDuration
}

func (d *fakeDelay) Step() time.Duration {
	d.Lock()
	defer d.Unlock()
	return d.duration
}

var _ = Describe("A ticker for exponential backoffs", func() {
	It("respects context", func() {
		ctx, cancel := context.WithCancel(context.Background())
		delay := NewExponentialDelay(&Config{MaxDelay: 10 * time.Microsecond})

		ticker := New(delay, WithContext(ctx))
		defer ticker.Stop()

		Eventually(ticker.C, time.Second, time.Microsecond).Should(Receive())
		cancel()
		Consistently(ticker.C).ShouldNot(Receive())
	})

	It("stops", func() {
		delay := NewExponentialDelay(&Config{MaxDelay: 10 * time.Microsecond})
		ticker := New(delay)

		Eventually(ticker.C, time.Second, time.Microsecond).Should(Receive())
		ticker.Stop()
		Consistently(ticker.C).ShouldNot(Receive())
	})

	Context("Reset()", func() {
		It("resets the delay", func() {
			ticker := New(&fakeDelay{duration: time.Hour, nextDuration: time.Microsecond})
			defer ticker.Stop()

			Consistently(ticker.C, 10*time.Millisecond, time.Millisecond).ShouldNot(Receive())
			ticker.Reset()
			Eventually(ticker.C, 10*time.Millisecond, time.Millisecond).Should(Receive())
		})

		It("cancels the existing goroutine", func() {
			ticker := New(&fakeDelay{duration: time.Microsecond, nextDuration: time.Hour})
			defer ticker.Stop()

			Eventually(ticker.C, 10*time.Millisecond, time.Millisecond).Should(Receive())
			ticker.Reset()
			Consistently(ticker.C, 10*time.Millisecond, time.Millisecond).ShouldNot(Receive())
		})
	})
})
