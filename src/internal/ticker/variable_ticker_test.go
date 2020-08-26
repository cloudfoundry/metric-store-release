package ticker_test

import (
	"context"
	"sync"
	"time"

	. "github.com/cloudfoundry/metric-store-release/src/internal/ticker"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type fakeDelay struct {
	ready          bool
	nextReadyState bool

	sync.Mutex
}

func (d *fakeDelay) Reset(time.Time) {
	d.Lock()
	defer d.Unlock()
	d.ready = d.nextReadyState
}

func (d *fakeDelay) Ready(time.Time) bool {
	d.Lock()
	defer d.Unlock()
	return d.ready
}

var _ = Describe("A ticker for exponential backoffs", func() {
	It("respects context", func() {
		ctx, cancel := context.WithCancel(context.Background())
		delay := NewExponentialDelay(&Config{MaxDelay: 10 * time.Millisecond})

		ticker := New(
			delay,
			WithContext(ctx),
		)

		Eventually(ticker.C, time.Second, time.Millisecond).Should(Receive())
		cancel()
		Consistently(ticker.C).ShouldNot(Receive())
	})

	It("stops", func() {
		delay := NewExponentialDelay(&Config{MaxDelay: 10 * time.Millisecond})
		ticker := New(delay)

		Eventually(ticker.C, time.Second, time.Millisecond).Should(Receive())
		ticker.Stop()
		Consistently(ticker.C).ShouldNot(Receive())
	})

	Context("Reset()", func() {
		It("resets the delay", func() {
			unreadyDelay := &fakeDelay{nextReadyState: true}
			ticker := New(unreadyDelay)

			ticker.Reset()
			Eventually(ticker.C, 10*time.Millisecond, time.Millisecond).Should(Receive())
		})

		It("cancels the existing goroutine", func() {
			d := &fakeDelay{ready: true, nextReadyState: true}
			ticker := New(d)

			Eventually(ticker.C, 10*time.Millisecond, time.Millisecond).Should(Receive())
			d.nextReadyState = false
			ticker.Reset()
			Consistently(ticker.C, 10*time.Millisecond, time.Millisecond).ShouldNot(Receive())
		})
	})
})
