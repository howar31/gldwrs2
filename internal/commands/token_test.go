package commands

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/howar31/gldwrs2/internal/api"
	"github.com/howar31/gldwrs2/internal/output"
	"github.com/spf13/cobra"
)

// TestBuildPublic exercises `gw2 build`, public (no configured key needed):
// GET /v2/build, {"id": 115000} -> output containing the build id.
func TestBuildPublic(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Write([]byte(`{"id":115000}`))
	}))
	defer srv.Close()

	var out bytes.Buffer
	app := &App{Out: &out, BaseURL: srv.URL, Lang: "en", Mode: output.ModeConcise, Limiter: api.NewLimiter(300, 5)}
	root := &cobra.Command{Use: "gw2"}
	root.AddCommand(newBuildCmd(app))
	root.SetArgs([]string{"build"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if gotPath != "/v2/build" {
		t.Fatalf("path = %q, want /v2/build", gotPath)
	}
	if !strings.Contains(out.String(), "115000") {
		t.Fatalf("output = %q, want it to contain the build id", out.String())
	}
}

// TestTokenInfoAuthed exercises `gw2 token info`, authed: GET /v2/tokeninfo
// with a bearer token, asserting the response's permissions show up in the
// concise output.
func TestTokenInfoAuthed(t *testing.T) {
	var gotPath, gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		w.Write([]byte(`{"id":"ABCD","name":"key","permissions":["account","characters"]}`))
	}))
	defer srv.Close()

	var out bytes.Buffer
	app := authedTestApp(t, &out, srv)
	root := &cobra.Command{Use: "gw2"}
	root.AddCommand(newTokenCmd(app))
	root.SetArgs([]string{"token", "info"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if gotPath != "/v2/tokeninfo" {
		t.Fatalf("path = %q, want /v2/tokeninfo", gotPath)
	}
	if gotAuth != "Bearer TESTTOKEN" {
		t.Fatalf("Authorization header = %q, want %q", gotAuth, "Bearer TESTTOKEN")
	}
	if !strings.Contains(out.String(), "account") {
		t.Fatalf("output = %q, want it to contain the account permission", out.String())
	}
}

// TestTokenSubtoken exercises `gw2 token subtoken --permissions ... --expire
// ...`, authed: GET /v2/createsubtoken with the flags mapped to query
// params, asserting the returned JWT is printed.
func TestTokenSubtoken(t *testing.T) {
	var gotPath, gotPermissions, gotExpire string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotPermissions = r.URL.Query().Get("permissions")
		gotExpire = r.URL.Query().Get("expire")
		w.Write([]byte(`{"subtoken":"JWT123"}`))
	}))
	defer srv.Close()

	var out bytes.Buffer
	app := authedTestApp(t, &out, srv)
	root := &cobra.Command{Use: "gw2"}
	root.AddCommand(newTokenCmd(app))
	root.SetArgs([]string{"token", "subtoken", "--permissions", "account,characters", "--expire", "2027-01-01T00:00:00Z"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if gotPath != "/v2/createsubtoken" {
		t.Fatalf("path = %q, want /v2/createsubtoken", gotPath)
	}
	if gotPermissions != "account,characters" {
		t.Fatalf("permissions query = %q, want %q", gotPermissions, "account,characters")
	}
	if gotExpire != "2027-01-01T00:00:00Z" {
		t.Fatalf("expire query = %q, want %q", gotExpire, "2027-01-01T00:00:00Z")
	}
	if !strings.Contains(out.String(), "JWT123") {
		t.Fatalf("output = %q, want it to contain the subtoken", out.String())
	}
}

// TestTokenInfoRequiresToken verifies `token info` goes through
// authedClient: with no profile/token configured, it must fail with
// guidance to run `gw2 auth set`, never silently send an anonymous request.
func TestTokenInfoRequiresToken(t *testing.T) {
	var out bytes.Buffer
	app := testApp(t, &out) // no profile configured -> empty token
	root := &cobra.Command{Use: "gw2"}
	root.AddCommand(newTokenCmd(app))
	root.SetArgs([]string{"token", "info"})
	err := root.Execute()
	if err == nil {
		t.Fatal("token info with no configured key should error")
	}
	if !strings.Contains(err.Error(), "gw2 auth set") {
		t.Fatalf("error = %v, want it to mention gw2 auth set", err)
	}
}
