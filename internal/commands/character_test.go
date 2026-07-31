package commands

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// TestCharacterList exercises the bare `gw2 character` leaf (no args):
// GET /v2/characters, a bare array of character names.
func TestCharacterList(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Write([]byte(`["Alice","Bob"]`))
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
	if gotAuth != "Bearer TESTTOKEN" {
		t.Fatalf("Authorization header = %q, want %q", gotAuth, "Bearer TESTTOKEN")
	}
	if !strings.Contains(out.String(), "Alice") || !strings.Contains(out.String(), "Bob") {
		t.Fatalf("output = %q, want it to contain Alice and Bob", out.String())
	}
}

// TestCharacterSubresource proves the character name is URL-escaped when
// building the subresource path (spaces -> %20 via url.PathEscape).
func TestCharacterSubresource(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.EscapedPath()
		w.Write([]byte(`[{"id":1,"count":1}]`))
	}))
	defer srv.Close()

	var out bytes.Buffer
	app := authedTestApp(t, &out, srv)
	root := &cobra.Command{Use: "gw2"}
	root.AddCommand(newCharacterCmd(app))
	root.SetArgs([]string{"character", "My Char", "equipment"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if gotPath != "/v2/characters/My%20Char/equipment" {
		t.Fatalf("request path = %q, want %q", gotPath, "/v2/characters/My%20Char/equipment")
	}
}

// TestCharacterActiveFlag verifies --active appends /active for an
// active-capable subresource (buildtabs).
func TestCharacterActiveFlag(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Write([]byte(`{"tab":1}`))
	}))
	defer srv.Close()

	var out bytes.Buffer
	app := authedTestApp(t, &out, srv)
	root := &cobra.Command{Use: "gw2"}
	root.AddCommand(newCharacterCmd(app))
	root.SetArgs([]string{"character", "X", "buildtabs", "--active"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.HasSuffix(gotPath, "/buildtabs/active") {
		t.Fatalf("request path = %q, want it to end in /buildtabs/active", gotPath)
	}
}

// TestCharacterUnknownSubresource verifies an unrecognized subresource
// errors instead of hitting the API with a bogus path.
func TestCharacterUnknownSubresource(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("unexpected request for unknown subresource: %s", r.URL.Path)
	}))
	defer srv.Close()

	var out bytes.Buffer
	app := authedTestApp(t, &out, srv)
	root := &cobra.Command{Use: "gw2"}
	root.AddCommand(newCharacterCmd(app))
	root.SetArgs([]string{"character", "X", "bogus"})
	err := root.Execute()
	if err == nil {
		t.Fatal("character X bogus should error")
	}
	if !strings.Contains(err.Error(), "valid") || !strings.Contains(err.Error(), "core") {
		t.Fatalf("error = %v, want it to list valid subresources", err)
	}
}

// TestCharacterRequiresToken verifies the character command goes through
// authedClient: with no profile/token configured, it must fail with
// guidance to run `gw2 auth set`, never silently send an anonymous request.
func TestCharacterRequiresToken(t *testing.T) {
	var out bytes.Buffer
	app := testApp(t, &out) // no profile configured -> empty token
	root := &cobra.Command{Use: "gw2"}
	root.AddCommand(newCharacterCmd(app))
	root.SetArgs([]string{"character"})
	err := root.Execute()
	if err == nil {
		t.Fatal("character with no configured key should error")
	}
	if !strings.Contains(err.Error(), "gw2 auth set") {
		t.Fatalf("error = %v, want it to mention gw2 auth set", err)
	}
}
