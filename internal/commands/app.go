package commands

import (
	"io"
	"os"

	"github.com/spf13/cobra"
)

// App carries dependencies shared by all commands, populated from the root
// persistent flags in PersistentPreRunE.
type App struct {
	Out     io.Writer
	Profile string
	Lang    string
}

// Root builds the gw2 root command tree.
func Root(version string) *cobra.Command {
	app := &App{Out: os.Stdout}
	root := &cobra.Command{
		Use:           "gw2",
		Short:         "Guild Wars 2 API command-line client",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.SetVersionTemplate(
		"gw2 {{.Version}}\nSource: github.com/howar31/gldwrs2\n")
	root.PersistentFlags().StringVar(&app.Profile, "profile", "", "credential profile name")
	root.PersistentFlags().StringVar(&app.Lang, "lang", "en", "response language (en,es,de,fr,zh)")
	return root
}
