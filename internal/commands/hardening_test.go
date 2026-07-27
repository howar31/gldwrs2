package commands

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/howar31/gldwrs2/internal/api"
	"github.com/howar31/gldwrs2/internal/auth"
	"github.com/howar31/gldwrs2/internal/output"
	"github.com/spf13/cobra"
)

// TestSplitIDsTrims locks the --ids whitespace fix: `--ids "1, 2"` must
// query "2", not the literal " 2".
func TestSplitIDsTrims(t *testing.T) {
	got := splitIDs(" 1, 2 ,,3 ")
	want := []string{"1", "2", "3"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("splitIDs = %#v, want %#v", got, want)
	}
}

// TestListLeafRejectsPositionalArgs locks the cobra.NoArgs fix: a stray
// positional id on a by-ids list command must be a usage error before any
// network call -- not silently ignored while the full id list is dumped.
func TestListLeafRejectsPositionalArgs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("no request should be made when positional args are rejected, got %s", r.URL)
	}))
	defer srv.Close()

	var out bytes.Buffer
	app := &App{Out: &out, BaseURL: srv.URL, Lang: "en", Mode: output.ModeConcise, Limiter: api.NewLimiter(300, 5)}
	root := &cobra.Command{Use: "gw2", SilenceUsage: true, SilenceErrors: true}
	root.AddCommand(newDataCmd(app))
	root.SetArgs([]string{"data", "items", "123"})
	if err := root.Execute(); err == nil {
		t.Fatal("`data items 123` should be a usage error, got nil")
	}
}

// TestRawModeSkipsConciseRender locks the wasted-work fix: in --raw/--json
// modes the concise renderer's output is discarded by output.Render, so it
// must not run at all.
func TestRawModeSkipsConciseRender(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`[{"id":1}]`))
	}))
	defer srv.Close()

	rendered := 0
	var out bytes.Buffer
	app := &App{Out: &out, BaseURL: srv.URL, Lang: "en", Mode: output.ModeRaw, Limiter: api.NewLimiter(300, 5)}
	root := &cobra.Command{Use: "gw2"}
	root.AddCommand(newByIDsListCmd(app, "things", "Fetch things", "/v2/things",
		"data things-test", func(items []json.RawMessage) string { rendered++; return "x" }, false, false))
	root.SetArgs([]string{"things", "--ids", "1"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if rendered != 0 {
		t.Fatalf("concise renderer ran %d times in raw mode, want 0", rendered)
	}
}

// TestClientCorruptConfigSurfaces locks the swallowed-store-error fix: a
// corrupt config.toml must surface as a store error, not be silently
// treated as "no profiles configured".
func TestClientCorruptConfigSurfaces(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "config.toml"), []byte("not [valid toml"), 0o600); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	app := &App{Out: &out, Store: auth.NewStore(dir, make([]byte, 32))}
	if _, err := app.client(); err == nil {
		t.Fatal("client() with a corrupt config.toml should surface the store error, got nil")
	}
	if _, err := app.authedClient(); err == nil {
		t.Fatal("authedClient() with a corrupt config.toml should surface the store error, got nil")
	} else if strings.Contains(err.Error(), "needs an API key") {
		t.Fatalf("corrupt config misreported as missing key: %v", err)
	}
}

// TestPublicAnonymousWhenStoreUnavailable locks the lazy-store fix: a public
// command must work anonymously even where no credential store can be
// opened (e.g. an unreadable config dir / no usable HOME), while an authed
// command must surface the real store error instead of the generic
// "needs an API key" guidance.
func TestPublicAnonymousWhenStoreUnavailable(t *testing.T) {
	denied := filepath.Join(t.TempDir(), "denied")
	if err := os.Mkdir(denied, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(denied, 0o755) })
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(denied, "cfg"))
	t.Setenv("GW2_KEYRING_BACKEND", "")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			t.Error("public request should be anonymous")
		}
		w.Write([]byte(`[1,2,3]`))
	}))
	defer srv.Close()

	var out bytes.Buffer
	app := &App{Out: &out, BaseURL: srv.URL, Lang: "en", Mode: output.ModeConcise, Limiter: api.NewLimiter(300, 5), lazyStore: true}
	root := &cobra.Command{Use: "gw2", SilenceUsage: true, SilenceErrors: true}
	root.AddCommand(newDataCmd(app))
	root.SetArgs([]string{"data", "colors"})
	if err := root.Execute(); err != nil {
		t.Fatalf("public command should work without a usable store, got: %v", err)
	}

	// Authed path on a fresh App (openStore caches): must surface the real
	// store failure, not misdirect to `gw2 auth set`.
	app2 := &App{Out: &out, BaseURL: srv.URL, lazyStore: true}
	if _, err := app2.authedClient(); err == nil {
		t.Fatal("authedClient() with an unopenable store should error, got nil")
	} else if strings.Contains(err.Error(), "needs an API key") {
		t.Fatalf("store failure misreported as missing key: %v", err)
	}
}

// TestCommerceTransactionsPaginates locks the pagination fix: transaction
// history beyond the API's first page must be fetched, not silently
// dropped.
func TestCommerceTransactionsPaginates(t *testing.T) {
	var pages []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/commerce/transactions/history/sells" {
			t.Errorf("path = %s", r.URL.Path)
		}
		page := r.URL.Query().Get("page")
		pages = append(pages, page)
		w.Header().Set("X-Page-Total", "2")
		switch page {
		case "0":
			w.Write([]byte(`[{"item_id":1,"price":100,"quantity":1,"created":"2026-01-01"}]`))
		default:
			w.Write([]byte(`[{"item_id":2,"price":200,"quantity":2,"created":"2026-01-02"}]`))
		}
	}))
	defer srv.Close()

	var out bytes.Buffer
	app := authedTestApp(t, &out, srv)
	root := &cobra.Command{Use: "gw2"}
	root.AddCommand(newCommerceCmd(app))
	root.SetArgs([]string{"commerce", "transactions", "history", "sells"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(pages) != 2 {
		t.Fatalf("fetched %d pages (%v), want 2", len(pages), pages)
	}
	got := out.String()
	if !strings.Contains(got, "item 1") || !strings.Contains(got, "item 2") {
		t.Fatalf("output missing transactions from both pages:\n%s", got)
	}
}

// TestCharacterListOnePerLine locks the name-list fix: character names
// contain spaces, so the bare `gw2 character` list must print one name per
// line to keep name boundaries recoverable.
func TestCharacterListOnePerLine(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`["Zoja The Bold","Rytlock Fan"]`))
	}))
	defer srv.Close()

	var out bytes.Buffer
	app := authedTestApp(t, &out, srv)
	root := &cobra.Command{Use: "gw2"}
	root.AddCommand(newCharacterCmd(app))
	root.SetArgs([]string{"character"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(out.String(), "Zoja The Bold\nRytlock Fan") {
		t.Fatalf("names should be one per line, got:\n%q", out.String())
	}
}
