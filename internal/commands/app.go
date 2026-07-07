package commands

import (
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

// client builds an api.Client for the current profile. A missing/empty
// credential is fine for public endpoints (token stays empty).
func (a *App) client() (*api.Client, error) {
	token := ""
	if a.Store != nil {
		name := a.Profile
		if name == "" {
			if def, err := a.Store.DefaultProfile(); err == nil {
				name = def
			}
		}
		if name != "" {
			if tok, err := a.Store.Get(name); err == nil {
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

	root.AddCommand(newDataCmd(app))
	return root
}
