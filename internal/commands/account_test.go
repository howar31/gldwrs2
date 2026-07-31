package commands

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/howar31/gldwrs2/internal/api"
	"github.com/howar31/gldwrs2/internal/auth"
	"github.com/howar31/gldwrs2/internal/output"
	"github.com/spf13/cobra"
)

// authedTestApp is like testApp, but pre-seeds a "main" profile with a token
// so app.authedClient() resolves a non-empty token, and points the client at
// srv so requests can be asserted against.
func authedTestApp(t *testing.T, out *bytes.Buffer, srv *httptest.Server) *App {
	t.Helper()
	dir := t.TempDir()
	key := make([]byte, 32)
	store := auth.NewStore(dir, key)
	if err := store.Set("main", "TESTTOKEN"); err != nil {
		t.Fatalf("seed profile: %v", err)
	}
	return &App{
		Out:     out,
		BaseURL: srv.URL,
		Lang:    "en",
		Mode:    output.ModeConcise,
		Store:   store,
		Limiter: api.NewLimiter(300, 5),
	}
}

// TestAccountBareFetch exercises the bare `gw2 account` leaf (the account
// info object), which lives directly on the "account" command's own RunE
// rather than as an accountResources entry (its segments would collide with
// the root command name).
func TestAccountBareFetch(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Write([]byte(`{"name":"Test.1234","world":1001,"created":"2020-01-01T00:00:00Z","access":["GuildWars2"]}`))
	}))
	defer srv.Close()

	var out bytes.Buffer
	app := authedTestApp(t, &out, srv)
	root := &cobra.Command{Use: "gw2"}
	root.AddCommand(newAccountCmd(app))
	root.SetArgs([]string{"account"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if gotAuth != "Bearer TESTTOKEN" {
		t.Fatalf("Authorization header = %q, want %q", gotAuth, "Bearer TESTTOKEN")
	}
	if !strings.Contains(out.String(), "Test.1234") {
		t.Fatalf("output = %q, want it to contain the account name", out.String())
	}
}

// TestAllAccountResourcesFetch is a table-driven test that exercises every
// registered accountResource leaf, asserting the command dispatches without
// error and that the request carried the profile's bearer token (proving
// the authedClient path is used, not the anonymous client()). This is what
// keeps the coverage meta-test (zz_coverage_test.go) green as the registry
// grows: every account resource is fetched here exactly once.
func TestAllAccountResourcesFetch(t *testing.T) {
	for _, res := range accountResources {
		res := res
		name := strings.Join(res.segments, " ")
		t.Run(name, func(t *testing.T) {
			var gotAuth string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotAuth = r.Header.Get("Authorization")
				// One body shape shared by every resource: a valid JSON
				// array carrying every field any bespoke renderer (or
				// conciseIDList) might look for. Endpoints with no render
				// (nil) ignore the shape entirely and pretty-print it.
				w.Write([]byte(`[{"id":1,"current":2,"max":3,"done":true,"category":4,"count":5,"value":6}]`))
			}))
			defer srv.Close()

			var out bytes.Buffer
			app := authedTestApp(t, &out, srv)
			root := &cobra.Command{Use: "gw2"}
			root.AddCommand(newAccountCmd(app))

			args := append([]string{"account"}, res.segments...)
			root.SetArgs(args)
			if err := root.Execute(); err != nil {
				t.Fatalf("execute %v: %v", args, err)
			}
			if gotAuth != "Bearer TESTTOKEN" {
				t.Fatalf("Authorization header = %q, want %q (authedClient not used?)", gotAuth, "Bearer TESTTOKEN")
			}
			if out.String() == "" {
				t.Fatalf("expected non-empty output for %q", name)
			}
		})
	}
}

// TestAccountRequiresToken verifies every account leaf goes through
// authedClient: with no profile/token configured, it must fail with
// guidance to run `gw2 auth set`, never silently send an anonymous request.
func TestAccountRequiresToken(t *testing.T) {
	var out bytes.Buffer
	app := testApp(t, &out) // no profile configured -> empty token
	root := &cobra.Command{Use: "gw2"}
	root.AddCommand(newAccountCmd(app))
	root.SetArgs([]string{"account", "wallet"})
	err := root.Execute()
	if err == nil {
		t.Fatal("account wallet with no configured key should error")
	}
	if !strings.Contains(err.Error(), "gw2 auth set") {
		t.Fatalf("error = %v, want it to mention gw2 auth set", err)
	}
}
