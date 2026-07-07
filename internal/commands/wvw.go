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

// wvwListResource is one of the five /v2/wvw/* static-data endpoints that
// follow the exact enumerate-ids -> fetch-by-ids shape as catalogResource
// (data.go): bare invocation lists ids only, --ids/--all fetch full,
// concisely-rendered objects. They're kept in a small local slice (rather
// than folded into data.go's catalogResources) because they live under
// "wvw", not "data", and newCatalogResourceCmd hardcodes a "data " coverage
// prefix that doesn't fit here; newWvwListLeafCmd below mirrors its logic
// with a "wvw " prefix instead. All five are localized (lang-sensitive)
// responses, same as their data.go siblings.
type wvwListResource struct {
	name   string
	path   string
	render func([]json.RawMessage) string
}

var wvwListResources = []wvwListResource{
	{name: "abilities", path: "/v2/wvw/abilities", render: renderNamed},
	{name: "ranks", path: "/v2/wvw/ranks", render: renderNamed},
	{name: "upgrades", path: "/v2/wvw/upgrades", render: renderNamed},
	{name: "objectives", path: "/v2/wvw/objectives", render: renderNamed},
	{name: "rewardtracks", path: "/v2/wvw/rewardtracks", render: renderNamed},
}

// newWvwCmd builds the "wvw" command tree: World vs World static data
// (abilities/ranks/upgrades/objectives/rewardtracks), live match data
// (matches, and its overview/scores/stats sub-resources), map rotation
// timers, and guild claims by region. All public (client(), no auth key
// needed) -- WvW match/objective/timer/guild data is server-wide public
// information, unlike /v2/account/wvw (already covered by the account
// group), which is the one WvW resource scoped to the caller's account.
//
// DEFERRED (not registered here; see task-E-report.md): the three deep
// path-param match-stats endpoints, /v2/wvw/matches/stats/:id/guilds/:guild_id,
// .../teams/:team/top/kdr, .../teams/:team/top/kills.
func newWvwCmd(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "wvw",
		Short: "World vs World matches, objectives, and rankings",
	}
	for _, res := range wvwListResources {
		cmd.AddCommand(newWvwListLeafCmd(app, res))
	}
	cmd.AddCommand(newWvwMatchesCmd(app), newWvwTimersCmd(app), newWvwGuildsCmd(app))
	return cmd
}

// newWvwListLeafCmd mirrors newCatalogResourceCmd's (data.go) bare/--ids/--all
// branch for one of the five wvw list resources; see wvwListResource's doc
// comment for why this isn't reused directly.
func newWvwListLeafCmd(app *App, res wvwListResource) *cobra.Command {
	covID := "wvw " + res.name
	var ids string
	var all bool
	c := &cobra.Command{
		Use:   res.name,
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

// newWvwMatchesCmd builds "matches" and its three sub-resources (overview,
// scores, stats), all sharing the same bare-id-list / by-ids / by-world GET
// shape via newWvwMatchesLeafCmd. Match objects (and their sub-resources)
// are large and don't have a natural named-list concise form, so all four
// leaves render pretty JSON in concise mode (empty concise string).
func newWvwMatchesCmd(app *App) *cobra.Command {
	cmd := newWvwMatchesLeafCmd(app, "matches [ids...]", "/v2/wvw/matches", "wvw matches", true)
	cmd.Short = "WvW match ids, or full match data by id/world"
	cmd.AddCommand(
		newWvwMatchesLeafCmd(app, "overview", "/v2/wvw/matches/overview", "wvw matches overview", false),
		newWvwMatchesLeafCmd(app, "scores", "/v2/wvw/matches/scores", "wvw matches scores", false),
		newWvwMatchesLeafCmd(app, "stats", "/v2/wvw/matches/stats", "wvw matches stats", false),
	)
	return cmd
}

// newWvwMatchesLeafCmd builds one matches leaf (the top-level "matches", or
// one of its overview/scores/stats children -- same endpoint shape, just a
// deeper path). Modes, in priority order:
//   - --world N: fetch the single match containing that world (?world=N).
//   - ids (positional, when allowPositional, and/or --ids): GetByIDs.
//   - bare (neither): the endpoint's own id list only, never auto-dumped.
//
// --world combined with any id is rejected up front: the API only accepts
// one selector at a time, and silently preferring one would be surprising.
// allowPositional is false for the overview/scores/stats children, whose
// brief-specified UX is flags-only (no bare positional ids): a stray
// positional there is rejected by cobra.NoArgs rather than silently ignored.
func newWvwMatchesLeafCmd(app *App, use, path, covID string, allowPositional bool) *cobra.Command {
	var idsFlag string
	var world int
	c := &cobra.Command{
		Use:   use,
		Short: "Fetch " + covID,
		RunE: func(cmd *cobra.Command, args []string) error {
			markCovered(covID)
			var idList []string
			if allowPositional {
				idList = append(idList, args...)
			}
			if idsFlag != "" {
				idList = append(idList, strings.Split(idsFlag, ",")...)
			}
			if world != 0 && len(idList) > 0 {
				return fmt.Errorf("%s: --world cannot be combined with ids/--ids", covID)
			}
			ctx := context.Background()
			client, err := app.client()
			if err != nil {
				return err
			}
			if world != 0 {
				params := url.Values{}
				params.Set("world", strconv.Itoa(world))
				raw, err := client.Get(ctx, path, params)
				if err != nil {
					return err
				}
				return output.Render(app.Out, raw, app.Mode, "")
			}
			if len(idList) == 0 {
				raw, err := client.Get(ctx, path, nil)
				if err != nil {
					return err
				}
				return output.Render(app.Out, raw, app.Mode, conciseIDList(raw))
			}
			items, err := client.GetByIDs(ctx, path, idList, nil)
			if err != nil {
				return err
			}
			merged, _ := json.Marshal(items)
			return output.Render(app.Out, merged, app.Mode, "")
		},
	}
	idsHelp := "comma-separated match ids"
	if allowPositional {
		idsHelp += " (in addition to any positional ids)"
	} else {
		c.Args = cobra.NoArgs
	}
	c.Flags().StringVar(&idsFlag, "ids", "", idsHelp)
	c.Flags().IntVar(&world, "world", 0, "filter to the match containing this world id")
	return c
}

// newWvwTimersCmd builds "timers" and its two children, all plain public
// GETs with no parameters; pretty-JSON rendered since none of the three
// payloads (map rotation schedule, lockout window, team-assignment window)
// have a natural named-list concise form.
func newWvwTimersCmd(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "timers",
		Short: "WvW map rotation and lockout timers",
		RunE:  newWvwSimpleGetRunE(app, "/v2/wvw/timers", "wvw timers"),
	}
	cmd.AddCommand(
		&cobra.Command{
			Use:   "lockout",
			Short: "Fetch wvw timers lockout",
			RunE:  newWvwSimpleGetRunE(app, "/v2/wvw/timers/lockout", "wvw timers lockout"),
		},
		&cobra.Command{
			Use:   "teamAssignment",
			Short: "Fetch wvw timers teamAssignment",
			RunE:  newWvwSimpleGetRunE(app, "/v2/wvw/timers/teamAssignment", "wvw timers teamAssignment"),
		},
	)
	return cmd
}

// newWvwGuildsCmd builds "guilds" ["--region R"]: GET /v2/wvw/guilds, or
// /v2/wvw/guilds/<region> when --region is given. Region is a short string
// code (e.g. "na", "eu"), not a numeric id, so it's path-escaped rather than
// passed through the ids/GetByIDs machinery.
func newWvwGuildsCmd(app *App) *cobra.Command {
	var region string
	c := &cobra.Command{
		Use:   "guilds",
		Short: "WvW guild claims by region",
		RunE: func(cmd *cobra.Command, args []string) error {
			markCovered("wvw guilds")
			ctx := context.Background()
			client, err := app.client()
			if err != nil {
				return err
			}
			path := "/v2/wvw/guilds"
			if region != "" {
				path += "/" + url.PathEscape(region)
			}
			raw, err := client.Get(ctx, path, nil)
			if err != nil {
				return err
			}
			return output.Render(app.Out, raw, app.Mode, "")
		},
	}
	c.Flags().StringVar(&region, "region", "", "filter to one region (e.g. na, eu)")
	return c
}

// newWvwSimpleGetRunE builds a RunE for a plain public GET with no
// parameters and no natural named-list concise form (pretty JSON fallback).
// Shared by all three "timers" leaves.
func newWvwSimpleGetRunE(app *App, path, covID string) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		markCovered(covID)
		ctx := context.Background()
		client, err := app.client()
		if err != nil {
			return err
		}
		raw, err := client.Get(ctx, path, nil)
		if err != nil {
			return err
		}
		return output.Render(app.Out, raw, app.Mode, "")
	}
}
