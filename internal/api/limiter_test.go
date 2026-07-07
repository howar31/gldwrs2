package api

import (
	"testing"
	"time"
)

func TestLimiterBurstThenRefill(t *testing.T) {
	now := time.Unix(0, 0)
	l := NewLimiterClock(3, 5, func() time.Time { return now })

	for i := 0; i < 3; i++ {
		if ok, _ := l.tryAcquire(); !ok {
			t.Fatalf("burst token %d should be available", i)
		}
	}
	ok, wait := l.tryAcquire()
	if ok {
		t.Fatal("4th token should be denied at t=0")
	}
	if wait <= 0 {
		t.Fatalf("expected positive wait, got %v", wait)
	}

	now = now.Add(time.Second) // refill 5/sec -> capped at max 3
	got := 0
	for {
		if ok, _ := l.tryAcquire(); !ok {
			break
		}
		got++
	}
	if got != 3 {
		t.Fatalf("after 1s got %d tokens, want 3 (capped at max)", got)
	}
}
