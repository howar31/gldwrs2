package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/howar31/gldwrs2/internal/output"
	"github.com/howar31/gldwrs2/internal/resolve"
	"github.com/spf13/cobra"
)

// guildSubresources is the known set of /v2/guild/:id/* subresource
// segments reached via the guild's positional dispatch (e.g. "gw2 guild
// <name> members"). All of these require a leader/officer API key
// (authedClient), unlike the public "search"/"permissions"/"upgrades"
// subcommands registered as children below. Only "members" gets a bespoke
// concise renderer, so (unlike character.go's characterSubresources) the
// map value here is just presence-in-set rather than a render func.
var guildSubresources = map[string]bool{
	"log":      true,
	"members":  true,
	"ranks":    true,
	"stash":    true,
	"storage":  true,
	"teams":    true,
	"treasury": true,
	"upgrades": true,
}

// newGuildCmd builds the "guild" command: a positional dispatcher (`gw2
// guild <id-or-name> [<subresource>]`, matching character.go's UX) plus
// three public subcommands (search/permissions/upgrades) that are
// server-wide catalogs, not scoped to any one guild.
//
// The id-or-name argument is resolved through resolve.GuildID using the
// PUBLIC client (/v2/guild/search needs no key); a bare guild id/name then
// fetches /v2/guild/<id> (also public -- basic guild info isn't
// leader-scoped); a subresource fetches /v2/guild/<id>/<subresource> via
// the AUTHED client (the API requires guild leader/officer scope for
// roster/treasury/log/etc).
//
// "upgrades" plays a dual role, same pattern as data.go's recipes/recipes-
// search and achievements.go's daily/tomorrow: `gw2 guild upgrades` (no id)
// is the public upgrade catalog, a registered subcommand that cobra
// resolves before ever reaching this command's own RunE below; `gw2 guild
// <id-or-name> upgrades` (two positional args, the second not matching any
// subcommand name) falls through to the positional dispatch and fetches
// that one guild's unlocked upgrades, authed. Both forms work; cobra's
// arg-matching against registered child commands is what disambiguates
// them, not any logic in this file.
func newGuildCmd(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "guild <id-or-name> [<subresource>]",
		Short: "Guild info, roster, and treasury",
		Long: "Guild info, roster, and treasury.\n\n" +
			"With just an id or name, fetches the guild's public basic info.\n" +
			"With a subresource too, fetches /v2/guild/<id>/<subresource>\n" +
			"(requires a leader/officer API key). Valid subresources: " +
			strings.Join(sortedGuildSubresourceKeys(), ", ") + ".",
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			markCovered("guild")

			// Guard: "gw2 guild members" (forgot the id) is a 1-arg
			// invocation whose sole arg happens to be a known subresource
			// name. Catch it before resolve.GuildID treats "members" as a
			// guild NAME and searches for it, which would surface a much
			// less helpful "no guild found named members".
			if len(args) == 1 && guildSubresources[args[0]] {
				return fmt.Errorf("specify a guild id or name first, e.g. gw2 guild <name> members")
			}

			ctx := context.Background()
			publicClient, err := app.client()
			if err != nil {
				return err
			}
			id, err := resolve.GuildID(ctx, publicClient, args[0])
			if err != nil {
				return err
			}

			if len(args) == 1 {
				raw, err := publicClient.Get(ctx, "/v2/guild/"+url.PathEscape(id), nil)
				if err != nil {
					return err
				}
				return output.Render(app.Out, raw, app.Mode, renderGuild(raw))
			}

			sub := args[1]
			if !guildSubresources[sub] {
				return fmt.Errorf("unknown guild subresource %q; valid: %s", sub, strings.Join(sortedGuildSubresourceKeys(), ", "))
			}
			authed, err := app.authedClient()
			if err != nil {
				return err
			}
			raw, err := authed.Get(ctx, "/v2/guild/"+url.PathEscape(id)+"/"+sub, nil)
			if err != nil {
				return err
			}
			concise := ""
			if sub == "members" {
				concise = renderGuildMembers(raw)
			}
			return output.Render(app.Out, raw, app.Mode, concise)
		},
	}

	cmd.AddCommand(
		newGuildSearchCmd(app),
		newByIDsListCmd(app, "permissions", "Fetch guild permissions", "/v2/guild/permissions", "guild permissions", renderNamed, false),
		newByIDsListCmd(app, "upgrades", "Fetch guild upgrades", "/v2/guild/upgrades", "guild upgrades", renderNamed, false),
	)
	return cmd
}

func sortedGuildSubresourceKeys() []string {
	keys := make([]string, 0, len(guildSubresources))
	for k := range guildSubresources {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// newGuildSearchCmd builds `guild search <name>`: GET /v2/guild/search,
// returning the list of guild ids matching that name (occasionally more
// than one -- guild names aren't unique across worlds/regions). Public.
func newGuildSearchCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "search <name>",
		Short: "Find a guild's id by name",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			markCovered("guild search")
			ctx := context.Background()
			client, err := app.client()
			if err != nil {
				return err
			}
			raw, err := client.Get(ctx, "/v2/guild/search", url.Values{"name": {args[0]}})
			if err != nil {
				return err
			}
			return output.Render(app.Out, raw, app.Mode, conciseIDList(raw))
		},
	}
}

// renderGuild renders the basic /v2/guild/:id object as
// "[TAG] Name (level N, M members)". Falls back to pretty JSON (returns
// "") if the object doesn't even carry a name.
func renderGuild(raw json.RawMessage) string {
	var v struct {
		Name        string `json:"name"`
		Tag         string `json:"tag"`
		Level       int    `json:"level"`
		MemberCount int    `json:"member_count"`
	}
	if err := json.Unmarshal(raw, &v); err != nil || v.Name == "" {
		return ""
	}
	s := fmt.Sprintf("[%s] %s (level %d", v.Tag, v.Name, v.Level)
	if v.MemberCount > 0 {
		s += fmt.Sprintf(", %d members", v.MemberCount)
	}
	return s + ")"
}

// renderGuildMembers renders /v2/guild/:id/members, an array of
// {name, rank, joined}, as "name\trank" (one per line). The join
// timestamp isn't shown concisely; use --json for the full object.
func renderGuildMembers(raw json.RawMessage) string {
	var items []struct {
		Name string `json:"name"`
		Rank string `json:"rank"`
	}
	if err := json.Unmarshal(raw, &items); err != nil {
		return ""
	}
	var b strings.Builder
	for _, it := range items {
		fmt.Fprintf(&b, "%s\t%s\n", it.Name, it.Rank)
	}
	return strings.TrimRight(b.String(), "\n")
}
