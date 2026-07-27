package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// TestBackoffCapsRetryAfter locks the Retry-After cap: a server-supplied
// value (hostile or misconfigured) must never exceed maxBackoff, and a
// negative value must not produce a negative sleep.
func TestBackoffCapsRetryAfter(t *testing.T) {
	if got := backoff(0, "86400"); got != maxBackoff {
		t.Fatalf("backoff(Retry-After 86400) = %v, want capped at %v", got, maxBackoff)
	}
	if got := backoff(0, "-5"); got != 0 {
		t.Fatalf("backoff(Retry-After -5) = %v, want 0", got)
	}
	if got := backoff(0, "2"); got != 2*time.Second {
		t.Fatalf("backoff(Retry-After 2) = %v, want 2s", got)
	}
	if got := backoff(0, ""); got != 500*time.Millisecond {
		t.Fatalf("backoff(no header, attempt 0) = %v, want 500ms", got)
	}
}

// TestGetAllPagesRetriesTransientMidWalk locks the shared-pipeline fix: a
// transient 429 on one page of a paginated walk must retry that page, not
// fail the whole multi-page fetch.
func TestGetAllPagesRetriesTransientMidWalk(t *testing.T) {
	var page1Attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Page-Total", "2")
		switch r.URL.Query().Get("page") {
		case "0":
			fmt.Fprint(w, `[{"id":1}]`)
		case "1":
			if page1Attempts.Add(1) == 1 {
				w.WriteHeader(http.StatusTooManyRequests)
				return
			}
			fmt.Fprint(w, `[{"id":2}]`)
		default:
			t.Errorf("unexpected page %q", r.URL.Query().Get("page"))
		}
	}))
	defer srv.Close()

	c := New(WithBaseURL(srv.URL), WithSleeper(func(time.Duration) {}))
	out, err := c.GetAllPages(context.Background(), "/v2/things", nil)
	if err != nil {
		t.Fatalf("GetAllPages should survive a transient 429 mid-walk, got: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("merged len = %d, want 2", len(out))
	}
	if got := page1Attempts.Load(); got != 2 {
		t.Fatalf("page 1 attempts = %d, want 2 (one 429 + one success)", got)
	}
}

// TestGetSuppressesLangWithLangNone locks the localized wiring: a caller
// setting lang=LangNone must produce a request with NO lang param at all.
func TestGetSuppressesLangWithLangNone(t *testing.T) {
	var gotLang string
	var hadLang bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotLang = r.URL.Query().Get("lang")
		_, hadLang = r.URL.Query()["lang"]
		fmt.Fprint(w, `[]`)
	}))
	defer srv.Close()

	c := New(WithBaseURL(srv.URL), WithLang("de"))
	p := make(map[string][]string)
	p["lang"] = []string{LangNone}
	if _, err := c.Get(context.Background(), "/v2/recipes", p); err != nil {
		t.Fatal(err)
	}
	if hadLang {
		t.Fatalf("lang param should be absent with LangNone, got %q", gotLang)
	}

	if _, err := c.Get(context.Background(), "/v2/items", nil); err != nil {
		t.Fatal(err)
	}
	if gotLang != "de" {
		t.Fatalf("default lang injection broken, got %q want de", gotLang)
	}
}

// TestExitCode401MapsToAuth locks the 401 mapping: the live API answers 401
// ("Invalid access token") for missing/invalid keys on many authenticated
// endpoints; that is an auth failure (exit 3) with a key hint, same as 403.
func TestExitCode401MapsToAuth(t *testing.T) {
	err := apiErrorFrom(401, []byte(`{"text":"Invalid access token"}`))
	if got := ExitCode(err); got != 3 {
		t.Fatalf("ExitCode(401) = %d, want 3", got)
	}
	if err.Hint == "" {
		t.Fatal("401 Invalid access token should carry a key hint")
	}
}
