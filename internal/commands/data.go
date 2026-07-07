package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/howar31/gldwrs2/internal/output"
	"github.com/spf13/cobra"
)

// catalogResource is one static "game data" endpoint following the
// enumerate-ids -> fetch-by-ids pattern. segments is the full command path
// under "data" (e.g. []string{"mounts", "skins"} for `gw2 data mounts
// skins`); resources sharing a segment prefix share one parent command.
type catalogResource struct {
	segments  []string // command path under "data", e.g. ["mounts","skins"]
	path      string   // API path, e.g. "/v2/mounts/skins"
	localized bool
	render    func([]json.RawMessage) string // optional concise; nil -> JSON fallback

	// buildCmd overrides the standard enumerate/by-ids leaf command for
	// resources that don't follow that pattern (e.g. recipes search). When
	// nil, newCatalogResourceCmd is used.
	buildCmd func(app *App, res catalogResource) *cobra.Command
}

// catalogResources is the full static-data catalog registry. Nested
// resources (mounts, homestead, backstory, home, wizardsvault, stories) rely
// on their shared parent already existing earlier in this slice -- see
// newDataCmd / addCatalogResource.
//
// DEFERRED (not registered here, need path parameters): /v2/adventures/:id/
// leaderboards/:board/:region and /v2/homestead/decorations/categories/:id.
// Plain /v2/adventures (id list) and the categories list are registered.
var catalogResources = []catalogResource{
	{segments: []string{"items"}, path: "/v2/items", localized: true, render: renderNamed},
	{segments: []string{"itemstats"}, path: "/v2/itemstats", localized: true, render: renderNamed},
	{segments: []string{"skins"}, path: "/v2/skins", localized: true, render: renderNamed},
	{segments: []string{"colors"}, path: "/v2/colors", localized: true, render: renderColors},
	{segments: []string{"recipes"}, path: "/v2/recipes", localized: false, render: renderNamed},
	{segments: []string{"skills"}, path: "/v2/skills", localized: true, render: renderNamed},
	{segments: []string{"traits"}, path: "/v2/traits", localized: true, render: renderNamed},
	{segments: []string{"specializations"}, path: "/v2/specializations", localized: true, render: renderNamed},
	{segments: []string{"professions"}, path: "/v2/professions", localized: true, render: renderNamed},
	{segments: []string{"races"}, path: "/v2/races", localized: true, render: renderNamed},
	{segments: []string{"pets"}, path: "/v2/pets", localized: true, render: renderNamed},
	{segments: []string{"legends"}, path: "/v2/legends", localized: false, render: renderNamed},
	{segments: []string{"masteries"}, path: "/v2/masteries", localized: true, render: renderNamed},
	{segments: []string{"materials"}, path: "/v2/materials", localized: true, render: nil},
	{segments: []string{"currencies"}, path: "/v2/currencies", localized: true, render: renderNamed},
	{segments: []string{"titles"}, path: "/v2/titles", localized: true, render: renderNamed},
	{segments: []string{"minis"}, path: "/v2/minis", localized: true, render: renderNamed},
	{segments: []string{"finishers"}, path: "/v2/finishers", localized: true, render: renderNamed},
	{segments: []string{"emotes"}, path: "/v2/emotes", localized: true, render: renderNamed},
	{segments: []string{"outfits"}, path: "/v2/outfits", localized: true, render: renderNamed},
	{segments: []string{"gliders"}, path: "/v2/gliders", localized: true, render: renderNamed},
	{segments: []string{"mailcarriers"}, path: "/v2/mailcarriers", localized: true, render: renderNamed},
	{segments: []string{"novelties"}, path: "/v2/novelties", localized: true, render: renderNamed},
	{segments: []string{"quaggans"}, path: "/v2/quaggans", localized: false, render: nil},
	{segments: []string{"quests"}, path: "/v2/quests", localized: true, render: renderNamed},
	{segments: []string{"raids"}, path: "/v2/raids", localized: true, render: renderNamed},
	{segments: []string{"dungeons"}, path: "/v2/dungeons", localized: true, render: renderNamed},
	{segments: []string{"dailycrafting"}, path: "/v2/dailycrafting", localized: true, render: renderNamed},
	{segments: []string{"worldbosses"}, path: "/v2/worldbosses", localized: true, render: renderNamed},
	{segments: []string{"worlds"}, path: "/v2/worlds", localized: true, render: renderNamed},
	{segments: []string{"maps"}, path: "/v2/maps", localized: true, render: renderNamed},
	{segments: []string{"continents"}, path: "/v2/continents", localized: true, render: renderNamed},
	{segments: []string{"mapchests"}, path: "/v2/mapchests", localized: true, render: renderNamed},
	{segments: []string{"jadebots"}, path: "/v2/jadebots", localized: true, render: renderNamed},
	{segments: []string{"skiffs"}, path: "/v2/skiffs", localized: true, render: renderNamed},
	{segments: []string{"legendaryarmory"}, path: "/v2/legendaryarmory", localized: false, render: renderNamed},
	{segments: []string{"files"}, path: "/v2/files", localized: false, render: nil},
	{segments: []string{"logos"}, path: "/v2/logos", localized: false, render: nil},
	{segments: []string{"emblem"}, path: "/v2/emblem", localized: false, render: renderNamed},
	{segments: []string{"vendors"}, path: "/v2/vendors", localized: true, render: renderNamed},
	{segments: []string{"events"}, path: "/v2/events", localized: true, render: renderNamed},
	{segments: []string{"events-state"}, path: "/v2/events-state", localized: false, render: renderNamed},
	{segments: []string{"gemstore-catalog"}, path: "/v2/gemstore/catalog", localized: true, render: renderNamed},
	// Plain id list only; leaderboards need path params and are deferred.
	{segments: []string{"adventures"}, path: "/v2/adventures", localized: false, render: renderNamed},

	// Nested resources. Order matters: a resource that is itself both an
	// endpoint and a parent (stories, homestead decorations) must appear
	// before its children so the tree builder finds it as an existing leaf
	// to descend into, rather than creating a duplicate.
	{segments: []string{"backstory", "answers"}, path: "/v2/backstory/answers", localized: true, render: renderNamed},
	{segments: []string{"backstory", "questions"}, path: "/v2/backstory/questions", localized: true, render: renderNamed},
	{segments: []string{"home", "cats"}, path: "/v2/home/cats", localized: true, render: renderNamed},
	{segments: []string{"home", "nodes"}, path: "/v2/home/nodes", localized: true, render: renderNamed},
	{segments: []string{"homestead", "decorations"}, path: "/v2/homestead/decorations", localized: true, render: renderNamed},
	// DEFERRED: /v2/homestead/decorations/categories/:id needs a path param.
	{segments: []string{"homestead", "decorations", "categories"}, path: "/v2/homestead/decorations/categories", localized: true, render: renderNamed},
	{segments: []string{"homestead", "glyphs"}, path: "/v2/homestead/glyphs", localized: true, render: renderNamed},
	{segments: []string{"mounts", "skins"}, path: "/v2/mounts/skins", localized: true, render: renderNamed},
	{segments: []string{"mounts", "types"}, path: "/v2/mounts/types", localized: true, render: renderNamed},
	{segments: []string{"stories"}, path: "/v2/stories", localized: true, render: renderNamed},
	{segments: []string{"stories", "seasons"}, path: "/v2/stories/seasons", localized: true, render: renderNamed},
	{segments: []string{"wizardsvault", "listings"}, path: "/v2/wizardsvault/listings", localized: true, render: renderNamed},
	{segments: []string{"wizardsvault", "objectives"}, path: "/v2/wizardsvault/objectives", localized: true, render: renderNamed},
	{segments: []string{"recipes", "search"}, path: "/v2/recipes/search", localized: false, buildCmd: newRecipesSearchCmd},
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

// renderNamed is the generic concise renderer for resources whose objects
// carry an id plus a human-readable name or title. id is `any` because some
// endpoints use int ids and others use string ids; fmt.Sprint normalizes
// either for display. Objects with neither name nor title fall back to
// printing just the id (still better than nothing in concise mode).
func renderNamed(items []json.RawMessage) string {
	var b strings.Builder
	for _, it := range items {
		var v struct {
			ID    any    `json:"id"`
			Name  string `json:"name"`
			Title string `json:"title"`
		}
		if err := json.Unmarshal(it, &v); err != nil {
			continue
		}
		label := v.Name
		if label == "" {
			label = v.Title
		}
		if label == "" {
			fmt.Fprintf(&b, "%s\n", fmt.Sprint(v.ID))
			continue
		}
		fmt.Fprintf(&b, "%s\t%s\n", fmt.Sprint(v.ID), label)
	}
	return strings.TrimRight(b.String(), "\n")
}

// newDataCmd builds the "data" command tree from catalogResources, grouping
// resources by shared segment prefixes (e.g. "mounts skins"/"mounts types"
// share one "mounts" parent command).
func newDataCmd(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "data",
		Short: "Static game data (items, colors, recipes, ...)",
	}
	for _, res := range catalogResources {
		addCatalogResource(cmd, app, res)
	}
	return cmd
}

// addCatalogResource walks res.segments from root, creating intermediate
// group commands as needed (reusing one already created for a sibling
// resource), and attaches the leaf command at the final segment.
func addCatalogResource(root *cobra.Command, app *App, res catalogResource) {
	cur := root
	for i, seg := range res.segments {
		if i == len(res.segments)-1 {
			cur.AddCommand(newCatalogLeafCmd(app, res, seg))
			return
		}
		child := findSubcommand(cur, seg)
		if child == nil {
			child = &cobra.Command{
				Use:   seg,
				Short: "Subcommands for " + seg,
			}
			cur.AddCommand(child)
		}
		cur = child
	}
}

func findSubcommand(parent *cobra.Command, use string) *cobra.Command {
	for _, c := range parent.Commands() {
		if c.Name() == use {
			return c
		}
	}
	return nil
}

func newCatalogLeafCmd(app *App, res catalogResource, use string) *cobra.Command {
	if res.buildCmd != nil {
		return res.buildCmd(app, res)
	}
	return newCatalogResourceCmd(app, res, use)
}

func newCatalogResourceCmd(app *App, res catalogResource, use string) *cobra.Command {
	covID := "data " + strings.Join(res.segments, " ")
	var ids string
	var all bool
	c := &cobra.Command{
		Use:   use,
		Short: "Fetch " + covID,
		RunE: func(cmd *cobra.Command, args []string) error {
			markCovered(covID)
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

// newRecipesSearchCmd builds `data recipes search`, which is not a by-ids
// endpoint: it takes ?input=<itemid> or ?output=<itemid> and returns a
// plain list of recipe ids.
func newRecipesSearchCmd(app *App, res catalogResource) *cobra.Command {
	covID := "data " + strings.Join(res.segments, " ")
	var inputID, outputID int
	c := &cobra.Command{
		Use:   "search",
		Short: "Search recipes by input or output item id",
		RunE: func(cmd *cobra.Command, args []string) error {
			markCovered(covID)
			if inputID == 0 && outputID == 0 {
				return fmt.Errorf("recipes search requires --input or --output")
			}
			ctx := context.Background()
			client, err := app.client()
			if err != nil {
				return err
			}
			params := url.Values{}
			if inputID != 0 {
				params.Set("input", strconv.Itoa(inputID))
			}
			if outputID != 0 {
				params.Set("output", strconv.Itoa(outputID))
			}
			raw, err := client.Get(ctx, res.path, params)
			if err != nil {
				return err
			}
			return output.Render(app.Out, raw, app.Mode, conciseIDList(raw))
		},
	}
	c.Flags().IntVar(&inputID, "input", 0, "filter recipes producible from this item id")
	c.Flags().IntVar(&outputID, "output", 0, "filter recipes that produce this item id")
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
