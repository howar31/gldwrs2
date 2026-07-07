package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/howar31/gldwrs2/internal/output"
	"github.com/spf13/cobra"
)

// accountResource is one authenticated /v2/account/* endpoint. Unlike
// catalogResource (data.go), every account endpoint is a whole-account
// single GET: there is no ids/--all enumerate-then-fetch machinery, since
// these resources belong to one account and aren't looked up by id.
//
// segments is the command path under "account" (e.g. []string{"home",
// "cats"} for `gw2 account home cats`); resources sharing a segment prefix
// share one parent command, same convention as catalogResource.segments.
type accountResource struct {
	segments []string
	path     string                       // API path, e.g. "/v2/account/wallet"
	render   func(json.RawMessage) string // concise renderer; nil -> pretty-JSON fallback
}

// accountResources is the full /v2/account/* registry (the bare /v2/account
// itself is wired directly onto the "account" command in newAccountCmd, not
// listed here, since its segments would collide with the root command
// name). Where a response is a bare array of ids (int or string), render is
// conciseIDList, reused as-is from data.go: it works directly off the raw
// response body, and account leaves never fetch by-ids so there's no
// []json.RawMessage of already-parsed items lying around to reuse
// data.go's renderNamed-style renderers against. Endpoints whose shape is
// an object, or an array of objects with no bespoke renderer named in the
// design brief, fall back to pretty JSON (render left nil).
var accountResources = []accountResource{
	{segments: []string{"achievements"}, path: "/v2/account/achievements", render: renderAccountAchievements},
	{segments: []string{"bank"}, path: "/v2/account/bank"},
	{segments: []string{"buildstorage"}, path: "/v2/account/buildstorage"},
	{segments: []string{"dailycrafting"}, path: "/v2/account/dailycrafting", render: conciseIDList},
	{segments: []string{"dungeons"}, path: "/v2/account/dungeons", render: conciseIDList},
	{segments: []string{"dyes"}, path: "/v2/account/dyes", render: conciseIDList},
	{segments: []string{"emotes"}, path: "/v2/account/emotes", render: conciseIDList},
	{segments: []string{"finishers"}, path: "/v2/account/finishers"},
	{segments: []string{"gliders"}, path: "/v2/account/gliders", render: conciseIDList},
	{segments: []string{"home", "cats"}, path: "/v2/account/home/cats"},
	{segments: []string{"home", "nodes"}, path: "/v2/account/home/nodes", render: conciseIDList},
	{segments: []string{"homestead", "decorations"}, path: "/v2/account/homestead/decorations"},
	{segments: []string{"homestead", "glyphs"}, path: "/v2/account/homestead/glyphs"},
	{segments: []string{"inventory"}, path: "/v2/account/inventory"},
	{segments: []string{"jadebots"}, path: "/v2/account/jadebots", render: conciseIDList},
	{segments: []string{"legendaryarmory"}, path: "/v2/account/legendaryarmory"},
	{segments: []string{"luck"}, path: "/v2/account/luck"},
	{segments: []string{"mail"}, path: "/v2/account/mail"},
	{segments: []string{"mailcarriers"}, path: "/v2/account/mailcarriers", render: conciseIDList},
	{segments: []string{"mapchests"}, path: "/v2/account/mapchests", render: conciseIDList},
	{segments: []string{"masteries"}, path: "/v2/account/masteries"},
	{segments: []string{"mastery", "points"}, path: "/v2/account/mastery/points"},
	{segments: []string{"materials"}, path: "/v2/account/materials", render: renderMaterials},
	{segments: []string{"minis"}, path: "/v2/account/minis", render: conciseIDList},
	{segments: []string{"mounts", "skins"}, path: "/v2/account/mounts/skins", render: conciseIDList},
	{segments: []string{"mounts", "types"}, path: "/v2/account/mounts/types", render: conciseIDList},
	{segments: []string{"novelties"}, path: "/v2/account/novelties", render: conciseIDList},
	{segments: []string{"outfits"}, path: "/v2/account/outfits", render: conciseIDList},
	{segments: []string{"progression"}, path: "/v2/account/progression"},
	{segments: []string{"pvp", "heroes"}, path: "/v2/account/pvp/heroes", render: conciseIDList},
	{segments: []string{"raids"}, path: "/v2/account/raids", render: conciseIDList},
	{segments: []string{"recipes"}, path: "/v2/account/recipes", render: conciseIDList},
	{segments: []string{"skiffs"}, path: "/v2/account/skiffs", render: conciseIDList},
	{segments: []string{"skins"}, path: "/v2/account/skins", render: conciseIDList},
	{segments: []string{"titles"}, path: "/v2/account/titles", render: conciseIDList},
	{segments: []string{"wallet"}, path: "/v2/account/wallet", render: renderWallet},
	{segments: []string{"wizardsvault", "daily"}, path: "/v2/account/wizardsvault/daily"},
	{segments: []string{"wizardsvault", "weekly"}, path: "/v2/account/wizardsvault/weekly"},
	{segments: []string{"wizardsvault", "special"}, path: "/v2/account/wizardsvault/special"},
	{segments: []string{"wizardsvault", "listings"}, path: "/v2/account/wizardsvault/listings"},
	{segments: []string{"worldbosses"}, path: "/v2/account/worldbosses", render: conciseIDList},
	{segments: []string{"wvw"}, path: "/v2/account/wvw"},
}

// newAccountCmd builds the "account" command tree: the bare command fetches
// /v2/account itself, and every accountResources entry is registered as a
// (possibly nested) subcommand, sharing addToTree with data.go.
func newAccountCmd(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "account",
		Short: "Your account assets and unlocks",
		RunE:  accountLeafRunE(app, "/v2/account", "account", renderAccount),
	}
	for _, res := range accountResources {
		addToTree(cmd, res.segments, func(use string) *cobra.Command {
			return newAccountLeafCmd(app, res, use)
		})
	}
	return cmd
}

func newAccountLeafCmd(app *App, res accountResource, use string) *cobra.Command {
	covID := "account " + strings.Join(res.segments, " ")
	return &cobra.Command{
		Use:   use,
		Short: "Fetch " + covID,
		RunE:  accountLeafRunE(app, res.path, covID, res.render),
	}
}

// accountLeafRunE builds the RunE for one account leaf. Every account
// endpoint is a single whole-account GET (no --ids/--all): it requires
// authedClient (erroring clearly if no key is configured, instead of
// sending an anonymous request that would just 401/403), fetches path with
// no params, and renders concisely via render (nil falls back to pretty
// JSON, same as output.Render's default behavior for an empty string).
func accountLeafRunE(app *App, path, covID string, render func(json.RawMessage) string) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		markCovered(covID)
		ctx := context.Background()
		client, err := app.authedClient()
		if err != nil {
			return err
		}
		raw, err := client.Get(ctx, path, nil)
		if err != nil {
			return err
		}
		concise := ""
		if render != nil {
			concise = render(raw)
		}
		return output.Render(app.Out, raw, app.Mode, concise)
	}
}

// renderAccount is the bare `gw2 account` concise renderer: the account
// info object isn't a list, so this prints a handful of headline fields
// rather than reusing an id-list/named-list renderer. Falls back to pretty
// JSON (by returning "") if the object doesn't even have a name.
func renderAccount(raw json.RawMessage) string {
	var v struct {
		Name    string   `json:"name"`
		World   int      `json:"world"`
		Created string   `json:"created"`
		Access  []string `json:"access"`
	}
	if err := json.Unmarshal(raw, &v); err != nil || v.Name == "" {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "name\t%s\n", v.Name)
	if v.World != 0 {
		fmt.Fprintf(&b, "world\t%d\n", v.World)
	}
	if v.Created != "" {
		fmt.Fprintf(&b, "created\t%s\n", v.Created)
	}
	if len(v.Access) > 0 {
		fmt.Fprintf(&b, "access\t%s\n", strings.Join(v.Access, ","))
	}
	return strings.TrimRight(b.String(), "\n")
}

// renderAccountAchievements renders /v2/account/achievements, an array of
// {id,current,max,done}, as "id current/max" with a trailing "done" marker.
func renderAccountAchievements(raw json.RawMessage) string {
	var items []struct {
		ID      int  `json:"id"`
		Current int  `json:"current"`
		Max     int  `json:"max"`
		Done    bool `json:"done"`
	}
	if err := json.Unmarshal(raw, &items); err != nil {
		return ""
	}
	var b strings.Builder
	for _, it := range items {
		status := ""
		if it.Done {
			status = " done"
		}
		fmt.Fprintf(&b, "%d\t%d/%d%s\n", it.ID, it.Current, it.Max, status)
	}
	return strings.TrimRight(b.String(), "\n")
}

// renderMaterials renders /v2/account/materials, an array of
// {id,category,count}, as "id count (catN)".
func renderMaterials(raw json.RawMessage) string {
	var items []struct {
		ID       int `json:"id"`
		Category int `json:"category"`
		Count    int `json:"count"`
	}
	if err := json.Unmarshal(raw, &items); err != nil {
		return ""
	}
	var b strings.Builder
	for _, it := range items {
		fmt.Fprintf(&b, "%d\t%d (cat%d)\n", it.ID, it.Count, it.Category)
	}
	return strings.TrimRight(b.String(), "\n")
}

// renderWallet renders /v2/account/wallet, an array of {id,value}, as
// "id value" (currency id, currency amount).
func renderWallet(raw json.RawMessage) string {
	var items []struct {
		ID    int `json:"id"`
		Value int `json:"value"`
	}
	if err := json.Unmarshal(raw, &items); err != nil {
		return ""
	}
	var b strings.Builder
	for _, it := range items {
		fmt.Fprintf(&b, "%d\t%d\n", it.ID, it.Value)
	}
	return strings.TrimRight(b.String(), "\n")
}
