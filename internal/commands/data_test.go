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
