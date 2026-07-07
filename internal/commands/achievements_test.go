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

// TestAchievementsByIDs exercises `achievements --ids`, the shared by-ids
// leaf helper (leaf.go) at the achievements group's own dual-role root.
func TestAchievementsByIDs(t *testing.T) {
	var gotPath, gotIDs string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotIDs = r.URL.Query().Get("ids")
		w.Write([]byte(`[{"id":1,"name":"Test Achievement"}]`))
	}))
	defer srv.Close()

	var out bytes.Buffer
	app := &App{Out: &out, BaseURL: srv.URL, Lang: "en", Mode: output.ModeConcise, Limiter: api.NewLimiter(300, 5)}
	root := &cobra.Command{Use: "gw2"}
	root.AddCommand(newAchievementsCmd(app))
	root.SetArgs([]string{"achievements", "--ids", "1"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if gotPath != "/v2/achievements" {
		t.Fatalf("path = %q, want /v2/achievements", gotPath)
	}
	if gotIDs != "1" {
		t.Fatalf("ids query = %q, want %q", gotIDs, "1")
	}
	if !strings.Contains(out.String(), "Test Achievement") {
		t.Fatalf("output = %q, want it to contain the achievement name", out.String())
	}
}

// TestAchievementsBareNoDump verifies bare `achievements` (no --ids/--all)
// never dumps every entry: the server must not receive an "ids" param, and
// the output is just the plain id list.
func TestAchievementsBareNoDump(t *testing.T) {
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
	root.AddCommand(newAchievementsCmd(app))
	root.SetArgs([]string{"achievements"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if fetchedIDs {
		t.Fatal("bare `achievements` must not auto-fetch all entries")
	}
	if !strings.Contains(out.String(), "1") {
		t.Fatalf("expected id list, got %q", out.String())
	}
}

// TestAchievementsCategories exercises `achievements categories --ids`, the
// by-ids child attached to the dual-role achievements root.
func TestAchievementsCategories(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Write([]byte(`[{"id":1,"name":"Test Category"}]`))
	}))
	defer srv.Close()

	var out bytes.Buffer
	app := &App{Out: &out, BaseURL: srv.URL, Lang: "en", Mode: output.ModeConcise, Limiter: api.NewLimiter(300, 5)}
	root := &cobra.Command{Use: "gw2"}
	root.AddCommand(newAchievementsCmd(app))
	root.SetArgs([]string{"achievements", "categories", "--ids", "1"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if gotPath != "/v2/achievements/categories" {
		t.Fatalf("path = %q, want /v2/achievements/categories", gotPath)
	}
	if !strings.Contains(out.String(), "Test Category") {
		t.Fatalf("output = %q, want it to contain the category name", out.String())
	}
}

// TestAchievementsGroups exercises `achievements groups --ids`, rounding out
// leaf coverage for the by-ids child whose ids are GUIDs rather than ints.
func TestAchievementsGroups(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Write([]byte(`[{"id":"A2879737-2CB0-4C64-8674-53287AAAF181","name":"Test Group"}]`))
	}))
	defer srv.Close()

	var out bytes.Buffer
	app := &App{Out: &out, BaseURL: srv.URL, Lang: "en", Mode: output.ModeConcise, Limiter: api.NewLimiter(300, 5)}
	root := &cobra.Command{Use: "gw2"}
	root.AddCommand(newAchievementsCmd(app))
	root.SetArgs([]string{"achievements", "groups", "--ids", "A2879737-2CB0-4C64-8674-53287AAAF181"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if gotPath != "/v2/achievements/groups" {
		t.Fatalf("path = %q, want /v2/achievements/groups", gotPath)
	}
	if !strings.Contains(out.String(), "Test Group") {
		t.Fatalf("output = %q, want it to contain the group name", out.String())
	}
}

// TestAchievementsDaily exercises `achievements daily`, a plain single GET
// (not a by-ids endpoint) that is also the parent of "tomorrow".
func TestAchievementsDaily(t *testing.T) {
	var gotPath, gotIDs string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotIDs = r.URL.Query().Get("ids")
		w.Write([]byte(`{"pve":[{"id":1}],"pvp":[],"wvw":[],"fractals":[],"special":[]}`))
	}))
	defer srv.Close()

	var out bytes.Buffer
	app := &App{Out: &out, BaseURL: srv.URL, Lang: "en", Mode: output.ModeConcise, Limiter: api.NewLimiter(300, 5)}
	root := &cobra.Command{Use: "gw2"}
	root.AddCommand(newAchievementsCmd(app))
	root.SetArgs([]string{"achievements", "daily"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if gotPath != "/v2/achievements/daily" {
		t.Fatalf("path = %q, want /v2/achievements/daily", gotPath)
	}
	if gotIDs != "" {
		t.Fatalf("achievements daily must not send an ids param, got %q", gotIDs)
	}
	if out.String() == "" {
		t.Fatal("expected non-empty output for achievements daily")
	}
}

// TestAchievementsDailyTomorrow exercises `achievements daily tomorrow`, the
// child of the dual-role "daily" command.
func TestAchievementsDailyTomorrow(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Write([]byte(`{"pve":[],"pvp":[],"wvw":[],"fractals":[],"special":[]}`))
	}))
	defer srv.Close()

	var out bytes.Buffer
	app := &App{Out: &out, BaseURL: srv.URL, Lang: "en", Mode: output.ModeConcise, Limiter: api.NewLimiter(300, 5)}
	root := &cobra.Command{Use: "gw2"}
	root.AddCommand(newAchievementsCmd(app))
	root.SetArgs([]string{"achievements", "daily", "tomorrow"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if gotPath != "/v2/achievements/daily/tomorrow" {
		t.Fatalf("path = %q, want /v2/achievements/daily/tomorrow", gotPath)
	}
	if out.String() == "" {
		t.Fatal("expected non-empty output for achievements daily tomorrow")
	}
}
