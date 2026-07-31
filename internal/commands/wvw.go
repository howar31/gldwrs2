package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"

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
// The deep path-param match-stats endpoints
// (/v2/wvw/matches/stats/:id/guilds/:guild_id and
// .../teams/:team/top/{kdr,kills}) are covered by the stats leaf's
// "guilds"/"top" children below.
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

// newWvwListLeafCmd delegates to the shared by-ids leaf helper (leaf.go),
// which reproduces newCatalogResourceCmd's (data.go) bare/--ids/--all
// branch; see wvwListResource's doc comment for why these five resources
// live here rather than in data.go's catalogResources.
func newWvwListLeafCmd(app *App, res wvwListResource) *cobra.Command {
	covID := "wvw " + res.name
	return newByIDsListCmd(app, res.name, "Fetch "+covID, res.path, covID, res.render, false, false)
}

// newWvwMatchesCmd builds "matches" and its three sub-resources (overview,
// scores, stats), all sharing the same bare-id-list / by-ids / by-world GET
// shape via newWvwMatchesLeafCmd. Match objects (and their sub-resources)
// are large and don't have a natural named-list concise form, so all four
// leaves render pretty JSON in concise mode (empty concise string).
func newWvwMatchesCmd(app *App) *cobra.Command {
	cmd := newWvwMatchesLeafCmd(app, "matches [ids...]", "/v2/wvw/matches", "wvw matches", true)
	cmd.Short = "WvW match ids, or full match data by id/world"
	stats := newWvwMatchesLeafCmd(app, "stats", "/v2/wvw/matches/stats", "wvw matches stats", false)
	// stats plays a dual role (same pattern as pvp seasons/leaderboards):
	// it is itself a by-ids leaf AND the parent of the deep per-match
	// analytics endpoints below.
	stats.AddCommand(newWvwStatsGuildsCmd(app), newWvwStatsTopCmd(app))
	cmd.AddCommand(
		newWvwMatchesLeafCmd(app, "overview", "/v2/wvw/matches/overview", "wvw matches overview", false),
		newWvwMatchesLeafCmd(app, "scores", "/v2/wvw/matches/scores", "wvw matches scores", false),
		stats,
	)
	return cmd
}

// wvwTeams is the fixed team-color vocabulary for the per-team top boards,
// and wvwTopBoards the two boards the API exposes; both are validated
// before any network call.
var (
	wvwTeams     = map[string]bool{"red": true, "blue": true, "green": true}
	wvwTopBoards = map[string]bool{"kdr": true, "kills": true}
)

// newWvwStatsGuildsCmd builds `wvw matches stats guilds <matchId>
// <guildId>` -> /v2/wvw/matches/stats/:id/guilds/:guild_id: one guild's
// kill/death stats within one match. Public. Both user-supplied path
// segments are path-escaped.
func newWvwStatsGuildsCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "guilds <matchId> <guildId>",
		Short: "One guild's stats within a WvW match",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			markCovered("wvw matches stats guilds")
			ctx := context.Background()
			client, err := app.client()
			if err != nil {
				return err
			}
			path := "/v2/wvw/matches/stats/" + url.PathEscape(args[0]) + "/guilds/" + url.PathEscape(args[1])
			raw, err := client.Get(ctx, path, nil)
			if err != nil {
				return err
			}
			return output.Render(app.Out, raw, app.Mode, "")
		},
	}
}

// newWvwStatsTopCmd builds `wvw matches stats top <matchId> <team>
// <kdr|kills>` -> /v2/wvw/matches/stats/:id/teams/:team/top/:board: the
// top-guild leaderboard for one team in one match. Public. team and board
// come from fixed vocabularies validated up front; matchId is path-escaped.
func newWvwStatsTopCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "top <matchId> <red|blue|green> <kdr|kills>",
		Short: "Top guilds by kdr/kills for one team in a WvW match",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			markCovered("wvw matches stats top")
			matchID, team, board := args[0], args[1], args[2]
			if !wvwTeams[team] {
				return fmt.Errorf("unknown team %q; valid: red, blue, green", team)
			}
			if !wvwTopBoards[board] {
				return fmt.Errorf("unknown board %q; valid: kdr, kills", board)
			}
			ctx := context.Background()
			client, err := app.client()
			if err != nil {
				return err
			}
			path := "/v2/wvw/matches/stats/" + url.PathEscape(matchID) + "/teams/" + team + "/top/" + board
			raw, err := client.Get(ctx, path, nil)
			if err != nil {
				return err
			}
			return output.Render(app.Out, raw, app.Mode, "")
		},
	}
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
				idList = append(idList, splitIDs(idsFlag)...)
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
				concise := ""
				if app.Mode == output.ModeConcise {
					concise = conciseIDList(raw)
				}
				return output.Render(app.Out, raw, app.Mode, concise)
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
		Args:  cobra.NoArgs,
		RunE:  newSimpleGetRunE(app, "/v2/wvw/timers", "wvw timers"),
	}
	cmd.AddCommand(
		&cobra.Command{
			Use:   "lockout",
			Short: "Fetch wvw timers lockout",
			Args:  cobra.NoArgs,
			RunE:  newSimpleGetRunE(app, "/v2/wvw/timers/lockout", "wvw timers lockout"),
		},
		&cobra.Command{
			Use:   "teamAssignment",
			Short: "Fetch wvw timers teamAssignment",
			Args:  cobra.NoArgs,
			RunE:  newSimpleGetRunE(app, "/v2/wvw/timers/teamAssignment", "wvw timers teamAssignment"),
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
		Args:  cobra.NoArgs,
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
