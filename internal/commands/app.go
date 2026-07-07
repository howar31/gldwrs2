package commands

import (
	"fmt"
	"io"
	"os"

	"github.com/howar31/gldwrs2/internal/api"
	"github.com/howar31/gldwrs2/internal/auth"
	"github.com/howar31/gldwrs2/internal/output"
	"github.com/spf13/cobra"
)

// App carries dependencies shared by all commands.
type App struct {
	Out     io.Writer
	BaseURL string
	Profile string
	Lang    string
	Mode    output.Mode
	Store   *auth.Store
	Limiter *api.Limiter
}

// client builds an api.Client for the current profile.
//
// If a.Profile is explicitly set and the lookup fails, that error is
// returned (never falls back to anonymous) so a typo'd --profile doesn't
// silently send an unauthenticated request. If a.Profile is empty, the
// default profile (if any) is used; a decrypt/lookup error for an existing
// default is propagated. If no default is configured, an empty token is
// fine for public endpoints.
func (a *App) client() (*api.Client, error) {
	token := ""
	if a.Store != nil {
		switch {
		case a.Profile != "":
			tok, err := a.Store.Get(a.Profile)
			if err != nil {
				return nil, fmt.Errorf("profile %q: %w", a.Profile, err)
			}
			token = tok
		default:
			if def, err := a.Store.DefaultProfile(); err == nil {
				tok, err := a.Store.Get(def)
				if err != nil {
					return nil, fmt.Errorf("profile %q: %w", def, err)
				}
				token = tok
			}
		}
	}
	return api.New(
		api.WithBaseURL(a.BaseURL),
		api.WithToken(token),
		api.WithLang(a.Lang),
		api.WithLimiter(a.Limiter),
		api.WithUserAgent("gw2-cli/"+versionOrDev),
	), nil
}

var versionOrDev = "dev"

// Root builds the gw2 root command tree.
func Root(version string) *cobra.Command {
	versionOrDev = version
	app := &App{Out: os.Stdout, Mode: output.ModeConcise, Lang: "en", BaseURL: api.DefaultBaseURL}
	var raw, jsonOut bool

	root := &cobra.Command{
		Use:           "gw2",
		Short:         "Guild Wars 2 API command-line client",
		Long:          "Guild Wars 2 API command-line client\n\nSource: github.com/howar31/gldwrs2",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if v := os.Getenv("GW2_API_BASE"); v != "" {
				app.BaseURL = v
			}
			switch {
			case raw:
				app.Mode = output.ModeRaw
			case jsonOut:
				app.Mode = output.ModeJSON
			default:
				app.Mode = output.ModeConcise
			}
			dir, err := auth.DefaultDir()
			if err != nil {
				return err
			}
			key, err := auth.LoadOrCreateKey(dir)
			if err != nil {
				return err
			}
			app.Store = auth.NewStore(dir, key)
			app.Limiter = api.NewLimiter(300, 5)
			return nil
		},
	}
	root.SetVersionTemplate("gw2 {{.Version}}\nSource: github.com/howar31/gldwrs2\n")
	root.PersistentFlags().StringVar(&app.Profile, "profile", "", "credential profile name")
	root.PersistentFlags().StringVar(&app.Lang, "lang", "en", "response language (en,es,de,fr,zh)")
	root.PersistentFlags().BoolVar(&raw, "raw", false, "print the API's raw JSON")
	root.PersistentFlags().BoolVar(&jsonOut, "json", false, "print pretty JSON")

	root.AddCommand(newDataCmd(app), newAuthCmd(app))
	return root
}
