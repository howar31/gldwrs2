package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/howar31/gldwrs2/internal/output"
	"github.com/spf13/cobra"
)

// characterSubresources is the known set of /v2/characters/:id/* subresource
// segments. The GW2 API also exposes buildtabs/active and
// equipmenttabs/active, but those are reached via the --active flag rather
// than as separate tokens here: cobra.MaximumNArgs(2) leaves no positional
// slot for a third path segment, so "buildtabs"/"equipmenttabs" plus
// --active is the CLI's stand-in for the API's third segment. render is the
// concise renderer for the subresource's response; nil falls back to pretty
// JSON (output.Render's default in concise mode).
var characterSubresources = map[string]func(json.RawMessage) string{
	"core":            renderCharacterCore,
	"backstory":       nil,
	"buildtabs":       nil,
	"crafting":        renderCrafting,
	"dungeons":        nil,
	"equipment":       nil,
	"equipmenttabs":   nil,
	"heropoints":      nil,
	"inventory":       nil,
	"quests":          nil,
	"recipes":         nil,
	"sab":             nil,
	"skills":          nil,
	"specializations": nil,
	"training":        nil,
}

// activeCapableSubresources is the subset of characterSubresources for which
// the GW2 API also exposes a ".../active" variant (the currently equipped
// build/equipment tab, instead of the full list of tabs).
var activeCapableSubresources = map[string]bool{
	"buildtabs":     true,
	"equipmenttabs": true,
}

// newCharacterCmd builds the single "character" command covering
// /v2/characters (list) and the /v2/characters/:id/* subresources. Unlike
// data.go/account.go's registries, this is deliberately one cobra command
// with positional args (matching the spec UX `gw2 character "Name"
// equipment`), not one leaf per subresource, since the ":id" here is a
// free-form character name rather than a fixed path segment.
func newCharacterCmd(app *App) *cobra.Command {
	var active bool
	cmd := &cobra.Command{
		Use:   "character [name] [subresource]",
		Short: "Your characters and their details",
		Long: "Your characters and their details.\n\n" +
			"With no args, lists your character names. With a name, fetches that\n" +
			"character's core info. With a name and subresource, fetches\n" +
			"/v2/characters/<name>/<subresource>. Valid subresources: " +
			strings.Join(sortedSubresourceKeys(), ", ") + ".",
		Args: cobra.MaximumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			markCovered("character")
			ctx := context.Background()
			client, err := app.authedClient()
			if err != nil {
				return err
			}

			if len(args) < 2 {
				// --active only means anything once a buildtabs/
				// equipmenttabs subresource is selected; error instead of
				// silently ignoring a flag the user explicitly set.
				if active {
					return fmt.Errorf("--active requires a buildtabs or equipmenttabs subresource")
				}
				if len(args) == 0 {
					raw, err := client.Get(ctx, "/v2/characters", nil)
					if err != nil {
						return err
					}
					return output.Render(app.Out, raw, app.Mode, conciseIDList(raw))
				}
				name := url.PathEscape(args[0])
				raw, err := client.Get(ctx, "/v2/characters/"+name, nil)
				if err != nil {
					return err
				}
				return output.Render(app.Out, raw, app.Mode, renderCharacterCore(raw))
			}

			name := url.PathEscape(args[0])
			sub := args[1]
			render, ok := characterSubresources[sub]
			if !ok {
				return fmt.Errorf("unknown character subresource %q; valid: %s", sub, strings.Join(sortedSubresourceKeys(), ", "))
			}
			path := "/v2/characters/" + name + "/" + sub
			if active {
				if !activeCapableSubresources[sub] {
					return fmt.Errorf("--active only applies to buildtabs and equipmenttabs, not %q", sub)
				}
				path += "/active"
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
		},
	}
	cmd.Flags().BoolVar(&active, "active", false, "with buildtabs or equipmenttabs, fetch only the active tab (appends /active to the path)")
	return cmd
}

func sortedSubresourceKeys() []string {
	keys := make([]string, 0, len(characterSubresources))
	for k := range characterSubresources {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// renderCharacterCore renders the core character object -- returned by both
// GET /v2/characters/:id and GET /v2/characters/:id/core -- as
// "name (level L race profession)". Falls back to pretty JSON (returns "")
// if the object doesn't even have a name.
func renderCharacterCore(raw json.RawMessage) string {
	var v struct {
		Name       string `json:"name"`
		Level      int    `json:"level"`
		Race       string `json:"race"`
		Gender     string `json:"gender"`
		Profession string `json:"profession"`
	}
	if err := json.Unmarshal(raw, &v); err != nil || v.Name == "" {
		return ""
	}
	return fmt.Sprintf("%s (level %d %s %s)", v.Name, v.Level, v.Race, v.Profession)
}

// renderCrafting renders /v2/characters/:id/crafting, an array of
// {discipline, rating, active}, as "discipline rating" with a trailing
// "active" marker for the currently active discipline.
func renderCrafting(raw json.RawMessage) string {
	var items []struct {
		Discipline string `json:"discipline"`
		Rating     int    `json:"rating"`
		Active     bool   `json:"active"`
	}
	if err := json.Unmarshal(raw, &items); err != nil {
		return ""
	}
	var b strings.Builder
	for _, it := range items {
		status := ""
		if it.Active {
			status = " active"
		}
		fmt.Fprintf(&b, "%s\t%d%s\n", it.Discipline, it.Rating, status)
	}
	return strings.TrimRight(b.String(), "\n")
}
