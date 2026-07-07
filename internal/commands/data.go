package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/howar31/gldwrs2/internal/output"
	"github.com/spf13/cobra"
)

// catalogResource is one static "game data" endpoint following the
// enumerate-ids -> fetch-by-ids pattern.
type catalogResource struct {
	name      string // subcommand name, e.g. "colors"
	path      string // API path, e.g. "/v2/colors"
	localized bool
	render    func([]json.RawMessage) string // optional concise; nil -> JSON fallback
}

// catalogResources is the registry. This plan registers colors; the full
// ~64-row table is a follow-on plan.
var catalogResources = []catalogResource{
	{name: "colors", path: "/v2/colors", localized: true, render: renderColors},
}

func renderColors(items []json.RawMessage) string {
	var b strings.Builder
	for _, it := range items {
		var c struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		}
		if err := json.Unmarshal(it, &c); err != nil {
			continue
		}
		fmt.Fprintf(&b, "%d\t%s\n", c.ID, c.Name)
	}
	return strings.TrimRight(b.String(), "\n")
}

func newDataCmd(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "data",
		Short: "Static game data (items, colors, recipes, ...)",
	}
	for _, res := range catalogResources {
		cmd.AddCommand(newCatalogResourceCmd(app, res))
	}
	return cmd
}

func newCatalogResourceCmd(app *App, res catalogResource) *cobra.Command {
	var ids string
	var all bool
	c := &cobra.Command{
		Use:   res.name,
		Short: "Fetch " + res.name,
		RunE: func(cmd *cobra.Command, args []string) error {
			markCovered("data " + res.name)
			ctx := context.Background()
			client, err := app.client()
			if err != nil {
				return err
			}
			// No ids and not --all: show the id list only (never auto-dump).
			if ids == "" && !all {
				raw, err := client.Get(ctx, res.path, nil)
				if err != nil {
					return err
				}
				return output.Render(app.Out, raw, app.Mode, conciseIDList(raw))
			}
			var idList []string
			if all {
				raw, err := client.Get(ctx, res.path, nil)
				if err != nil {
					return err
				}
				var nums []json.RawMessage
				if err := json.Unmarshal(raw, &nums); err != nil {
					return err
				}
				for _, n := range nums {
					idList = append(idList, strings.Trim(string(n), `"`))
				}
			} else {
				idList = strings.Split(ids, ",")
			}
			items, err := client.GetByIDs(ctx, res.path, idList, nil)
			if err != nil {
				return err
			}
			concise := ""
			if res.render != nil {
				concise = res.render(items)
			}
			merged, _ := json.Marshal(items)
			return output.Render(app.Out, merged, app.Mode, concise)
		},
	}
	c.Flags().StringVar(&ids, "ids", "", "comma-separated ids (omit to list ids)")
	c.Flags().BoolVar(&all, "all", false, "fetch every entry (explicit; may be large)")
	return c
}

func conciseIDList(raw json.RawMessage) string {
	var ids []json.RawMessage
	if err := json.Unmarshal(raw, &ids); err != nil {
		return ""
	}
	parts := make([]string, 0, len(ids))
	for _, id := range ids {
		parts = append(parts, strings.Trim(string(id), `"`))
	}
	return strings.Join(parts, " ")
}

// markCovered is a no-op in production; tests replace it to track coverage.
var markCovered = func(string) {}
