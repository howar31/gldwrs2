package api

import (
	"context"
	"sync"
	"time"
)

// Limiter is a token bucket. GW2 allows burst 300, refill 5/sec.
type Limiter struct {
	mu     sync.Mutex
	tokens float64
	max    float64
	refill float64
	last   time.Time
	now    func() time.Time
}

func NewLimiter(max, refillPerSec float64) *Limiter {
	return NewLimiterClock(max, refillPerSec, time.Now)
}

func NewLimiterClock(max, refillPerSec float64, now func() time.Time) *Limiter {
	return &Limiter{tokens: max, max: max, refill: refillPerSec, last: now(), now: now}
}

func (l *Limiter) tryAcquire() (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	t := l.now()
	l.tokens += t.Sub(l.last).Seconds() * l.refill
	if l.tokens > l.max {
		l.tokens = l.max
	}
	l.last = t
	if l.tokens >= 1 {
		l.tokens--
		return true, 0
	}
	wait := time.Duration((1 - l.tokens) / l.refill * float64(time.Second))
	return false, wait
}

// Wait blocks until a token is available or ctx is cancelled.
func (l *Limiter) Wait(ctx context.Context) error {
	for {
		ok, wait := l.tryAcquire()
		if ok {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(wait):
		}
	}
}
