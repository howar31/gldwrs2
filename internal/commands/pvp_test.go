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

// TestPvpAmuletsByIDs exercises `pvp amulets --ids`, the shared by-ids leaf
// helper (leaf.go) now also used by pvp's public list resources.
func TestPvpAmuletsByIDs(t *testing.T) {
	var gotPath, gotIDs string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotIDs = r.URL.Query().Get("ids")
		w.Write([]byte(`[{"id":1,"name":"Test Amulet"}]`))
	}))
	defer srv.Close()

	var out bytes.Buffer
	app := &App{Out: &out, BaseURL: srv.URL, Lang: "en", Mode: output.ModeConcise, Limiter: api.NewLimiter(300, 5)}
	root := &cobra.Command{Use: "gw2"}
	root.AddCommand(newPvpCmd(app))
	root.SetArgs([]string{"pvp", "amulets", "--ids", "1"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if gotPath != "/v2/pvp/amulets" {
		t.Fatalf("path = %q, want /v2/pvp/amulets", gotPath)
	}
	if gotIDs != "1" {
		t.Fatalf("ids query = %q, want %q", gotIDs, "1")
	}
	if !strings.Contains(out.String(), "Test Amulet") {
		t.Fatalf("output = %q, want it to contain the amulet name", out.String())
	}
}

// TestPvpAmuletsBareNoDump verifies bare `pvp amulets` (no --ids/--all)
// never dumps every entry: the server must not receive an "ids" param, and
// the output is just the plain id list.
func TestPvpAmuletsBareNoDump(t *testing.T) {
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
	root.AddCommand(newPvpCmd(app))
	root.SetArgs([]string{"pvp", "amulets"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if fetchedIDs {
		t.Fatal("bare `pvp amulets` must not auto-fetch all entries")
	}
	if !strings.Contains(out.String(), "1") {
		t.Fatalf("expected id list, got %q", out.String())
	}
}

// TestPvpSeasonsLeaderboards exercises `pvp seasons leaderboards`, the
// parameterized public child attached to the "seasons" by-ids leaf (dual
// role, mirroring data.go's recipes/recipes-search).
func TestPvpSeasonsLeaderboards(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Write([]byte(`[{"id":"Player.1234","rank":1}]`))
	}))
	defer srv.Close()

	var out bytes.Buffer
	app := &App{Out: &out, BaseURL: srv.URL, Lang: "en", Mode: output.ModeConcise, Limiter: api.NewLimiter(300, 5)}
	root := &cobra.Command{Use: "gw2"}
	root.AddCommand(newPvpCmd(app))
	root.SetArgs([]string{"pvp", "seasons", "leaderboards", "ABC", "ladder", "na"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if gotPath != "/v2/pvp/seasons/ABC/leaderboards/ladder/na" {
		t.Fatalf("path = %q, want /v2/pvp/seasons/ABC/leaderboards/ladder/na", gotPath)
	}
}

// TestPvpStandingsRequiresToken verifies `pvp standings` goes through
// authedClient: with no profile/token configured, it must fail with
// guidance to run `gw2 auth set`, never silently send an anonymous request.
func TestPvpStandingsRequiresToken(t *testing.T) {
	var out bytes.Buffer
	app := testApp(t, &out) // no profile configured -> empty token
	root := &cobra.Command{Use: "gw2"}
	root.AddCommand(newPvpCmd(app))
	root.SetArgs([]string{"pvp", "standings"})
	err := root.Execute()
	if err == nil {
		t.Fatal("pvp standings with no configured key should error")
	}
	if !strings.Contains(err.Error(), "gw2 auth set") {
		t.Fatalf("error = %v, want it to mention gw2 auth set", err)
	}
}

// TestPvpGamesAuthed exercises `pvp games --ids`, the shared by-ids leaf
// helper used in authed mode: the request must carry the bearer token.
func TestPvpGamesAuthed(t *testing.T) {
	var gotPath, gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		w.Write([]byte(`[{"id":"1"}]`))
	}))
	defer srv.Close()

	var out bytes.Buffer
	app := authedTestApp(t, &out, srv)
	root := &cobra.Command{Use: "gw2"}
	root.AddCommand(newPvpCmd(app))
	root.SetArgs([]string{"pvp", "games", "--ids", "1"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if gotPath != "/v2/pvp/games" {
		t.Fatalf("path = %q, want /v2/pvp/games", gotPath)
	}
	if gotAuth != "Bearer TESTTOKEN" {
		t.Fatalf("Authorization header = %q, want %q", gotAuth, "Bearer TESTTOKEN")
	}
}

// TestAllPvpByIDsResourcesFetch is a table-driven test that exercises every
// registered pvpByIDsResource leaf (ranks/heroes/rewardtracks/runes/sigils/
// seasons, plus amulets again), same role as TestAllWvwListResourcesFetch
// (wvw_test.go): keeps the coverage meta-test (zz_coverage_test.go) green as
// the registry grows.
func TestAllPvpByIDsResourcesFetch(t *testing.T) {
	for _, res := range pvpByIDsResources {
		res := res
		t.Run(res.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte(`[{"id":1,"name":"Test"}]`))
			}))
			defer srv.Close()

			var out bytes.Buffer
			app := &App{Out: &out, BaseURL: srv.URL, Lang: "en", Mode: output.ModeConcise, Limiter: api.NewLimiter(300, 5)}
			root := &cobra.Command{Use: "gw2"}
			root.AddCommand(newPvpCmd(app))
			root.SetArgs([]string{"pvp", res.name, "--ids", "1"})
			if err := root.Execute(); err != nil {
				t.Fatalf("execute pvp %s: %v", res.name, err)
			}
			if out.String() == "" {
				t.Fatalf("expected non-empty output for pvp %s", res.name)
			}
		})
	}
}

// TestPvpStatsAuthed rounds out leaf coverage for `pvp stats`, not touched
// by the tests above, and exercises renderPvpStats' concise summary line.
func TestPvpStatsAuthed(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Write([]byte(`{"pvp_rank":43,"aggregate":{"wins":45,"losses":30,"desertions":0,"byes":0,"forfeits":0}}`))
	}))
	defer srv.Close()

	var out bytes.Buffer
	app := authedTestApp(t, &out, srv)
	root := &cobra.Command{Use: "gw2"}
	root.AddCommand(newPvpCmd(app))
	root.SetArgs([]string{"pvp", "stats"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if gotAuth != "Bearer TESTTOKEN" {
		t.Fatalf("Authorization header = %q, want %q", gotAuth, "Bearer TESTTOKEN")
	}
	if !strings.Contains(out.String(), "43") {
		t.Fatalf("output = %q, want it to contain the pvp_rank", out.String())
	}
}
