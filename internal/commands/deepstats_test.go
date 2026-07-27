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

func publicTestApp(out *bytes.Buffer, srv *httptest.Server) *App {
	return &App{Out: out, BaseURL: srv.URL, Lang: "en", Mode: output.ModeConcise, Limiter: api.NewLimiter(300, 5)}
}

// TestWvwStatsGuilds covers the per-match per-guild stats path, including
// path-escaping of both user-supplied segments.
func TestWvwStatsGuilds(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.EscapedPath()
		w.Write([]byte(`{"id":"1-1"}`))
	}))
	defer srv.Close()

	var out bytes.Buffer
	root := &cobra.Command{Use: "gw2"}
	root.AddCommand(newWvwCmd(publicTestApp(&out, srv)))
	root.SetArgs([]string{"wvw", "matches", "stats", "guilds", "1-1", "AAAA-BBBB"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if gotPath != "/v2/wvw/matches/stats/1-1/guilds/AAAA-BBBB" {
		t.Fatalf("path = %s", gotPath)
	}
}

// TestWvwStatsTop covers the per-team top board path and its up-front
// vocabulary validation (no network call on a bad team/board).
func TestWvwStatsTop(t *testing.T) {
	var gotPath string
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		gotPath = r.URL.EscapedPath()
		w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	var out bytes.Buffer
	run := func(args ...string) error {
		root := &cobra.Command{Use: "gw2", SilenceUsage: true, SilenceErrors: true}
		root.AddCommand(newWvwCmd(publicTestApp(&out, srv)))
		root.SetArgs(args)
		return root.Execute()
	}

	if err := run("wvw", "matches", "stats", "top", "1-1", "red", "kdr"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if gotPath != "/v2/wvw/matches/stats/1-1/teams/red/top/kdr" {
		t.Fatalf("path = %s", gotPath)
	}

	if err := run("wvw", "matches", "stats", "top", "1-1", "purple", "kdr"); err == nil || !strings.Contains(err.Error(), "unknown team") {
		t.Fatalf("bad team should error before network, got: %v", err)
	}
	if err := run("wvw", "matches", "stats", "top", "1-1", "red", "deaths"); err == nil || !strings.Contains(err.Error(), "unknown board") {
		t.Fatalf("bad board should error before network, got: %v", err)
	}
	if calls != 1 {
		t.Fatalf("validation failures must not hit the network; calls = %d, want 1", calls)
	}
}

// TestAdventuresLeaderboards covers both arg forms of the (upstream-
// disabled, httptest-only) adventures leaderboards command, including
// path-escaping of the adventure id and the both-or-neither board/region
// rule.
func TestAdventuresLeaderboards(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.EscapedPath()
		w.Write([]byte(`["board-a"]`))
	}))
	defer srv.Close()

	var out bytes.Buffer
	run := func(args ...string) error {
		root := &cobra.Command{Use: "gw2", SilenceUsage: true, SilenceErrors: true}
		root.AddCommand(newDataCmd(publicTestApp(&out, srv)))
		root.SetArgs(args)
		return root.Execute()
	}

	if err := run("data", "adventures", "leaderboards", "A B"); err != nil {
		t.Fatalf("1-arg form: %v", err)
	}
	if gotPath != "/v2/adventures/A%20B/leaderboards" {
		t.Fatalf("1-arg path = %s", gotPath)
	}

	if err := run("data", "adventures", "leaderboards", "A B", "board-a", "na"); err != nil {
		t.Fatalf("3-arg form: %v", err)
	}
	if gotPath != "/v2/adventures/A%20B/leaderboards/board-a/na" {
		t.Fatalf("3-arg path = %s", gotPath)
	}

	if err := run("data", "adventures", "leaderboards", "A B", "board-a"); err == nil {
		t.Fatal("2-arg form should error (board without region)")
	}
}
