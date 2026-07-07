# gw2 CLI — Foundation + Catalog Engine Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship a working, tested `gw2` CLI skeleton that authenticates with an encrypted API key, enforces the GW2 rate limit, and fetches any static "catalog" endpoint end-to-end — proven through `gw2 data colors` plus `gw2 auth` and `gw2 --version`.

**Architecture:** A Go/Cobra CLI. `internal/api` is a self-contained HTTP client (rate limiter, id-chunking, pagination, schema/lang params, error→exit-code mapping). `internal/auth` stores API keys encrypted (AES-256-GCM) in `~/.config/gw2/config.toml`. `internal/output` renders concise/raw/json. `internal/commands` wires Cobra; the `data` group is a **catalog engine**: one shared handler driven by a resource registry table, so each of the ~64 static endpoints is one table row. This plan wires the engine and registers a single resource (`colors`); the remaining rows are a follow-on plan.

**Tech Stack:** Go 1.25+, `github.com/spf13/cobra`, stdlib `crypto/aes`+`crypto/cipher`, `github.com/BurntSushi/toml`, `net/http/httptest` for tests.

## Global Constraints

- Module path: `github.com/howar31/gldwrs2` (source side). Binary/command, config dir, env prefix, skills: `gw2` (user side). `gldwrs2` appears only in the module path / imports / repo; never in user-facing strings.
- Go version floor: `go 1.25`.
- Schema pin constant `api.SchemaVersion = "2026-07-07T00:00:00Z"`, sent as `v=` on every request unless the caller overrides.
- Rate limit: token bucket, burst 300, refill 5/sec. Bulk requests cap at 200 ids each.
- Languages: `en`,`es`,`de`,`fr`,`zh` (default `en`).
- Exit codes: `0` ok · `3` auth (403) · `4` not found (404) · `5` rate-limited (429, after retries) · `1` other.
- Never print or log the API key or the encryption key.
- Tests never hit the live API: they use `httptest` servers via `WithBaseURL`. No real ids, keys, or account data in fixtures.
- Code comments and test names in English.
- **Commits & git:** this repo is not yet `git init`-ed. `git init` and every commit happen **only on the user's explicit instruction** (per the user's commit discipline) — the commit steps below are the intended checkpoints, not license to auto-commit. Messages use Conventional Commits.

---

## File Structure

- `go.mod` — module + deps.
- `VERSION` — version SSOT (plain `0.1.0`).
- `version.go` — embeds VERSION into a package var.
- `cmd/gw2/main.go` — entry; builds root command, maps errors to exit codes.
- `internal/api/client.go` — `Client`, `Get`, `GetByIDs`, `GetAllPages`, options.
- `internal/api/limiter.go` — token-bucket `Limiter`.
- `internal/api/errors.go` — `APIError`, `ExitCode`.
- `internal/api/*_test.go` — client, limiter, errors tests.
- `internal/auth/store.go` — encrypted key store + profiles.
- `internal/auth/store_test.go`.
- `internal/output/render.go` — `Mode`, `Render`, `Conciser`.
- `internal/output/render_test.go`.
- `internal/commands/app.go` — `App` (shared deps) + `Root` builder.
- `internal/commands/data.go` — catalog engine + `catalogResources` registry (colors row).
- `internal/commands/auth.go` — `gw2 auth set/list/remove`.
- `internal/commands/*_test.go` + `internal/commands/zz_coverage_test.go`.

---

## Task 1: Project scaffold + `gw2 --version`

**Files:**
- Create: `go.mod`, `VERSION`, `version.go`, `cmd/gw2/main.go`, `internal/commands/app.go`
- Test: `internal/commands/root_test.go`

**Interfaces:**
- Produces: `commands.Root(version string) *cobra.Command`; `Version string` (package `gldwrs2`, in `version.go`).

- [ ] **Step 1: Write the failing test**

`internal/commands/root_test.go`:
```go
package commands

import (
	"bytes"
	"testing"
)

func TestRootVersion(t *testing.T) {
	root := Root("9.9.9")
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"--version"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if got := out.String(); got == "" || !bytes.Contains(out.Bytes(), []byte("9.9.9")) {
		t.Fatalf("version output = %q, want it to contain 9.9.9", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/commands/ -run TestRootVersion`
Expected: FAIL — package/`Root` undefined (compile error).

- [ ] **Step 3: Create the module and minimal implementation**

`go.mod`:
```
module github.com/howar31/gldwrs2

go 1.25
```
Then: `go get github.com/spf13/cobra@latest github.com/BurntSushi/toml@latest`

`VERSION`:
```
0.1.0
```

`version.go`:
```go
package gldwrs2

import _ "embed"

//go:embed VERSION
var version string

// Version is the CLI version, sourced from the VERSION file.
var Version = trimSpace(version)

func trimSpace(s string) string {
	for len(s) > 0 && (s[len(s)-1] == '\n' || s[len(s)-1] == '\r' || s[len(s)-1] == ' ') {
		s = s[:len(s)-1]
	}
	return s
}
```

`internal/commands/app.go`:
```go
package commands

import (
	"io"
	"os"

	"github.com/spf13/cobra"
)

// App carries dependencies shared by all commands, populated from the root
// persistent flags in PersistentPreRunE.
type App struct {
	Out     io.Writer
	Profile string
	Lang    string
}

// Root builds the gw2 root command tree.
func Root(version string) *cobra.Command {
	app := &App{Out: os.Stdout}
	root := &cobra.Command{
		Use:           "gw2",
		Short:         "Guild Wars 2 API command-line client",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.SetVersionTemplate(
		"gw2 {{.Version}}\nSource: github.com/howar31/gldwrs2\n")
	root.PersistentFlags().StringVar(&app.Profile, "profile", "", "credential profile name")
	root.PersistentFlags().StringVar(&app.Lang, "lang", "en", "response language (en,es,de,fr,zh)")
	return root
}
```

`cmd/gw2/main.go`:
```go
package main

import (
	"fmt"
	"os"

	gldwrs2 "github.com/howar31/gldwrs2"
	"github.com/howar31/gldwrs2/internal/api"
	"github.com/howar31/gldwrs2/internal/commands"
)

func main() {
	root := commands.Root(gldwrs2.Version)
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "gw2:", err)
		os.Exit(api.ExitCode(err))
	}
}
```
(Note: `api.ExitCode` is created in Task 3; until then, temporarily replace `os.Exit(api.ExitCode(err))` with `os.Exit(1)` and drop the `api` import so the tree compiles. Restore it in Task 3.)

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/commands/ -run TestRootVersion`
Expected: PASS.

- [ ] **Step 5: Verify the binary builds and runs**

Run: `go build -o gw2 ./cmd/gw2 && ./gw2 --version`
Expected: prints `gw2 0.1.0` then `Source: github.com/howar31/gldwrs2`.

- [ ] **Step 6: Commit** (only if the user has authorized commits)

```bash
git add go.mod go.sum VERSION version.go cmd internal
git commit -m "feat: scaffold gw2 cli with version command"
```

---

## Task 2: API client core (`Get` + schema/lang params + base-URL override)

**Files:**
- Create: `internal/api/client.go`
- Test: `internal/api/client_test.go`

**Interfaces:**
- Produces:
  - `const DefaultBaseURL = "https://api.guildwars2.com"`
  - `const SchemaVersion = "2026-07-07T00:00:00Z"`
  - `const MaxIDsPerRequest = 200`
  - `type Client struct{...}` with `func New(opts ...Option) *Client`
  - `type Option func(*Client)`; `WithBaseURL`, `WithToken`, `WithLang`, `WithHTTPClient`, `WithUserAgent`, `WithLimiter`, `WithSleeper`
  - `func (c *Client) Get(ctx context.Context, path string, params url.Values) (json.RawMessage, error)`

- [ ] **Step 1: Write the failing test**

`internal/api/client_test.go`:
```go
package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestGetSendsSchemaLangAndToken(t *testing.T) {
	var gotPath, gotAuth, gotUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.String()
		gotAuth = r.Header.Get("Authorization")
		gotUA = r.Header.Get("User-Agent")
		w.WriteHeader(200)
		w.Write([]byte(`{"id":1}`))
	}))
	defer srv.Close()

	c := New(WithBaseURL(srv.URL), WithToken("KEY"), WithLang("de"), WithUserAgent("gw2-test"))
	raw, err := c.Get(context.Background(), "/v2/colors", url.Values{"ids": {"1"}})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(raw) != `{"id":1}` {
		t.Fatalf("body = %s", raw)
	}
	if gotAuth != "Bearer KEY" {
		t.Fatalf("auth = %q", gotAuth)
	}
	if gotUA != "gw2-test" {
		t.Fatalf("ua = %q", gotUA)
	}
	u, _ := url.Parse(gotPath)
	q := u.Query()
	if q.Get("v") != SchemaVersion {
		t.Fatalf("schema v = %q", q.Get("v"))
	}
	if q.Get("lang") != "de" {
		t.Fatalf("lang = %q", q.Get("lang"))
	}
	if q.Get("ids") != "1" {
		t.Fatalf("ids = %q", q.Get("ids"))
	}
	_ = json.RawMessage(raw)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/api/ -run TestGetSendsSchemaLangAndToken`
Expected: FAIL — `New`/`Client` undefined.

- [ ] **Step 3: Write the implementation**

`internal/api/client.go`:
```go
package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	DefaultBaseURL   = "https://api.guildwars2.com"
	SchemaVersion    = "2026-07-07T00:00:00Z"
	MaxIDsPerRequest = 200
	defaultUserAgent = "gw2-cli"
	maxRetries       = 5
)

type Client struct {
	baseURL   string
	token     string
	lang      string
	userAgent string
	httpc     *http.Client
	limiter   *Limiter
	sleep     func(time.Duration)
}

type Option func(*Client)

func WithBaseURL(u string) Option        { return func(c *Client) { c.baseURL = strings.TrimRight(u, "/") } }
func WithToken(t string) Option          { return func(c *Client) { c.token = t } }
func WithLang(l string) Option           { return func(c *Client) { c.lang = l } }
func WithUserAgent(ua string) Option     { return func(c *Client) { c.userAgent = ua } }
func WithHTTPClient(h *http.Client) Option { return func(c *Client) { c.httpc = h } }
func WithLimiter(l *Limiter) Option      { return func(c *Client) { c.limiter = l } }
func WithSleeper(f func(time.Duration)) Option { return func(c *Client) { c.sleep = f } }

func New(opts ...Option) *Client {
	c := &Client{
		baseURL:   DefaultBaseURL,
		lang:      "en",
		userAgent: defaultUserAgent,
		httpc:     &http.Client{Timeout: 30 * time.Second},
		sleep:     time.Sleep,
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// Get performs a single GET, injecting schema (v) and lang params unless the
// caller already set them. Transient statuses (429, 502-504) are retried.
func (c *Client) Get(ctx context.Context, path string, params url.Values) (json.RawMessage, error) {
	if params == nil {
		params = url.Values{}
	} else {
		params = cloneValues(params)
	}
	if params.Get("v") == "" && SchemaVersion != "" {
		params.Set("v", SchemaVersion)
	}
	if params.Get("lang") == "" && c.lang != "" {
		params.Set("lang", c.lang)
	}
	u := c.baseURL + path
	if enc := params.Encode(); enc != "" {
		u += "?" + enc
	}

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if c.limiter != nil {
			if err := c.limiter.Wait(ctx); err != nil {
				return nil, err
			}
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", c.userAgent)
		if c.token != "" {
			req.Header.Set("Authorization", "Bearer "+c.token)
		}
		resp, err := c.httpc.Do(req)
		if err != nil {
			return nil, err
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode < 400 {
			return json.RawMessage(body), nil
		}
		if isTransient(resp.StatusCode) && attempt < maxRetries {
			c.sleep(backoff(attempt, resp.Header.Get("Retry-After")))
			lastErr = apiErrorFrom(resp.StatusCode, body)
			continue
		}
		return nil, apiErrorFrom(resp.StatusCode, body)
	}
	return nil, lastErr
}

func isTransient(code int) bool {
	return code == http.StatusTooManyRequests || (code >= 502 && code <= 504)
}

func backoff(attempt int, retryAfter string) time.Duration {
	if retryAfter != "" {
		if secs, err := time.ParseDuration(retryAfter + "s"); err == nil {
			return secs
		}
	}
	d := 500 * time.Millisecond
	for i := 0; i < attempt; i++ {
		d *= 2
	}
	if d > 30*time.Second {
		d = 30 * time.Second
	}
	return d
}

func cloneValues(v url.Values) url.Values {
	out := make(url.Values, len(v))
	for k, vs := range v {
		out[k] = append([]string(nil), vs...)
	}
	return out
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/api/ -run TestGetSendsSchemaLangAndToken`
Expected: FAIL to compile — `apiErrorFrom` undefined. That lives in Task 3. Add a temporary stub at the bottom of `client.go` to keep this task self-contained:
```go
// temporary; replaced by errors.go in Task 3
func apiErrorFrom(code int, body []byte) error { return &tmpErr{code, string(body)} }

type tmpErr struct { code int; body string }
func (e *tmpErr) Error() string { return e.body }
```
Re-run. Expected: PASS.

- [ ] **Step 5: Commit** (if authorized)

```bash
git add internal/api/client.go internal/api/client_test.go go.mod go.sum
git commit -m "feat(api): http client with schema, lang, token and retry"
```

---

## Task 3: Error mapping → exit codes

**Files:**
- Create: `internal/api/errors.go`
- Modify: `internal/api/client.go` (delete the `tmpErr`/`apiErrorFrom` stub), `cmd/gw2/main.go` (restore `api.ExitCode`)
- Test: `internal/api/errors_test.go`

**Interfaces:**
- Produces: `type APIError struct { StatusCode int; Body string; Hint string }`; `func (e *APIError) Error() string`; `func ExitCode(err error) int`; internal `func apiErrorFrom(code int, body []byte) *APIError`.

- [ ] **Step 1: Write the failing test**

`internal/api/errors_test.go`:
```go
package api

import (
	"errors"
	"testing"
)

func TestExitCodeMapping(t *testing.T) {
	cases := []struct {
		code int
		want int
	}{
		{403, 3}, {404, 4}, {429, 5}, {500, 1}, {503, 1},
	}
	for _, c := range cases {
		err := apiErrorFrom(c.code, []byte(`{"text":"boom"}`))
		if got := ExitCode(err); got != c.want {
			t.Fatalf("status %d -> exit %d, want %d", c.code, got, c.want)
		}
	}
	if ExitCode(nil) != 0 {
		t.Fatal("nil -> 0")
	}
	if ExitCode(errors.New("plain")) != 1 {
		t.Fatal("plain -> 1")
	}
}

func TestAuthHintOnInvalidKey(t *testing.T) {
	err := apiErrorFrom(403, []byte(`{"text":"invalid key"}`))
	var ae *APIError
	if !errors.As(err, &ae) || ae.Hint == "" {
		t.Fatalf("expected auth hint, got %+v", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/api/ -run TestExitCodeMapping`
Expected: FAIL — `APIError`/`ExitCode` undefined (and duplicate `apiErrorFrom` once you add errors.go; remove the stub in Step 3).

- [ ] **Step 3: Write the implementation and remove the stub**

Delete the temporary `apiErrorFrom`/`tmpErr` block from `client.go` (Task 2 Step 4).

`internal/api/errors.go`:
```go
package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// APIError is a non-2xx response from the GW2 API.
type APIError struct {
	StatusCode int
	Body       string
	Hint       string
}

func (e *APIError) Error() string {
	msg := fmt.Sprintf("api %d", e.StatusCode)
	if t := extractText(e.Body); t != "" {
		msg += ": " + t
	}
	if e.Hint != "" {
		msg += " (" + e.Hint + ")"
	}
	return msg
}

func apiErrorFrom(code int, body []byte) *APIError {
	e := &APIError{StatusCode: code, Body: string(body)}
	if code == http.StatusForbidden {
		text := strings.ToLower(extractText(e.Body))
		switch {
		case strings.Contains(text, "invalid key"):
			e.Hint = "check your API key (gw2 auth set)"
		case strings.Contains(text, "requires scope") || strings.Contains(text, "permission"):
			e.Hint = "your key lacks a required permission; see gw2 token info"
		default:
			e.Hint = "authentication required or insufficient permission"
		}
	}
	return e
}

func extractText(body string) string {
	var m struct {
		Text  string `json:"text"`
		Error string `json:"error"`
	}
	if json.Unmarshal([]byte(body), &m) == nil {
		if m.Text != "" {
			return m.Text
		}
		if m.Error != "" {
			return m.Error
		}
	}
	return ""
}

// ExitCode maps an error to a process exit code.
func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	var ae *APIError
	if errors.As(err, &ae) {
		switch ae.StatusCode {
		case http.StatusForbidden:
			return 3
		case http.StatusNotFound:
			return 4
		case http.StatusTooManyRequests:
			return 5
		}
	}
	return 1
}
```

Restore `cmd/gw2/main.go` to use `os.Exit(api.ExitCode(err))` with the `api` import.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/api/ -run 'TestExitCode|TestAuthHint'`
Expected: PASS. Also `go build ./...` succeeds.

- [ ] **Step 5: Commit** (if authorized)

```bash
git add internal/api/errors.go internal/api/errors_test.go internal/api/client.go cmd/gw2/main.go
git commit -m "feat(api): map api errors to exit codes with auth hints"
```

---

## Task 4: Rate limiter (token bucket + retry)

**Files:**
- Create: `internal/api/limiter.go`
- Test: `internal/api/limiter_test.go`

**Interfaces:**
- Produces: `type Limiter struct{...}`; `func NewLimiter(max, refillPerSec float64) *Limiter`; `func NewLimiterClock(max, refillPerSec float64, now func() time.Time) *Limiter`; `func (l *Limiter) Wait(ctx context.Context) error`; internal `func (l *Limiter) tryAcquire() (ok bool, wait time.Duration)`.

- [ ] **Step 1: Write the failing test**

`internal/api/limiter_test.go`:
```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/api/ -run TestLimiterBurstThenRefill`
Expected: FAIL — `NewLimiterClock` undefined.

- [ ] **Step 3: Write the implementation**

`internal/api/limiter.go`:
```go
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
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/api/ -run TestLimiterBurstThenRefill`
Expected: PASS.

- [ ] **Step 5: Commit** (if authorized)

```bash
git add internal/api/limiter.go internal/api/limiter_test.go
git commit -m "feat(api): token-bucket rate limiter"
```

---

## Task 5: Bulk id chunking (`GetByIDs`)

**Files:**
- Modify: `internal/api/client.go` (add `GetByIDs` + `chunkIDs`)
- Test: `internal/api/getbyids_test.go`

**Interfaces:**
- Produces: `func (c *Client) GetByIDs(ctx context.Context, path string, ids []string, params url.Values) ([]json.RawMessage, error)`; internal `func chunkIDs(ids []string, size int) [][]string`.

- [ ] **Step 1: Write the failing test**

`internal/api/getbyids_test.go`:
```go
package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGetByIDsChunksAt200AndMerges(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		ids := strings.Split(r.URL.Query().Get("ids"), ",")
		var b strings.Builder
		b.WriteString("[")
		for i, id := range ids {
			if i > 0 {
				b.WriteString(",")
			}
			fmt.Fprintf(&b, `{"id":%s}`, id)
		}
		b.WriteString("]")
		w.Write([]byte(b.String()))
	}))
	defer srv.Close()

	ids := make([]string, 0, 250)
	for i := 1; i <= 250; i++ {
		ids = append(ids, fmt.Sprintf("%d", i))
	}
	c := New(WithBaseURL(srv.URL))
	out, err := c.GetByIDs(context.Background(), "/v2/items", ids, nil)
	if err != nil {
		t.Fatalf("GetByIDs: %v", err)
	}
	if calls != 2 {
		t.Fatalf("expected 2 chunked calls, got %d", calls)
	}
	if len(out) != 250 {
		t.Fatalf("merged len = %d, want 250", len(out))
	}
}

func TestChunkIDs(t *testing.T) {
	got := chunkIDs([]string{"a", "b", "c"}, 2)
	if len(got) != 2 || len(got[0]) != 2 || len(got[1]) != 1 {
		t.Fatalf("chunk = %v", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/api/ -run 'GetByIDs|ChunkIDs'`
Expected: FAIL — `GetByIDs`/`chunkIDs` undefined.

- [ ] **Step 3: Write the implementation (append to `client.go`)**

```go
// GetByIDs fetches multiple resources, chunking ids at MaxIDsPerRequest and
// merging the returned arrays.
func (c *Client) GetByIDs(ctx context.Context, path string, ids []string, params url.Values) ([]json.RawMessage, error) {
	var all []json.RawMessage
	for _, chunk := range chunkIDs(ids, MaxIDsPerRequest) {
		p := url.Values{}
		if params != nil {
			p = cloneValues(params)
		}
		p.Set("ids", strings.Join(chunk, ","))
		raw, err := c.Get(ctx, path, p)
		if err != nil {
			return nil, err
		}
		var batch []json.RawMessage
		if err := json.Unmarshal(raw, &batch); err != nil {
			return nil, err
		}
		all = append(all, batch...)
	}
	return all, nil
}

func chunkIDs(ids []string, size int) [][]string {
	if size <= 0 {
		size = MaxIDsPerRequest
	}
	var out [][]string
	for i := 0; i < len(ids); i += size {
		end := i + size
		if end > len(ids) {
			end = len(ids)
		}
		out = append(out, ids[i:end])
	}
	return out
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/api/ -run 'GetByIDs|ChunkIDs'`
Expected: PASS.

- [ ] **Step 5: Commit** (if authorized)

```bash
git add internal/api/client.go internal/api/getbyids_test.go
git commit -m "feat(api): bulk id chunking with merge"
```

---

## Task 6: Pagination (`GetAllPages`)

**Files:**
- Modify: `internal/api/client.go` (add `GetAllPages`)
- Test: `internal/api/pages_test.go`

**Interfaces:**
- Produces: `func (c *Client) GetAllPages(ctx context.Context, path string, params url.Values) ([]json.RawMessage, error)` — walks pages using the `X-Page-Total` header.

- [ ] **Step 1: Write the failing test**

`internal/api/pages_test.go`:
```go
package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetAllPagesFollowsPageTotal(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page := r.URL.Query().Get("page")
		w.Header().Set("X-Page-Total", "3")
		w.WriteHeader(200)
		switch page {
		case "0":
			fmt.Fprint(w, `[{"id":1},{"id":2}]`)
		case "1":
			fmt.Fprint(w, `[{"id":3},{"id":4}]`)
		case "2":
			fmt.Fprint(w, `[{"id":5}]`)
		default:
			t.Errorf("unexpected page %q", page)
		}
	}))
	defer srv.Close()

	c := New(WithBaseURL(srv.URL))
	out, err := c.GetAllPages(context.Background(), "/v2/colors", nil)
	if err != nil {
		t.Fatalf("GetAllPages: %v", err)
	}
	if len(out) != 5 {
		t.Fatalf("len = %d, want 5", len(out))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/api/ -run TestGetAllPages`
Expected: FAIL — `GetAllPages` undefined.

- [ ] **Step 3: Write the implementation (append to `client.go`)**

`GetAllPages` needs the response headers, so add a small internal `getWithResp` variant. Append:
```go
// getWithHeader is like Get but also returns the X-Page-Total header value.
func (c *Client) getWithPageTotal(ctx context.Context, path string, params url.Values) (json.RawMessage, int, error) {
	if params == nil {
		params = url.Values{}
	} else {
		params = cloneValues(params)
	}
	if params.Get("v") == "" {
		params.Set("v", SchemaVersion)
	}
	if params.Get("lang") == "" && c.lang != "" {
		params.Set("lang", c.lang)
	}
	u := c.baseURL + path
	if enc := params.Encode(); enc != "" {
		u += "?" + enc
	}
	if c.limiter != nil {
		if err := c.limiter.Wait(ctx); err != nil {
			return nil, 0, err
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("User-Agent", c.userAgent)
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.httpc.Do(req)
	if err != nil {
		return nil, 0, err
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, 0, apiErrorFrom(resp.StatusCode, body)
	}
	total := 1
	if v := resp.Header.Get("X-Page-Total"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			total = n
		}
	}
	return json.RawMessage(body), total, nil
}

// GetAllPages walks every page (page_size 200) using X-Page-Total.
func (c *Client) GetAllPages(ctx context.Context, path string, params url.Values) ([]json.RawMessage, error) {
	base := url.Values{}
	if params != nil {
		base = cloneValues(params)
	}
	base.Set("page_size", "200")
	var all []json.RawMessage
	total := 1
	for page := 0; page < total; page++ {
		p := cloneValues(base)
		p.Set("page", strconv.Itoa(page))
		raw, t, err := c.getWithPageTotal(ctx, path, p)
		if err != nil {
			return nil, err
		}
		total = t
		var batch []json.RawMessage
		if err := json.Unmarshal(raw, &batch); err != nil {
			return nil, err
		}
		all = append(all, batch...)
	}
	return all, nil
}
```
Add `"strconv"` to the `client.go` import block.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/api/ -run TestGetAllPages`
Expected: PASS. Run full package: `go test ./internal/api/` — all PASS.

- [ ] **Step 5: Commit** (if authorized)

```bash
git add internal/api/client.go internal/api/pages_test.go
git commit -m "feat(api): auto-pagination via X-Page-Total"
```

---

## Task 7: Encrypted credential store + profiles

**Files:**
- Create: `internal/auth/store.go`
- Test: `internal/auth/store_test.go`

**Interfaces:**
- Produces:
  - `type Store struct{...}`; `func NewStore(dir string, key []byte) *Store`
  - `func DefaultDir() (string, error)` → `~/.config/gw2`
  - `func LoadOrCreateKey(dir string) ([]byte, error)` → 32-byte key at `<dir>/key` (0600), overridable by `GW2_KEYRING_BACKEND=file:<path>`
  - `func (s *Store) Set(profile, token string) error`
  - `func (s *Store) Get(profile string) (string, error)`
  - `func (s *Store) List() ([]string, error)`
  - `func (s *Store) Remove(profile string) error`
  - `func (s *Store) DefaultProfile() (string, error)`

- [ ] **Step 1: Write the failing test**

`internal/auth/store_test.go`:
```go
package auth

import (
	"path/filepath"
	"testing"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	return NewStore(dir, key)
}

func TestSetGetRoundTrip(t *testing.T) {
	s := newTestStore(t)
	if err := s.Set("main", "SECRET-KEY"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, err := s.Get("main")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != "SECRET-KEY" {
		t.Fatalf("got %q", got)
	}
}

func TestTokenNotStoredInPlaintext(t *testing.T) {
	dir := t.TempDir()
	key := make([]byte, 32)
	s := NewStore(dir, key)
	if err := s.Set("main", "PLAINTEXT-SECRET"); err != nil {
		t.Fatal(err)
	}
	data := readFile(t, filepath.Join(dir, "config.toml"))
	if contains(data, "PLAINTEXT-SECRET") {
		t.Fatal("token stored in plaintext")
	}
}

func TestListAndRemoveAndDefault(t *testing.T) {
	s := newTestStore(t)
	s.Set("a", "ka")
	s.Set("b", "kb")
	names, _ := s.List()
	if len(names) != 2 {
		t.Fatalf("names = %v", names)
	}
	def, _ := s.DefaultProfile()
	if def != "a" {
		t.Fatalf("default = %q, want first-set 'a'", def)
	}
	if err := s.Remove("a"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get("a"); err == nil {
		t.Fatal("expected error after remove")
	}
}
```
Add helpers at the bottom of the test file:
```go
import "os"
import "strings"

func readFile(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
func contains(hay, needle string) bool { return strings.Contains(hay, needle) }
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/auth/`
Expected: FAIL — `NewStore` etc. undefined.

- [ ] **Step 3: Write the implementation**

`internal/auth/store.go`:
```go
package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/BurntSushi/toml"
)

type profile struct {
	Enc   string `toml:"enc"`   // base64(nonce||ciphertext)
	Order int    `toml:"order"` // set-order, for stable DefaultProfile
}

type config struct {
	Default  string             `toml:"default"`
	Profiles map[string]profile `toml:"profiles"`
}

// Store persists API keys encrypted with AES-256-GCM in <dir>/config.toml.
type Store struct {
	dir string
	key []byte // 32 bytes
}

func NewStore(dir string, key []byte) *Store { return &Store{dir: dir, key: key} }

func DefaultDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "gw2"), nil
}

// LoadOrCreateKey returns the 32-byte encryption key, creating one at
// <dir>/key (0600) on first use. GW2_KEYRING_BACKEND=file:<path> overrides.
func LoadOrCreateKey(dir string) ([]byte, error) {
	path := filepath.Join(dir, "key")
	if b := os.Getenv("GW2_KEYRING_BACKEND"); len(b) > 5 && b[:5] == "file:" {
		path = b[5:]
	}
	if data, err := os.ReadFile(path); err == nil {
		if len(data) != 32 {
			return nil, fmt.Errorf("key file %s is not 32 bytes", path)
		}
		return data, nil
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, key, 0o600); err != nil {
		return nil, err
	}
	return key, nil
}

func (s *Store) path() string { return filepath.Join(s.dir, "config.toml") }

func (s *Store) load() (config, error) {
	c := config{Profiles: map[string]profile{}}
	data, err := os.ReadFile(s.path())
	if errors.Is(err, os.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return c, err
	}
	if err := toml.Unmarshal(data, &c); err != nil {
		return c, err
	}
	if c.Profiles == nil {
		c.Profiles = map[string]profile{}
	}
	return c, nil
}

func (s *Store) save(c config) error {
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(s.path(), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	return toml.NewEncoder(f).Encode(c)
}

func (s *Store) gcm() (cipher.AEAD, error) {
	block, err := aes.NewCipher(s.key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func (s *Store) Set(name, token string) error {
	g, err := s.gcm()
	if err != nil {
		return err
	}
	nonce := make([]byte, g.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return err
	}
	ct := g.Seal(nonce, nonce, []byte(token), nil)
	c, err := s.load()
	if err != nil {
		return err
	}
	order := len(c.Profiles)
	if existing, ok := c.Profiles[name]; ok {
		order = existing.Order
	}
	c.Profiles[name] = profile{Enc: base64.StdEncoding.EncodeToString(ct), Order: order}
	if c.Default == "" {
		c.Default = name
	}
	return s.save(c)
}

func (s *Store) Get(name string) (string, error) {
	c, err := s.load()
	if err != nil {
		return "", err
	}
	p, ok := c.Profiles[name]
	if !ok {
		return "", fmt.Errorf("profile %q not found", name)
	}
	raw, err := base64.StdEncoding.DecodeString(p.Enc)
	if err != nil {
		return "", err
	}
	g, err := s.gcm()
	if err != nil {
		return "", err
	}
	ns := g.NonceSize()
	if len(raw) < ns {
		return "", errors.New("ciphertext too short")
	}
	pt, err := g.Open(nil, raw[:ns], raw[ns:], nil)
	if err != nil {
		return "", err
	}
	return string(pt), nil
}

func (s *Store) List() ([]string, error) {
	c, err := s.load()
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(c.Profiles))
	for n := range c.Profiles {
		names = append(names, n)
	}
	sort.Slice(names, func(i, j int) bool {
		return c.Profiles[names[i]].Order < c.Profiles[names[j]].Order
	})
	return names, nil
}

func (s *Store) Remove(name string) error {
	c, err := s.load()
	if err != nil {
		return err
	}
	if _, ok := c.Profiles[name]; !ok {
		return fmt.Errorf("profile %q not found", name)
	}
	delete(c.Profiles, name)
	if c.Default == name {
		c.Default = ""
		best := int(^uint(0) >> 1)
		for n, p := range c.Profiles {
			if p.Order < best {
				best, c.Default = p.Order, n
			}
		}
	}
	return s.save(c)
}

func (s *Store) DefaultProfile() (string, error) {
	c, err := s.load()
	if err != nil {
		return "", err
	}
	if c.Default == "" {
		return "", errors.New("no profiles configured")
	}
	return c.Default, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/auth/`
Expected: PASS.

- [ ] **Step 5: Commit** (if authorized)

```bash
git add internal/auth/ go.mod go.sum
git commit -m "feat(auth): aes-256-gcm encrypted key store with profiles"
```

---

## Task 8: Output rendering (concise / raw / json)

**Files:**
- Create: `internal/output/render.go`
- Test: `internal/output/render_test.go`

**Interfaces:**
- Produces: `type Mode int` with `ModeConcise`, `ModeRaw`, `ModeJSON`; `func Render(w io.Writer, raw json.RawMessage, mode Mode, concise string) error`. When `mode==ModeConcise` and `concise==""`, falls back to pretty JSON.

- [ ] **Step 1: Write the failing test**

`internal/output/render_test.go`:
```go
package output

import (
	"bytes"
	"strings"
	"testing"
)

func TestRenderRawPassthrough(t *testing.T) {
	var b bytes.Buffer
	if err := Render(&b, []byte(`{"id":1}`), ModeRaw, "human"); err != nil {
		t.Fatal(err)
	}
	if b.String() != `{"id":1}` {
		t.Fatalf("raw = %q", b.String())
	}
}

func TestRenderJSONPretty(t *testing.T) {
	var b bytes.Buffer
	if err := Render(&b, []byte(`{"id":1}`), ModeJSON, "human"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(b.String(), "\n  \"id\": 1") {
		t.Fatalf("json = %q", b.String())
	}
}

func TestRenderConciseUsesString(t *testing.T) {
	var b bytes.Buffer
	if err := Render(&b, []byte(`{"id":1}`), ModeConcise, "red #FF0000"); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(b.String()) != "red #FF0000" {
		t.Fatalf("concise = %q", b.String())
	}
}

func TestRenderConciseFallsBackToJSON(t *testing.T) {
	var b bytes.Buffer
	if err := Render(&b, []byte(`{"id":1}`), ModeConcise, ""); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(b.String(), "\"id\": 1") {
		t.Fatalf("fallback = %q", b.String())
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/output/`
Expected: FAIL — `Render`/`Mode` undefined.

- [ ] **Step 3: Write the implementation**

`internal/output/render.go`:
```go
package output

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

type Mode int

const (
	ModeConcise Mode = iota
	ModeRaw
	ModeJSON
)

// Conciser is implemented by response types that render a compact form.
type Conciser interface {
	Concise() string
}

// Render writes raw JSON in the chosen mode. In ModeConcise, the pre-rendered
// concise string is used; an empty concise falls back to pretty JSON.
func Render(w io.Writer, raw json.RawMessage, mode Mode, concise string) error {
	switch mode {
	case ModeRaw:
		_, err := w.Write(raw)
		return err
	case ModeJSON:
		return writePretty(w, raw)
	default:
		if concise != "" {
			_, err := fmt.Fprintln(w, concise)
			return err
		}
		return writePretty(w, raw)
	}
}

func writePretty(w io.Writer, raw json.RawMessage) error {
	var buf bytes.Buffer
	if err := json.Indent(&buf, raw, "", "  "); err != nil {
		// not valid JSON object/array; emit as-is
		_, err2 := w.Write(raw)
		return err2
	}
	buf.WriteByte('\n')
	_, err := w.Write(buf.Bytes())
	return err
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/output/`
Expected: PASS.

- [ ] **Step 5: Commit** (if authorized)

```bash
git add internal/output/
git commit -m "feat(output): concise/raw/json rendering"
```

---

## Task 9: Catalog engine + `gw2 data colors` end-to-end

**Files:**
- Modify: `internal/commands/app.go` (add client factory + mode wiring)
- Create: `internal/commands/data.go`, `internal/commands/zz_coverage_test.go`
- Test: `internal/commands/data_test.go`

**Interfaces:**
- Consumes: `api.New/Get/GetByIDs/GetAllPages`, `auth.Store`, `output.Render/Mode`.
- Produces:
  - `App` gains: fields `BaseURL string`, `Mode output.Mode`, `Store *auth.Store`, `Limiter *api.Limiter`; method `func (a *App) client() (*api.Client, error)`.
  - `type catalogResource struct { name, path string; localized bool; render func([]json.RawMessage) string }`
  - `var catalogResources []catalogResource`
  - `func newDataCmd(app *App) *cobra.Command`
  - test hook `var markCovered func(string)` pattern via `zz_coverage_test.go`.

- [ ] **Step 1: Write the failing test**

`internal/commands/data_test.go`:
```go
package commands

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/howar31/gldwrs2/internal/api"
	"github.com/howar31/gldwrs2/internal/output"
)

func TestDataColorsByIDsConcise(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/v2/colors") {
			t.Errorf("path = %s", r.URL.Path)
		}
		w.Write([]byte(`[{"id":10,"name":"Red"},{"id":11,"name":"Blue"}]`))
	}))
	defer srv.Close()

	var out bytes.Buffer
	app := &App{Out: &out, BaseURL: srv.URL, Lang: "en", Mode: output.ModeConcise, Limiter: api.NewLimiter(300, 5)}
	root := &cobra.Command{Use: "gw2"}
	root.AddCommand(newDataCmd(app))
	root.SetArgs([]string{"data", "colors", "--ids", "10,11"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "Red") || !strings.Contains(got, "Blue") {
		t.Fatalf("output = %q", got)
	}
}

func TestDataBareResourceDoesNotDumpAll(t *testing.T) {
	var fetched bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("ids") != "" {
			fetched = true
		}
		w.Write([]byte(`[1,2,3]`)) // bare list = ids only
	}))
	defer srv.Close()

	var out bytes.Buffer
	app := &App{Out: &out, BaseURL: srv.URL, Lang: "en", Mode: output.ModeConcise, Limiter: api.NewLimiter(300, 5)}
	root := &cobra.Command{Use: "gw2"}
	root.AddCommand(newDataCmd(app))
	root.SetArgs([]string{"data", "colors"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if fetched {
		t.Fatal("bare `data colors` must not auto-fetch all entries")
	}
	if !strings.Contains(out.String(), "1") {
		t.Fatalf("expected id list, got %q", out.String())
	}
}
```
Add `import "github.com/spf13/cobra"` to the test file.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/commands/ -run TestData`
Expected: FAIL — `newDataCmd`, `App.BaseURL`, etc. undefined.

- [ ] **Step 3: Extend `App` in `app.go`**

Replace `internal/commands/app.go` with:
```go
package commands

import (
	"io"
	"os"

	"github.com/howar31/gldwrs2/internal/api"
	"github.com/howar31/gldwrs2/internal/auth"
	"github.com/howar31/gldwrs2/internal/output"
	"github.com/spf13/cobra"
)

// App carries dependencies shared by all commands.
type App struct {
	Out     io.Writer
	BaseURL string
	Profile string
	Lang    string
	Mode    output.Mode
	Store   *auth.Store
	Limiter *api.Limiter
}

// client builds an api.Client for the current profile. A missing/empty
// credential is fine for public endpoints (token stays empty).
func (a *App) client() (*api.Client, error) {
	token := ""
	if a.Store != nil {
		name := a.Profile
		if name == "" {
			if def, err := a.Store.DefaultProfile(); err == nil {
				name = def
			}
		}
		if name != "" {
			if tok, err := a.Store.Get(name); err == nil {
				token = tok
			}
		}
	}
	return api.New(
		api.WithBaseURL(a.BaseURL),
		api.WithToken(token),
		api.WithLang(a.Lang),
		api.WithLimiter(a.Limiter),
		api.WithUserAgent("gw2-cli/"+versionOrDev),
	), nil
}

var versionOrDev = "dev"

// Root builds the gw2 root command tree.
func Root(version string) *cobra.Command {
	versionOrDev = version
	app := &App{Out: os.Stdout, Mode: output.ModeConcise, Lang: "en", BaseURL: api.DefaultBaseURL}
	var raw, jsonOut bool

	root := &cobra.Command{
		Use:           "gw2",
		Short:         "Guild Wars 2 API command-line client",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if v := os.Getenv("GW2_API_BASE"); v != "" {
				app.BaseURL = v
			}
			switch {
			case raw:
				app.Mode = output.ModeRaw
			case jsonOut:
				app.Mode = output.ModeJSON
			default:
				app.Mode = output.ModeConcise
			}
			dir, err := auth.DefaultDir()
			if err != nil {
				return err
			}
			key, err := auth.LoadOrCreateKey(dir)
			if err != nil {
				return err
			}
			app.Store = auth.NewStore(dir, key)
			app.Limiter = api.NewLimiter(300, 5)
			return nil
		},
	}
	root.SetVersionTemplate("gw2 {{.Version}}\nSource: github.com/howar31/gldwrs2\n")
	root.PersistentFlags().StringVar(&app.Profile, "profile", "", "credential profile name")
	root.PersistentFlags().StringVar(&app.Lang, "lang", "en", "response language (en,es,de,fr,zh)")
	root.PersistentFlags().BoolVar(&raw, "raw", false, "print the API's raw JSON")
	root.PersistentFlags().BoolVar(&jsonOut, "json", false, "print pretty JSON")

	root.AddCommand(newDataCmd(app), newAuthCmd(app))
	return root
}
```
(Note: `newAuthCmd` is Task 10. Until then, temporarily register only `newDataCmd(app)` so the tree compiles; add `newAuthCmd(app)` in Task 10.)

- [ ] **Step 4: Write the catalog engine `data.go`**

`internal/commands/data.go`:
```go
package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/howar31/gldwrs2/internal/output"
	"github.com/spf13/cobra"
)

// catalogResource is one static "game data" endpoint following the
// enumerate-ids -> fetch-by-ids pattern.
type catalogResource struct {
	name      string // subcommand name, e.g. "colors"
	path      string // API path, e.g. "/v2/colors"
	localized bool
	render    func([]json.RawMessage) string // optional concise; nil -> JSON fallback
}

// catalogResources is the registry. This plan registers colors; the full
// ~64-row table is a follow-on plan.
var catalogResources = []catalogResource{
	{name: "colors", path: "/v2/colors", localized: true, render: renderColors},
}

func renderColors(items []json.RawMessage) string {
	var b strings.Builder
	for _, it := range items {
		var c struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		}
		if err := json.Unmarshal(it, &c); err != nil {
			continue
		}
		fmt.Fprintf(&b, "%d\t%s\n", c.ID, c.Name)
	}
	return strings.TrimRight(b.String(), "\n")
}

func newDataCmd(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "data",
		Short: "Static game data (items, colors, recipes, ...)",
	}
	for _, res := range catalogResources {
		cmd.AddCommand(newCatalogResourceCmd(app, res))
	}
	return cmd
}

func newCatalogResourceCmd(app *App, res catalogResource) *cobra.Command {
	var ids string
	var all bool
	c := &cobra.Command{
		Use:   res.name,
		Short: "Fetch " + res.name,
		RunE: func(cmd *cobra.Command, args []string) error {
			markCovered("data " + res.name)
			ctx := context.Background()
			client, err := app.client()
			if err != nil {
				return err
			}
			// No ids and not --all: show the id list only (never auto-dump).
			if ids == "" && !all {
				raw, err := client.Get(ctx, res.path, nil)
				if err != nil {
					return err
				}
				return output.Render(app.Out, raw, app.Mode, conciseIDList(raw))
			}
			var idList []string
			if all {
				raw, err := client.Get(ctx, res.path, nil)
				if err != nil {
					return err
				}
				var nums []json.RawMessage
				if err := json.Unmarshal(raw, &nums); err != nil {
					return err
				}
				for _, n := range nums {
					idList = append(idList, strings.Trim(string(n), `"`))
				}
			} else {
				idList = strings.Split(ids, ",")
			}
			items, err := client.GetByIDs(ctx, res.path, idList, nil)
			if err != nil {
				return err
			}
			concise := ""
			if res.render != nil {
				concise = res.render(items)
			}
			merged, _ := json.Marshal(items)
			return output.Render(app.Out, merged, app.Mode, concise)
		},
	}
	c.Flags().StringVar(&ids, "ids", "", "comma-separated ids (omit to list ids)")
	c.Flags().BoolVar(&all, "all", false, "fetch every entry (explicit; may be large)")
	return c
}

func conciseIDList(raw json.RawMessage) string {
	var ids []json.RawMessage
	if err := json.Unmarshal(raw, &ids); err != nil {
		return ""
	}
	parts := make([]string, 0, len(ids))
	for _, id := range ids {
		parts = append(parts, strings.Trim(string(id), `"`))
	}
	return strings.Join(parts, " ")
}
```

- [ ] **Step 5: Add the coverage hook `zz_coverage_test.go`**

`internal/commands/zz_coverage_test.go`:
```go
package commands

import (
	"sort"
	"sync"
	"testing"
)

var (
	coverMu   sync.Mutex
	coverSeen = map[string]bool{}
)

func init() { markCovered = func(id string) { coverMu.Lock(); coverSeen[id] = true; coverMu.Unlock() } }

// leafCommands enumerates every leaf command that must be exercised by a test.
// Extend this as command groups are added.
func leafCommands() []string {
	var ids []string
	for _, res := range catalogResources {
		ids = append(ids, "data "+res.name)
	}
	sort.Strings(ids)
	return ids
}

func TestZZAllLeafCommandsCovered(t *testing.T) {
	coverMu.Lock()
	defer coverMu.Unlock()
	for _, id := range leafCommands() {
		if !coverSeen[id] {
			t.Errorf("leaf command %q was never exercised by a test", id)
		}
	}
}
```
And declare the hook in non-test code so production builds compile — add to `data.go`:
```go
// markCovered is a no-op in production; tests replace it to track coverage.
var markCovered = func(string) {}
```

- [ ] **Step 6: Run tests to verify they pass**

Run: `go test ./internal/commands/`
Expected: PASS (both data tests and the coverage meta-test — `data colors` is exercised by `TestDataColorsByIDsConcise`).

- [ ] **Step 7: Verify end-to-end against a mock, then the real API**

Run:
```bash
go build -o gw2 ./cmd/gw2
GW2_API_BASE=https://api.guildwars2.com ./gw2 data colors --ids 10,11
```
Expected: two tab-separated `id<TAB>name` lines. Then `./gw2 data colors` (bare) prints a space-separated id list without dumping details.

- [ ] **Step 8: Commit** (if authorized)

```bash
git add internal/commands/
git commit -m "feat(data): catalog engine with colors resource and coverage test"
```

---

## Task 10: `gw2 auth set/list/remove`

**Files:**
- Create: `internal/commands/auth.go`
- Modify: `internal/commands/app.go` (register `newAuthCmd`), `internal/commands/zz_coverage_test.go` (add auth leaves)
- Test: `internal/commands/auth_test.go`

**Interfaces:**
- Consumes: `App.Store` (`*auth.Store`).
- Produces: `func newAuthCmd(app *App) *cobra.Command` with `set <name>`, `list`, `remove <name>` subcommands.

- [ ] **Step 1: Write the failing test**

`internal/commands/auth_test.go`:
```go
package commands

import (
	"bytes"
	"strings"
	"testing"

	"github.com/howar31/gldwrs2/internal/auth"
	"github.com/spf13/cobra"
)

func testApp(t *testing.T, out *bytes.Buffer) *App {
	t.Helper()
	dir := t.TempDir()
	key := make([]byte, 32)
	return &App{Out: out, Store: auth.NewStore(dir, key)}
}

func TestAuthSetListRemove(t *testing.T) {
	var out bytes.Buffer
	app := testApp(t, &out)

	run := func(args ...string) {
		root := &cobra.Command{Use: "gw2"}
		root.AddCommand(newAuthCmd(app))
		root.SetArgs(args)
		if err := root.Execute(); err != nil {
			t.Fatalf("execute %v: %v", args, err)
		}
	}

	run("auth", "set", "main", "--key", "SECRET")
	out.Reset()
	run("auth", "list")
	if !strings.Contains(out.String(), "main") {
		t.Fatalf("list = %q", out.String())
	}
	if strings.Contains(out.String(), "SECRET") {
		t.Fatal("list must never print the key")
	}
	out.Reset()
	run("auth", "remove", "main")
	out.Reset()
	run("auth", "list")
	if strings.Contains(out.String(), "main") {
		t.Fatalf("main should be gone, got %q", out.String())
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/commands/ -run TestAuthSetListRemove`
Expected: FAIL — `newAuthCmd` undefined.

- [ ] **Step 3: Write the implementation**

`internal/commands/auth.go`:
```go
package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newAuthCmd(app *App) *cobra.Command {
	cmd := &cobra.Command{Use: "auth", Short: "Manage stored API keys"}

	var key string
	set := &cobra.Command{
		Use:   "set <profile>",
		Short: "Store an API key under a profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			markCovered("auth set")
			if key == "" {
				return fmt.Errorf("--key is required")
			}
			if err := app.Store.Set(args[0], key); err != nil {
				return err
			}
			fmt.Fprintf(app.Out, "stored key for profile %q\n", args[0])
			return nil
		},
	}
	set.Flags().StringVar(&key, "key", "", "the API key value")

	list := &cobra.Command{
		Use:   "list",
		Short: "List stored profiles (never prints keys)",
		RunE: func(cmd *cobra.Command, args []string) error {
			markCovered("auth list")
			names, err := app.Store.List()
			if err != nil {
				return err
			}
			def, _ := app.Store.DefaultProfile()
			for _, n := range names {
				marker := ""
				if n == def {
					marker = " (default)"
				}
				fmt.Fprintf(app.Out, "%s%s\n", n, marker)
			}
			return nil
		},
	}

	remove := &cobra.Command{
		Use:   "remove <profile>",
		Short: "Remove a stored profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			markCovered("auth remove")
			if err := app.Store.Remove(args[0]); err != nil {
				return err
			}
			fmt.Fprintf(app.Out, "removed profile %q\n", args[0])
			return nil
		},
	}

	cmd.AddCommand(set, list, remove)
	return cmd
}
```

Register it: in `app.go` `Root`, change `root.AddCommand(newDataCmd(app))` to `root.AddCommand(newDataCmd(app), newAuthCmd(app))`.

Extend the coverage list: in `zz_coverage_test.go` `leafCommands()`, append the auth leaves:
```go
	ids = append(ids, "auth set", "auth list", "auth remove")
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/commands/`
Expected: PASS (auth test + coverage meta-test now sees auth leaves exercised).

- [ ] **Step 5: Full build + vet**

Run: `go build ./... && go vet ./... && go test ./...`
Expected: all PASS.

- [ ] **Step 6: Commit** (if authorized)

```bash
git add internal/commands/
git commit -m "feat(auth): auth set/list/remove commands"
```

---

## Self-Review

**1. Spec coverage (foundation scope):**
- Naming split (§2) → Task 1 (`Root` version template `Source: github.com/howar31/gldwrs2`, module path), Global Constraints.
- Architecture / layout (§3) → Tasks 2–10 create `internal/{api,auth,output,commands}` + `cmd/gw2`.
- Auth & credentials (§6) → Task 7 (encrypted store, profiles, keyring env override) + Task 10 (commands). Scope-403 hint → Task 3.
- Query features (§7): bulk ids → Task 5; pagination → Task 6; `--lang` + schema pin → Task 2; `--raw`/`--json`/concise → Task 8. `--all` explicit / no implicit dump → Task 9 (`TestDataBareResourceDoesNotDumpAll`).
- Rate limiting & errors (§8) → Task 4 (limiter) + Task 2 (retry) + Task 3 (exit codes).
- Catalog archetype (§5.1) → Task 9 engine + registry.
- Testing (§10): httptest + `GW2_API_BASE`/`WithBaseURL` overrides throughout; coverage meta-test → Task 9/10; scrubbed fixtures (synthetic ids, `SECRET`/`main`).
- **Deferred to follow-on plans (documented, not gaps):** full ~64-row catalog table; groups account/character/commerce/wvw/pvp/guild/achievements/token; skillgen; goreleaser/npm distribution; `gw2 token info` (the scope-hint referrer — the hint text names it ahead of its implementation, which is acceptable as user guidance).

**2. Placeholder scan:** No "TBD"/"handle edge cases"/"similar to". Temporary stubs (Task 2 `tmpErr`, Task 9 partial `AddCommand`) are explicitly created and explicitly removed in a named later step — not placeholders.

**3. Type consistency:** `Client.Get/GetByIDs/GetAllPages` signatures identical across Tasks 2/5/6 and consumed in Task 9. `output.Render(w, raw, mode, concise)` defined Task 8, called Task 9. `auth.Store` methods defined Task 7, consumed Tasks 9/10. `markCovered` declared in `data.go` (prod no-op) and overridden in `zz_coverage_test.go` — single declaration, no duplicate. `App` fields grow monotonically (Task 1 → Task 9) with the Task 9 rewrite being the authoritative version.

---

## Follow-on plans (written after this foundation is reviewed)

1. **Catalog table fill + skillgen** — populate all ~64 `catalogResources` rows (nest where the API nests, hyphenate flat leaves; §14) with concise renderers; add `gw2 generate-skills` → `gw2-data`.
2. **account** group (registry + ~6 bespoke renderers).
3. **character** group (+ `resolve` name→id).
4. **commerce**, **wvw**, **pvp**, **achievements** (domain renderers).
5. **guild** (+ `resolve` guild name→GUID) and **token/meta** (`token info` fulfilling the Task 3 hint).
6. **Distribution** — `.goreleaser.yaml` (cask `gw2`), `npm/` wrapper (`@howar31/gw2`), VERSION release flow, README/CLAUDE/SPEC.
