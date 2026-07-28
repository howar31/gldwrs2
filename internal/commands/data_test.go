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

// TestAllCatalogResourcesFetch is a table-driven test that exercises every
// registered catalogResource leaf (bare "data <segments...> --ids 1", or
// "--input 1" for recipes search), asserting the command dispatches to the
// right leaf without error and produces non-empty output. This is what
// keeps the coverage meta-test (zz_coverage_test.go) green as the registry
// grows: every resource is fetched here exactly once.
func TestAllCatalogResourcesFetch(t *testing.T) {
	for _, res := range catalogResources {
		res := res
		name := strings.Join(res.segments, " ")
		t.Run(name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte(`[{"id":1,"name":"Test"}]`))
			}))
			defer srv.Close()

			var out bytes.Buffer
			app := &App{Out: &out, BaseURL: srv.URL, Lang: "en", Mode: output.ModeConcise, Limiter: api.NewLimiter(300, 5)}
			root := &cobra.Command{Use: "gw2"}
			root.AddCommand(newDataCmd(app))

			args := append([]string{"data"}, res.segments...)
			switch name {
			case "recipes search":
				args = append(args, "--input", "1")
			case "adventures leaderboards":
				args = append(args, "1") // positional adventure id, not a by-ids leaf
			case "continents floors":
				args = append(args, "1") // positional continent id, not a by-ids leaf
			case "wizardsvault":
				// bare season fetch; takes no --ids flag
			default:
				args = append(args, "--ids", "1")
			}
			root.SetArgs(args)
			if err := root.Execute(); err != nil {
				t.Fatalf("execute %v: %v", args, err)
			}
			if out.String() == "" {
				t.Fatalf("expected non-empty output for %q", name)
			}
		})
	}
}

// /v2/emblem itself is only a placeholder listing its two child resource
// names; the real data lives at /v2/emblem/foregrounds and
// /v2/emblem/backgrounds (guild emblem layer images).
func TestDataEmblemLayerResources(t *testing.T) {
	for _, sub := range []string{"foregrounds", "backgrounds"} {
		t.Run(sub, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if !strings.HasSuffix(r.URL.Path, "/v2/emblem/"+sub) {
					t.Errorf("path = %s", r.URL.Path)
				}
				w.Write([]byte(`[{"id":7,"layers":["a.png","b.png"]}]`))
			}))
			defer srv.Close()

			var out bytes.Buffer
			app := &App{Out: &out, BaseURL: srv.URL, Lang: "en", Mode: output.ModeConcise, Limiter: api.NewLimiter(300, 5)}
			root := &cobra.Command{Use: "gw2"}
			root.AddCommand(newDataCmd(app))
			root.SetArgs([]string{"data", "emblem", sub, "--ids", "7"})
			if err := root.Execute(); err != nil {
				t.Fatalf("execute: %v", err)
			}
			got := out.String()
			if !strings.Contains(got, "7") || !strings.Contains(got, "2 layers") {
				t.Fatalf("output = %q", got)
			}
		})
	}
}

// Bare `data wizardsvault` fetches /v2/wizardsvault, the current season's
// meta object (not listed in the API root listing, but live and documented).
func TestDataWizardsVaultSeason(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/v2/wizardsvault") {
			t.Errorf("path = %s", r.URL.Path)
		}
		w.Write([]byte(`{"title":"The Only Way Season","start":"2024-12-31T17:00:00Z","end":"2026-09-01T16:00:00Z","listings":[1,2,3],"objectives":[10,11]}`))
	}))
	defer srv.Close()

	var out bytes.Buffer
	app := &App{Out: &out, BaseURL: srv.URL, Lang: "en", Mode: output.ModeConcise, Limiter: api.NewLimiter(300, 5)}
	root := &cobra.Command{Use: "gw2"}
	root.AddCommand(newDataCmd(app))
	root.SetArgs([]string{"data", "wizardsvault"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "The Only Way Season") || !strings.Contains(got, "3 listings") || !strings.Contains(got, "2 objectives") {
		t.Fatalf("output = %q", got)
	}
}

// `data continents floors <continent>` lists the continent's floor ids
// (/v2/continents/:id/floors -- a nested subtree absent from the API root
// listing).
func TestDataContinentsFloorsList(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/v2/continents/1/floors") {
			t.Errorf("path = %s", r.URL.Path)
		}
		w.Write([]byte(`[0,1,2]`))
	}))
	defer srv.Close()

	var out bytes.Buffer
	app := &App{Out: &out, BaseURL: srv.URL, Lang: "en", Mode: output.ModeConcise, Limiter: api.NewLimiter(300, 5)}
	root := &cobra.Command{Use: "gw2"}
	root.AddCommand(newDataCmd(app))
	root.SetArgs([]string{"data", "continents", "floors", "1"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(out.String(), "0 1 2") {
		t.Fatalf("expected floor id list, got %q", out.String())
	}
}

// `data continents floors <continent> <floor>` fetches one floor object,
// which embeds the whole regions/maps subtree; concise output summarizes
// regions and their map counts.
func TestDataContinentsFloorFetch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/v2/continents/1/floors/3") {
			t.Errorf("path = %s", r.URL.Path)
		}
		w.Write([]byte(`{"texture_dims":[81920,114688],"regions":{"8":{"name":"Shiverpeaks","maps":{"26":{},"27":{}}},"4":{"name":"Kryta","maps":{"28":{}}}}}`))
	}))
	defer srv.Close()

	var out bytes.Buffer
	app := &App{Out: &out, BaseURL: srv.URL, Lang: "en", Mode: output.ModeConcise, Limiter: api.NewLimiter(300, 5)}
	root := &cobra.Command{Use: "gw2"}
	root.AddCommand(newDataCmd(app))
	root.SetArgs([]string{"data", "continents", "floors", "1", "3"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "Shiverpeaks") || !strings.Contains(got, "2 maps") || !strings.Contains(got, "Kryta") {
		t.Fatalf("output = %q", got)
	}
}
