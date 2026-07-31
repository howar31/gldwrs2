package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newAuthCmd(app *App) *cobra.Command {
	cmd := &cobra.Command{Use: "auth", Short: "Manage stored API keys"}

	var key string
	set := &cobra.Command{
		Use:   "set <profile>",
		Short: "Store an API key under a profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			markCovered("auth set")
			if key == "" {
				return fmt.Errorf("--key is required")
			}
			st, err := app.requireStore()
			if err != nil {
				return err
			}
			if err := st.Set(args[0], key); err != nil {
				return err
			}
			fmt.Fprintf(app.Out, "stored key for profile %q\n", args[0])
			return nil
		},
	}
	set.Flags().StringVar(&key, "key", "", "the API key value")

	list := &cobra.Command{
		Use:   "list",
		Short: "List stored profiles (never prints keys)",
		RunE: func(cmd *cobra.Command, args []string) error {
			markCovered("auth list")
			st, err := app.requireStore()
			if err != nil {
				return err
			}
			names, err := st.List()
			if err != nil {
				return err
			}
			def, _ := st.DefaultProfile()
			for _, n := range names {
				marker := ""
				if n == def {
					marker = " (default)"
				}
				fmt.Fprintf(app.Out, "%s%s\n", n, marker)
			}
			return nil
		},
	}

	remove := &cobra.Command{
		Use:   "remove <profile>",
		Short: "Remove a stored profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			markCovered("auth remove")
			st, err := app.requireStore()
			if err != nil {
				return err
			}
			if err := st.Remove(args[0]); err != nil {
				return err
			}
			fmt.Fprintf(app.Out, "removed profile %q\n", args[0])
			return nil
		},
	}

	cmd.AddCommand(set, list, remove)
	return cmd
}
