package commands

import (
	"errors"
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

	// lazyStore (set only by Root) enables on-demand opening of the real
	// credential store the first time a token or an auth command needs it.
	// Public commands that never touch credentials therefore run without
	// creating ~/.config/gw2/ or an encryption key -- including in
	// environments with no usable HOME. Tests construct App directly (with
	// Store set, or nil for "no store") and never hit the filesystem.
	lazyStore bool
	storeInit bool
	storeErr  error
}

// openStore opens (and on first run creates) the encrypted credential store,
// caching the result -- including a failure -- for the process lifetime.
func (a *App) openStore() (*auth.Store, error) {
	if a.Store != nil {
		return a.Store, nil
	}
	if !a.lazyStore {
		return nil, nil // tests / no store configured
	}
	if a.storeInit {
		return nil, a.storeErr
	}
	a.storeInit = true
	dir, err := auth.DefaultDir()
	if err != nil {
		a.storeErr = fmt.Errorf("locate config dir: %w", err)
		return nil, a.storeErr
	}
	key, err := auth.LoadOrCreateKey(dir)
	if err != nil {
		a.storeErr = err
		return nil, a.storeErr
	}
	a.Store = auth.NewStore(dir, key)
	return a.Store, nil
}

// requireStore is openStore for the auth subcommands, which cannot do
// anything useful without a store: a nil store (possible only in tests that
// deliberately omit one) is an error rather than a silent no-op.
func (a *App) requireStore() (*auth.Store, error) {
	st, err := a.openStore()
	if err != nil {
		return nil, err
	}
	if st == nil {
		return nil, errors.New("credential store unavailable")
	}
	return st, nil
}

// resolveToken resolves the API token for the current profile.
//
// If a.Profile is explicitly set and the lookup fails, that error is
// returned (never falls back to anonymous) so a typo'd --profile doesn't
// silently send an unauthenticated request. If a.Profile is empty, the
// default profile (if any) is used; a decrypt/lookup error for an existing
// default, and a corrupt config file, are propagated rather than
// misreported as "no key configured". Only the genuinely benign cases fall
// back to an anonymous (empty) token: no profiles configured, or -- for
// public (non-strict) callers -- no usable credential store in this
// environment at all (e.g. no HOME).
func (a *App) resolveToken(strict bool) (string, error) {
	st, err := a.openStore()
	if err != nil {
		if strict {
			return "", err
		}
		return "", nil
	}
	if st == nil {
		return "", nil
	}
	if a.Profile != "" {
		tok, err := st.Get(a.Profile)
		if err != nil {
			return "", fmt.Errorf("profile %q: %w", a.Profile, err)
		}
		return tok, nil
	}
	def, err := st.DefaultProfile()
	if err != nil {
		if errors.Is(err, auth.ErrNoProfiles) {
			return "", nil
		}
		return "", fmt.Errorf("credential store: %w", err)
	}
	tok, err := st.Get(def)
	if err != nil {
		return "", fmt.Errorf("profile %q: %w", def, err)
	}
	return tok, nil
}

func (a *App) newClient(token string) *api.Client {
	return api.New(
		api.WithBaseURL(a.BaseURL),
		api.WithToken(token),
		api.WithLang(a.Lang),
		api.WithLimiter(a.Limiter),
		api.WithUserAgent("gw2-cli/"+versionOrDev),
	)
}

// client builds an api.Client for the current profile. An empty (anonymous)
// token is fine here -- this is used by public data endpoints, so an
// unusable credential store (no HOME, unreadable key) degrades to anonymous
// instead of blocking commands that never needed credentials.
func (a *App) client() (*api.Client, error) {
	token, err := a.resolveToken(false)
	if err != nil {
		return nil, err
	}
	return a.newClient(token), nil
}

// authedClient is like client, but requires a non-empty token: it's used by
// authenticated /v2/account/* (and similar) endpoints that would otherwise
// fail with an opaque 401/403 from the API. Errors clearly up front instead,
// and surfaces real store failures (unreadable key file, corrupt config)
// rather than misdirecting the user to re-enter a key.
func (a *App) authedClient() (*api.Client, error) {
	token, err := a.resolveToken(true)
	if err != nil {
		return nil, err
	}
	if token == "" {
		return nil, errors.New("this command needs an API key; run: gw2 auth set <name> --key <key>")
	}
	return a.newClient(token), nil
}

var versionOrDev = "dev"

// Root builds the gw2 root command tree.
func Root(version string) *cobra.Command {
	versionOrDev = version
	app := &App{Out: os.Stdout, Mode: output.ModeConcise, Lang: "en", BaseURL: api.DefaultBaseURL}
	var raw, jsonOut bool

	app.lazyStore = true

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
			// The credential store is NOT opened here: it is created
			// lazily by openStore the first time a token or an auth
			// command needs it, so public commands work even where no
			// config dir can exist (see App.lazyStore).
			app.Limiter = api.NewLimiter(300, 5)
			return nil
		},
	}
	root.SetVersionTemplate("gw2 {{.Version}}\nSource: github.com/howar31/gldwrs2\n")
	root.PersistentFlags().StringVar(&app.Profile, "profile", "", "credential profile name")
	root.PersistentFlags().StringVar(&app.Lang, "lang", "en", "response language (en,es,de,fr,zh)")
	root.PersistentFlags().BoolVar(&raw, "raw", false, "print the API's raw JSON")
	root.PersistentFlags().BoolVar(&jsonOut, "json", false, "print pretty JSON")

	root.AddCommand(newDataCmd(app), newAuthCmd(app), newAccountCmd(app), newCharacterCmd(app), newCommerceCmd(app), newWvwCmd(app), newPvpCmd(app), newAchievementsCmd(app), newGuildCmd(app), newBuildCmd(app), newTokenCmd(app), newGenerateSkillsCmd(app))
	return root
}
