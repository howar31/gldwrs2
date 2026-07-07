package commands

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/howar31/gldwrs2/internal/output"
	"github.com/spf13/cobra"
)

// newByIDsListCmd builds a leaf command for the enumerate-ids -> fetch-by-ids
// shape shared by several /v2 resource families: bare invocation (no
// --ids/--all) lists ids only and never auto-dumps every entry; --ids
// fetches the given comma-separated ids; --all enumerates the full id list
// first and then fetches everything. covID is the string recorded via
// markCovered for the coverage meta-test. authed picks app.authedClient()
// (errors up front if no key is configured) over app.client() (anonymous is
// fine for public data).
//
// This is the shared extraction of what was, before this refactor, near-
// identical logic duplicated in data.go's newCatalogResourceCmd and wvw.go's
// newWvwListLeafCmd. data.go's version is intentionally left as-is (it also
// carries nesting/localized/buildCmd concerns this helper doesn't need);
// wvw.go's version now delegates here, as does pvp.go's public-resource set.
func newByIDsListCmd(app *App, use, short, path, covID string, render func([]json.RawMessage) string, authed bool) *cobra.Command {
	var ids string
	var all bool
	c := &cobra.Command{
		Use:   use,
		Short: short,
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
				raw, err := client.Get(ctx, path, nil)
				if err != nil {
					return err
				}
				return output.Render(app.Out, raw, app.Mode, conciseIDList(raw))
			}
			var idList []string
			if all {
				raw, err := client.Get(ctx, path, nil)
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
			items, err := client.GetByIDs(ctx, path, idList, nil)
			if err != nil {
				return err
			}
			concise := ""
			if render != nil {
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
