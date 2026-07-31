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

// TestWvwAbilitiesByIDs exercises `wvw abilities --ids`, the catalog-style
// by-ids branch shared by all five wvw list resources.
func TestWvwAbilitiesByIDs(t *testing.T) {
	var gotPath, gotIDs string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotIDs = r.URL.Query().Get("ids")
		w.Write([]byte(`[{"id":1,"name":"Test Ability"}]`))
	}))
	defer srv.Close()

	var out bytes.Buffer
	app := &App{Out: &out, BaseURL: srv.URL, Lang: "en", Mode: output.ModeConcise, Limiter: api.NewLimiter(300, 5)}
	root := &cobra.Command{Use: "gw2"}
	root.AddCommand(newWvwCmd(app))
	root.SetArgs([]string{"wvw", "abilities", "--ids", "1"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if gotPath != "/v2/wvw/abilities" {
		t.Fatalf("path = %q, want /v2/wvw/abilities", gotPath)
	}
	if gotIDs != "1" {
		t.Fatalf("ids query = %q, want %q", gotIDs, "1")
	}
	if out.String() == "" {
		t.Fatal("expected non-empty output")
	}
}

// TestWvwAbilitiesBareListsIDs verifies bare `wvw abilities` (no --ids/--all)
// never dumps every entry: the server must not receive an "ids" param, and
// the output is just the plain id list.
func TestWvwAbilitiesBareListsIDs(t *testing.T) {
	var fetchedIDs bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("ids") != "" {
			fetchedIDs = true
		}
		w.Write([]byte(`[1,2,3]`))
	}))
	defer srv.Close()

	var out bytes.Buffer
	app := &App{Out: &out, BaseURL: srv.URL, Lang: "en", Mode: output.ModeConcise, Limiter: api.NewLimiter(300, 5)}
	root := &cobra.Command{Use: "gw2"}
	root.AddCommand(newWvwCmd(app))
	root.SetArgs([]string{"wvw", "abilities"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if fetchedIDs {
		t.Fatal("bare `wvw abilities` must not auto-fetch all entries")
	}
	if !strings.Contains(out.String(), "1") {
		t.Fatalf("expected id list, got %q", out.String())
	}
}

// TestWvwMatchesByWorld exercises `wvw matches --world`, which passes
// ?world=N instead of an ids list.
func TestWvwMatchesByWorld(t *testing.T) {
	var gotPath, gotWorld string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotWorld = r.URL.Query().Get("world")
		w.Write([]byte(`{"id":"1001-1","world":1001}`))
	}))
	defer srv.Close()

	var out bytes.Buffer
	app := &App{Out: &out, BaseURL: srv.URL, Lang: "en", Mode: output.ModeConcise, Limiter: api.NewLimiter(300, 5)}
	root := &cobra.Command{Use: "gw2"}
	root.AddCommand(newWvwCmd(app))
	root.SetArgs([]string{"wvw", "matches", "--world", "1001"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if gotPath != "/v2/wvw/matches" {
		t.Fatalf("path = %q, want /v2/wvw/matches", gotPath)
	}
	if gotWorld != "1001" {
		t.Fatalf("world query = %q, want %q", gotWorld, "1001")
	}
}

// TestWvwGuildsByRegion exercises `wvw guilds --region`, which appends the
// region as a path segment rather than a query param or ids lookup.
func TestWvwGuildsByRegion(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Write([]byte(`[{"id":1001,"guild_id":"ABC"}]`))
	}))
	defer srv.Close()

	var out bytes.Buffer
	app := &App{Out: &out, BaseURL: srv.URL, Lang: "en", Mode: output.ModeConcise, Limiter: api.NewLimiter(300, 5)}
	root := &cobra.Command{Use: "gw2"}
	root.AddCommand(newWvwCmd(app))
	root.SetArgs([]string{"wvw", "guilds", "--region", "na"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if gotPath != "/v2/wvw/guilds/na" {
		t.Fatalf("path = %q, want /v2/wvw/guilds/na", gotPath)
	}
}

// TestWvwTimersLockout exercises `wvw timers lockout`.
func TestWvwTimersLockout(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Write([]byte(`{"start_time":"2026-01-01T00:00:00Z"}`))
	}))
	defer srv.Close()

	var out bytes.Buffer
	app := &App{Out: &out, BaseURL: srv.URL, Lang: "en", Mode: output.ModeConcise, Limiter: api.NewLimiter(300, 5)}
	root := &cobra.Command{Use: "gw2"}
	root.AddCommand(newWvwCmd(app))
	root.SetArgs([]string{"wvw", "timers", "lockout"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if gotPath != "/v2/wvw/timers/lockout" {
		t.Fatalf("path = %q, want /v2/wvw/timers/lockout", gotPath)
	}
}

// TestAllWvwListResourcesFetch is a table-driven test that exercises every
// registered wvwListResource leaf (ranks/upgrades/objectives/rewardtracks,
// plus abilities again), same role as TestAllCatalogResourcesFetch (data.go)
// and TestAllAccountResourcesFetch (account.go): keeps the coverage
// meta-test (zz_coverage_test.go) green as the registry grows.
func TestAllWvwListResourcesFetch(t *testing.T) {
	for _, res := range wvwListResources {
		res := res
		t.Run(res.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte(`[{"id":1,"name":"Test"}]`))
			}))
			defer srv.Close()

			var out bytes.Buffer
			app := &App{Out: &out, BaseURL: srv.URL, Lang: "en", Mode: output.ModeConcise, Limiter: api.NewLimiter(300, 5)}
			root := &cobra.Command{Use: "gw2"}
			root.AddCommand(newWvwCmd(app))
			root.SetArgs([]string{"wvw", res.name, "--ids", "1"})
			if err := root.Execute(); err != nil {
				t.Fatalf("execute wvw %s: %v", res.name, err)
			}
			if out.String() == "" {
				t.Fatalf("expected non-empty output for wvw %s", res.name)
			}
		})
	}
}

// TestWvwMatchesSubresourcesAndTimers rounds out leaf coverage for
// matches overview/scores/stats (same id/world logic as "matches" itself,
// exercised via --ids here) plus the bare "timers" and "timers
// teamAssignment" leaves not touched by TestWvwMatchesByWorld / TestWvwTimersLockout above.
func TestWvwMatchesSubresourcesAndTimers(t *testing.T) {
	cases := []struct {
		args     []string
		wantPath string
	}{
		{[]string{"wvw", "matches", "overview", "--ids", "1-1"}, "/v2/wvw/matches/overview"},
		{[]string{"wvw", "matches", "scores", "--ids", "1-1"}, "/v2/wvw/matches/scores"},
		{[]string{"wvw", "matches", "stats", "--ids", "1-1"}, "/v2/wvw/matches/stats"},
		{[]string{"wvw", "timers"}, "/v2/wvw/timers"},
		{[]string{"wvw", "timers", "teamAssignment"}, "/v2/wvw/timers/teamAssignment"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			var gotPath string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.Path
				w.Write([]byte(`[{"id":1}]`))
			}))
			defer srv.Close()

			var out bytes.Buffer
			app := &App{Out: &out, BaseURL: srv.URL, Lang: "en", Mode: output.ModeConcise, Limiter: api.NewLimiter(300, 5)}
			root := &cobra.Command{Use: "gw2"}
			root.AddCommand(newWvwCmd(app))
			root.SetArgs(tc.args)
			if err := root.Execute(); err != nil {
				t.Fatalf("execute %v: %v", tc.args, err)
			}
			if gotPath != tc.wantPath {
				t.Fatalf("path = %q, want %q", gotPath, tc.wantPath)
			}
		})
	}
}

// TestWvwMatchesWorldAndIDsConflict verifies --world can't be combined with
// ids (positional or --ids): the API only accepts one selector, so silently
// preferring one would be surprising.
func TestWvwMatchesWorldAndIDsConflict(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("unexpected request for conflicting --world/ids: %s", r.URL.String())
	}))
	defer srv.Close()

	var out bytes.Buffer
	app := &App{Out: &out, BaseURL: srv.URL, Lang: "en", Mode: output.ModeConcise, Limiter: api.NewLimiter(300, 5)}
	root := &cobra.Command{Use: "gw2"}
	root.AddCommand(newWvwCmd(app))
	root.SetArgs([]string{"wvw", "matches", "--world", "1001", "--ids", "1-1"})
	if err := root.Execute(); err == nil {
		t.Fatal("wvw matches --world combined with --ids should error")
	}
}
