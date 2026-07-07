package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"
)

// TestGetRetriesTransientThenSucceeds locks in Task 2's retry behavior: a
// transient 429 should be retried until the request succeeds.
func TestGetRetriesTransientThenSucceeds(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n <= 2 {
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"text":"rate limited"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":42}`))
	}))
	defer srv.Close()

	c := New(WithBaseURL(srv.URL), WithSleeper(func(time.Duration) {}))
	raw, err := c.Get(context.Background(), "/v2/colors", url.Values{"ids": {"1"}})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(raw) != `{"id":42}` {
		t.Fatalf("body = %s", raw)
	}
	if got := atomic.LoadInt32(&calls); got != 3 {
		t.Fatalf("calls = %d, want 3", got)
	}
}

// TestGetExhaustsRetriesReturnsAPIError locks in that once retries are
// exhausted, Get surfaces an *APIError with the last transient status so
// ExitCode can map it correctly.
func TestGetExhaustsRetriesReturnsAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(`{"text":"rate limited"}`))
	}))
	defer srv.Close()

	c := New(WithBaseURL(srv.URL), WithSleeper(func(time.Duration) {}))
	_, err := c.Get(context.Background(), "/v2/colors", url.Values{"ids": {"1"}})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var ae *APIError
	if !errors.As(err, &ae) {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if ae.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("StatusCode = %d, want %d", ae.StatusCode, http.StatusTooManyRequests)
	}
	if got := ExitCode(err); got != 5 {
		t.Fatalf("ExitCode = %d, want 5", got)
	}
}
