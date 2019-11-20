package handoff

import (
	"testing"
	"time"
)

func TestLimiter(t *testing.T) {
	l := NewRateLimiter(0)
	l.Increment(500)
	if l.Delay().Nanoseconds() != 0 {
		t.Errorf("limiter with no limit mismatch: got %v, exp 0", l.Delay())
	}
}

func TestLimiterWithinLimit(t *testing.T) {
	l := NewRateLimiter(1000)

	l.Increment(50)
	l.Delay()
	time.Sleep(100 * time.Millisecond)

	// Should not have any delay
	delay := l.Delay().Nanoseconds()
	if exp := int(0); int(delay) != exp {
		t.Errorf("limiter rate mismatch: got %v, exp %v", int(delay), exp)
	}

}

func TestLimiterExceeded(t *testing.T) {
	l := NewRateLimiter(1000)
	for i := 0; i < 10; i++ {
		l.Increment(200)
		l.Delay()
	}
	delay := l.Delay().Seconds()
	if int(delay) == 0 {
		t.Errorf("limiter rate mismatch. expected non-zero delay")
	}
}
