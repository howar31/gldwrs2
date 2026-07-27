package commands

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"

	"github.com/howar31/gldwrs2/internal/api"
	"github.com/howar31/gldwrs2/internal/output"
	"github.com/spf13/cobra"
)

// splitIDs splits a comma-separated --ids value, trimming whitespace around
// each id and dropping empties, so `--ids "1, 2"` queries "2" rather than
// the literal " 2" (which the API treats as an unknown id).
func splitIDs(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// noLangParams returns params that suppress the client's automatic lang
// injection, for endpoints that aren't localized.
func noLangParams() url.Values {
	p := url.Values{}
	p.Set("lang", api.LangNone)
	return p
}

// newByIDsListCmd builds a leaf command for the enumerate-ids -> fetch-by-ids
// shape shared by several /v2 resource families: bare invocation (no
// --ids/--all) lists ids only and never auto-dumps every entry; --ids
// fetches the given comma-separated ids; --all enumerates the full id list
// first and then fetches everything. covID is the string recorded via
// markCovered for the coverage meta-test. authed picks app.authedClient()
// (errors up front if no key is configured) over app.client() (anonymous is
// fine for public data). noLang suppresses the lang query param for
// endpoints that aren't localized.
//
// This is the single implementation of the by-ids list shape: data.go's
// catalog leaves, wvw.go's list leaves, and pvp.go's public-resource set all
// delegate here. Positional args are rejected (cobra.NoArgs) -- ids go
// through --ids, and silently ignoring a positional id would return the full
// id list while looking like a successful lookup.
func newByIDsListCmd(app *App, use, short, path, covID string, render func([]json.RawMessage) string, authed, noLang bool) *cobra.Command {
	var ids string
	var all bool
	params := func() url.Values {
		if noLang {
			return noLangParams()
		}
		return nil
	}
	c := &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			markCovered(covID)
			ctx := context.Background()
			getClient := app.client
			if authed {
				getClient = app.authedClient
			}
			client, err := getClient()
			if err != nil {
				return err
			}
			// No ids and not --all: show the id list only (never auto-dump).
			if ids == "" && !all {
				raw, err := client.Get(ctx, path, params())
				if err != nil {
					return err
				}
				concise := ""
				if app.Mode == output.ModeConcise {
					concise = conciseIDList(raw)
				}
				return output.Render(app.Out, raw, app.Mode, concise)
			}
			var idList []string
			if all {
				raw, err := client.Get(ctx, path, params())
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
				idList = splitIDs(ids)
			}
			items, err := client.GetByIDs(ctx, path, idList, params())
			if err != nil {
				return err
			}
			concise := ""
			if app.Mode == output.ModeConcise && render != nil {
				concise = render(items)
			}
			merged, _ := json.Marshal(items)
			return output.Render(app.Out, merged, app.Mode, concise)
		},
	}
	c.Flags().StringVar(&ids, "ids", "", "comma-separated ids (omit to list ids)")
	c.Flags().BoolVar(&all, "all", false, "fetch every entry (explicit; may be large)")
	return c
}

// newSimpleGetRunE builds a RunE for a plain public GET with no parameters
// and no natural named-list concise form (pretty JSON fallback). Originally
// written for wvw.go's "timers" group (three identical leaves); reused as-is
// by achievements.go's "daily"/"daily tomorrow" rather than duplicating a
// fourth and fifth copy of the same six lines. Commands built around this
// take no arguments; construction sites set cobra.NoArgs.
func newSimpleGetRunE(app *App, path, covID string) func(*cobra.Command, []string) error {
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
