package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/howar31/gldwrs2/internal/output"
	"github.com/spf13/cobra"
)

// newBuildCmd builds "build", a public single-value leaf returning the
// current game build id ({"id": <int>}). It has a natural one-line concise
// form (renderBuild), so unlike leaf.go's newSimpleGetRunE (always pretty-
// JSON fallback) RunE is written inline here rather than reused.
func newBuildCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "build",
		Short: "Current game build id",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			markCovered("build")
			ctx := context.Background()
			client, err := app.client()
			if err != nil {
				return err
			}
			raw, err := client.Get(ctx, "/v2/build", nil)
			if err != nil {
				return err
			}
			return output.Render(app.Out, raw, app.Mode, renderBuild(raw))
		},
	}
}

// renderBuild renders /v2/build, a single object {id}, as "build <id>".
func renderBuild(raw json.RawMessage) string {
	var v struct {
		ID int `json:"id"`
	}
	if err := json.Unmarshal(raw, &v); err != nil {
		return ""
	}
	return fmt.Sprintf("build %d", v.ID)
}

// newTokenCmd builds the "token" parent. Both children are authed: the
// endpoints describe the configured API key itself (or mint a derivative of
// it), so there is no anonymous variant to fall back to.
func newTokenCmd(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "token",
		Short: "API key info and subtokens",
	}
	cmd.AddCommand(
		newTokenInfoCmd(app),
		newTokenSubtokenCmd(app),
	)
	return cmd
}

// newTokenInfoCmd builds "info": GET /v2/tokeninfo, describing the API key
// currently configured for the active profile (id, name, granted
// permissions; subtokens additionally carry type/expires_at, left to the
// pretty-JSON/raw fallback since renderTokenInfo only surfaces the fields
// every key has).
func newTokenInfoCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "info",
		Short: "Info about the configured API key",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			markCovered("token info")
			ctx := context.Background()
			client, err := app.authedClient()
			if err != nil {
				return err
			}
			raw, err := client.Get(ctx, "/v2/tokeninfo", nil)
			if err != nil {
				return err
			}
			return output.Render(app.Out, raw, app.Mode, renderTokenInfo(raw))
		},
	}
}

// renderTokenInfo renders /v2/tokeninfo, {id, name, permissions:[...]}.
func renderTokenInfo(raw json.RawMessage) string {
	var v struct {
		ID          string   `json:"id"`
		Name        string   `json:"name"`
		Permissions []string `json:"permissions"`
	}
	if err := json.Unmarshal(raw, &v); err != nil {
		return ""
	}
	return fmt.Sprintf("name: %s\nid: %s\npermissions: %s", v.Name, v.ID, strings.Join(v.Permissions, ", "))
}

// newTokenSubtokenCmd builds "subtoken": GET /v2/createsubtoken, minting a
// restricted-scope subtoken derived from the configured API key. All three
// flags are optional and each maps to its query param only when the caller
// actually set it (non-empty) -- an omitted flag must never send an empty
// expire=/permissions=/urls=, which the API could read as "expire
// immediately" / "no permissions" / "no urls allowed" instead of "no
// restriction requested here".
func newTokenSubtokenCmd(app *App) *cobra.Command {
	var expire, permissions, urls string
	c := &cobra.Command{
		Use:   "subtoken",
		Short: "Create a restricted-scope subtoken from the configured key",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			markCovered("token subtoken")
			ctx := context.Background()
			client, err := app.authedClient()
			if err != nil {
				return err
			}
			params := url.Values{}
			if expire != "" {
				params.Set("expire", expire)
			}
			if permissions != "" {
				params.Set("permissions", permissions)
			}
			if urls != "" {
				params.Set("urls", urls)
			}
			raw, err := client.Get(ctx, "/v2/createsubtoken", params)
			if err != nil {
				return err
			}
			return output.Render(app.Out, raw, app.Mode, renderSubtoken(raw))
		},
	}
	c.Flags().StringVar(&expire, "expire", "", "ISO8601 expiration timestamp (optional)")
	c.Flags().StringVar(&permissions, "permissions", "", "comma-separated permission subset of the parent key (optional)")
	c.Flags().StringVar(&urls, "urls", "", "comma-separated endpoint prefixes to restrict the subtoken to (optional)")
	return c
}

// renderSubtoken renders /v2/createsubtoken, {subtoken: <jwt>}, printing
// just the token string (no label/wrapping) so it's easy to copy elsewhere.
func renderSubtoken(raw json.RawMessage) string {
	var v struct {
		Subtoken string `json:"subtoken"`
	}
	if err := json.Unmarshal(raw, &v); err != nil {
		return ""
	}
	return v.Subtoken
}
