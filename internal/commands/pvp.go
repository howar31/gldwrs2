package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/howar31/gldwrs2/internal/output"
	"github.com/spf13/cobra"
)

// pvpByIDsResource is one public /v2/pvp/* static-data endpoint following
// the same enumerate-ids -> fetch-by-ids shape as data.go's
// catalogResources and wvw.go's wvwListResources, built through the shared
// newByIDsListCmd helper (leaf.go) instead of a third near-duplicate
// implementation.
type pvpByIDsResource struct {
	name string
	path string
}

var pvpByIDsResources = []pvpByIDsResource{
	{name: "amulets", path: "/v2/pvp/amulets"},
	{name: "ranks", path: "/v2/pvp/ranks"},
	{name: "heroes", path: "/v2/pvp/heroes"},
	{name: "rewardtracks", path: "/v2/pvp/rewardtracks"},
	{name: "runes", path: "/v2/pvp/runes"},
	{name: "sigils", path: "/v2/pvp/sigils"},
	{name: "seasons", path: "/v2/pvp/seasons"},
}

// newPvpCmd builds the "pvp" command tree: public static PvP data (amulets,
// ranks, heroes, reward tracks, runes, sigils, seasons, and season
// leaderboards) needs no API key; your recent games, standings, and
// aggregate stats are scoped to the caller's account and require one.
func newPvpCmd(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pvp",
		Short: "PvP stats, seasons, and reward tracks",
	}
	for _, res := range pvpByIDsResources {
		covID := "pvp " + res.name
		leaf := newByIDsListCmd(app, res.name, "Fetch "+covID, res.path, covID, renderNamed, false, false)
		// seasons plays a dual role: it's both a by-ids leaf and the parent
		// of "leaderboards", same pattern as data.go's recipes/recipes
		// search -- attach leaderboards as a child of the leaf command
		// newByIDsListCmd already built, rather than registering it
		// separately and risking a duplicate "seasons" group command.
		if res.name == "seasons" {
			leaf.AddCommand(newPvpSeasonsLeaderboardsCmd(app))
		}
		cmd.AddCommand(leaf)
	}
	cmd.AddCommand(
		newByIDsListCmd(app, "games", "Fetch pvp games", "/v2/pvp/games", "pvp games", nil, true, true),
		newPvpStandingsCmd(app),
		newPvpStatsCmd(app),
	)
	return cmd
}

// newPvpSeasonsLeaderboardsCmd builds `pvp seasons leaderboards <seasonId>
// <board> <region>`. Public: leaderboard standings are server-wide
// information about a PvP season, not scoped to the caller's account.
// seasonId is path-escaped (its format isn't guaranteed numeric); board and
// region are the API's own short vocabulary words (e.g. "ladder", "na") and
// need no escaping.
func newPvpSeasonsLeaderboardsCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "leaderboards <seasonId> <board> <region>",
		Short: "Fetch pvp seasons leaderboards",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			markCovered("pvp seasons leaderboards")
			seasonID, board, region := args[0], args[1], args[2]
			ctx := context.Background()
			client, err := app.client()
			if err != nil {
				return err
			}
			path := "/v2/pvp/seasons/" + url.PathEscape(seasonID) + "/leaderboards/" + board + "/" + region
			raw, err := client.Get(ctx, path, nil)
			if err != nil {
				return err
			}
			return output.Render(app.Out, raw, app.Mode, "")
		},
	}
}

// newPvpStandingsCmd builds `pvp standings`, authed: your PvP season
// standings (best/current rank, etc.), scoped to the caller's account. No
// natural named-list concise form, so it always renders pretty JSON.
func newPvpStandingsCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "standings",
		Short: "Your PvP season standings",
		RunE: func(cmd *cobra.Command, args []string) error {
			markCovered("pvp standings")
			ctx := context.Background()
			client, err := app.authedClient()
			if err != nil {
				return err
			}
			raw, err := client.Get(ctx, "/v2/pvp/standings", nil)
			if err != nil {
				return err
			}
			return output.Render(app.Out, raw, app.Mode, "")
		},
	}
}

// newPvpStatsCmd builds `pvp stats`, authed: your aggregate PvP record
// (rank, win/loss totals), scoped to the caller's account.
func newPvpStatsCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "stats",
		Short: "Your aggregate PvP stats",
		RunE: func(cmd *cobra.Command, args []string) error {
			markCovered("pvp stats")
			ctx := context.Background()
			client, err := app.authedClient()
			if err != nil {
				return err
			}
			raw, err := client.Get(ctx, "/v2/pvp/stats", nil)
			if err != nil {
				return err
			}
			return output.Render(app.Out, raw, app.Mode, renderPvpStats(raw))
		},
	}
}

// renderPvpStats renders /v2/pvp/stats, a single object summarizing the
// account's PvP rank and aggregate win/loss record:
// {pvp_rank, aggregate:{wins,losses,desertions,byes,forfeits}, ...}. Falls
// back to pretty JSON if the object doesn't even carry a pvp_rank.
func renderPvpStats(raw json.RawMessage) string {
	var v struct {
		PvpRank   int `json:"pvp_rank"`
		Aggregate struct {
			Wins       int `json:"wins"`
			Losses     int `json:"losses"`
			Desertions int `json:"desertions"`
			Byes       int `json:"byes"`
			Forfeits   int `json:"forfeits"`
		} `json:"aggregate"`
	}
	if err := json.Unmarshal(raw, &v); err != nil || v.PvpRank == 0 {
		return ""
	}
	return fmt.Sprintf("pvp_rank\t%d\nwins\t%d\tlosses\t%d\tdesertions\t%d\tbyes\t%d\tforfeits\t%d",
		v.PvpRank, v.Aggregate.Wins, v.Aggregate.Losses, v.Aggregate.Desertions, v.Aggregate.Byes, v.Aggregate.Forfeits)
}
