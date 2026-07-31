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

// TestGuildSearch exercises `guild search <name>`, a public GET
// /v2/guild/search returning the list of guild ids matching that name.
func TestGuildSearch(t *testing.T) {
	var gotPath, gotName string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotName = r.URL.Query().Get("name")
		w.Write([]byte(`["GUID-1","GUID-2"]`))
	}))
	defer srv.Close()

	var out bytes.Buffer
	app := &App{Out: &out, BaseURL: srv.URL, Lang: "en", Mode: output.ModeConcise, Limiter: api.NewLimiter(300, 5)}
	root := &cobra.Command{Use: "gw2"}
	root.AddCommand(newGuildCmd(app))
	root.SetArgs([]string{"guild", "search", "Foo Bar"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if gotPath != "/v2/guild/search" {
		t.Fatalf("path = %q, want /v2/guild/search", gotPath)
	}
	if gotName != "Foo Bar" {
		t.Fatalf("name query = %q, want %q", gotName, "Foo Bar")
	}
	if !strings.Contains(out.String(), "GUID-1") {
		t.Fatalf("output = %q, want it to contain the guild ids", out.String())
	}
}

// TestGuildInfoByGUID exercises the bare positional form `guild <GUID>`: a
// GUID passes straight through resolve.GuildID (no /v2/guild/search call)
// and is fetched via the public client.
func TestGuildInfoByGUID(t *testing.T) {
	const guid = "AAAAAAAA-BBBB-CCCC-DDDD-EEEEEEEEEEEE"
	var requests []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.URL.Path)
		w.Write([]byte(`{"name":"Test Guild","tag":"TG","level":5,"member_count":10}`))
	}))
	defer srv.Close()

	var out bytes.Buffer
	app := &App{Out: &out, BaseURL: srv.URL, Lang: "en", Mode: output.ModeConcise, Limiter: api.NewLimiter(300, 5)}
	root := &cobra.Command{Use: "gw2"}
	root.AddCommand(newGuildCmd(app))
	root.SetArgs([]string{"guild", guid})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(requests) != 1 || requests[0] != "/v2/guild/"+guid {
		t.Fatalf("requests = %v, want exactly one request to /v2/guild/%s (GUID passthrough, no search call)", requests, guid)
	}
	if !strings.Contains(out.String(), "Test Guild") {
		t.Fatalf("output = %q, want it to contain the guild name", out.String())
	}
}

// TestGuildResolvesNameThenSubresource exercises the two-step positional
// dispatch for a guild NAME plus a subresource: the name is resolved via
// /v2/guild/search first, then the subresource is fetched from the
// resolved id via the authed client (guild leader/officer scope).
func TestGuildResolvesNameThenSubresource(t *testing.T) {
	var paths []string
	var membersAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		switch r.URL.Path {
		case "/v2/guild/search":
			if got := r.URL.Query().Get("name"); got != "My Guild" {
				t.Fatalf("search name = %q, want %q", got, "My Guild")
			}
			w.Write([]byte(`["GUID-1"]`))
		case "/v2/guild/GUID-1/members":
			membersAuth = r.Header.Get("Authorization")
			w.Write([]byte(`[{"name":"Alice","rank":"Leader","joined":"2020-01-01T00:00:00Z"}]`))
		default:
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	var out bytes.Buffer
	app := authedTestApp(t, &out, srv)
	root := &cobra.Command{Use: "gw2"}
	root.AddCommand(newGuildCmd(app))
	root.SetArgs([]string{"guild", "My Guild", "members"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(paths) != 2 || paths[0] != "/v2/guild/search" || paths[1] != "/v2/guild/GUID-1/members" {
		t.Fatalf("request paths = %v, want [/v2/guild/search /v2/guild/GUID-1/members]", paths)
	}
	if membersAuth != "Bearer TESTTOKEN" {
		t.Fatalf("members Authorization = %q, want %q", membersAuth, "Bearer TESTTOKEN")
	}
	if !strings.Contains(out.String(), "Alice") {
		t.Fatalf("output = %q, want it to contain Alice", out.String())
	}
}

// TestGuildSubresourceRequiresToken verifies the subresource fetch goes
// through authedClient: with no profile/token configured, it must fail
// with guidance to run `gw2 auth set`, never silently send an anonymous
// request. Resolving a GUID needs no HTTP call (passthrough), so this
// isolates the subresource fetch's auth requirement.
func TestGuildSubresourceRequiresToken(t *testing.T) {
	var out bytes.Buffer
	app := testApp(t, &out) // no profile configured -> empty token
	root := &cobra.Command{Use: "gw2"}
	root.AddCommand(newGuildCmd(app))
	root.SetArgs([]string{"guild", "AAAAAAAA-BBBB-CCCC-DDDD-EEEEEEEEEEEE", "members"})
	err := root.Execute()
	if err == nil {
		t.Fatal("guild <id> members with no configured key should error")
	}
	if !strings.Contains(err.Error(), "gw2 auth set") {
		t.Fatalf("error = %v, want it to mention gw2 auth set", err)
	}
}

// TestGuildPermissionsPublic exercises `guild permissions --ids`, public:
// no token needed.
func TestGuildPermissionsPublic(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Write([]byte(`[{"id":"EditBGColor","name":"Background Color"}]`))
	}))
	defer srv.Close()

	var out bytes.Buffer
	app := &App{Out: &out, BaseURL: srv.URL, Lang: "en", Mode: output.ModeConcise, Limiter: api.NewLimiter(300, 5)}
	root := &cobra.Command{Use: "gw2"}
	root.AddCommand(newGuildCmd(app))
	root.SetArgs([]string{"guild", "permissions", "--ids", "1"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if gotPath != "/v2/guild/permissions" {
		t.Fatalf("path = %q, want /v2/guild/permissions", gotPath)
	}
	if !strings.Contains(out.String(), "Background Color") {
		t.Fatalf("output = %q, want it to contain Background Color", out.String())
	}
}

// TestGuildUpgradesPublic exercises the bare `guild upgrades` subcommand
// (the public upgrade catalog), distinct from `guild <id> upgrades` (an
// authed per-guild subresource, covered by the positional-dispatch "guild"
// coverage id instead). Public: no token needed.
func TestGuildUpgradesPublic(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Write([]byte(`[{"id":1,"name":"Guild Hall Expansion"}]`))
	}))
	defer srv.Close()

	var out bytes.Buffer
	app := &App{Out: &out, BaseURL: srv.URL, Lang: "en", Mode: output.ModeConcise, Limiter: api.NewLimiter(300, 5)}
	root := &cobra.Command{Use: "gw2"}
	root.AddCommand(newGuildCmd(app))
	root.SetArgs([]string{"guild", "upgrades", "--ids", "1"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if gotPath != "/v2/guild/upgrades" {
		t.Fatalf("path = %q, want /v2/guild/upgrades", gotPath)
	}
	if !strings.Contains(out.String(), "Guild Hall Expansion") {
		t.Fatalf("output = %q, want it to contain Guild Hall Expansion", out.String())
	}
}
