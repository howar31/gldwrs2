package commands

import (
	"github.com/spf13/cobra"
)

// newAchievementsCmd builds the "achievements" command tree: the
// achievement catalog itself, its categories/groups, and the daily
// achievement schedule (today's and tomorrow's). All public -- these are
// server-wide reference/schedule data, unlike an account's earned
// achievement progress (/v2/account/achievements, already covered by the
// account group).
//
// achievements is a dual-role command, same pattern as data.go's
// recipes/recipes-search and pvp.go's seasons/leaderboards: newByIDsListCmd
// already builds a full by-ids leaf command (bare id list / --ids / --all),
// so categories/groups/daily are attached as children of that same command
// rather than registered as separate siblings, which would risk a duplicate
// "achievements" group command.
func newAchievementsCmd(app *App) *cobra.Command {
	cmd := newByIDsListCmd(app, "achievements", "Fetch achievements", "/v2/achievements", "achievements", renderNamed, false, false)
	cmd.Short = "Achievements, categories, groups, and dailies"
	cmd.AddCommand(
		newByIDsListCmd(app, "categories", "Fetch achievements categories", "/v2/achievements/categories", "achievements categories", renderNamed, false, false),
		newByIDsListCmd(app, "groups", "Fetch achievements groups", "/v2/achievements/groups", "achievements groups", renderNamed, false, false),
		newAchievementsDailyCmd(app),
	)
	return cmd
}

// newAchievementsDailyCmd builds "daily" and its "tomorrow" child. Like
// wvw.go's "timers" group, daily is itself a dual-role command: a plain
// public GET (today's daily achievements, no path params) that is also the
// parent of "tomorrow" (same GET, one day ahead). Neither is a by-ids
// endpoint, so both are built directly with newSimpleGetRunE (leaf.go)
// rather than newByIDsListCmd; the /v2/achievements/daily(/tomorrow) payload
// (pve/pvp/wvw/fractals/special buckets of objects) has no natural
// named-list concise form, so both fall back to pretty JSON.
func newAchievementsDailyCmd(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "daily",
		Short: "Today's daily achievements",
		Args:  cobra.NoArgs,
		RunE:  newSimpleGetRunE(app, "/v2/achievements/daily", "achievements daily"),
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "tomorrow",
		Short: "Tomorrow's daily achievements",
		Args:  cobra.NoArgs,
		RunE:  newSimpleGetRunE(app, "/v2/achievements/daily/tomorrow", "achievements daily tomorrow"),
	})
	return cmd
}
