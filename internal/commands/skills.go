package commands

import (
	"fmt"
	"io/fs"
	"path/filepath"

	gldwrs2 "github.com/howar31/gldwrs2"
	"github.com/howar31/gldwrs2/internal/skillgen"
	"github.com/spf13/cobra"
)

// newGenerateSkillsCmd builds "generate-skills", a utility command that
// regenerates the agent-facing skills/ tree (skills/gw2, skills/gw2-shared,
// and one skills/gw2-<group> skill per top-level command group) from the
// live command tree, so the generated docs can never drift from the actual
// CLI surface. It reads gldwrs2.Version directly rather than threading a
// version field through App, per the design brief.
//
// This is a developer utility, not a user-facing data command: it's hidden
// from --help and deliberately excluded from zz_coverage_test.go's
// leafCommands(), which enumerates commands that fetch GW2 API data.
func newGenerateSkillsCmd(app *App) *cobra.Command {
	var out string
	cmd := &cobra.Command{
		Use:    "generate-skills",
		Short:  "Regenerate the skills/ tree from the command tree",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := skillgen.Generate(cmd.Root(), gldwrs2.Version, out); err != nil {
				return err
			}
			n, err := countSkillFiles(out)
			if err != nil {
				return err
			}
			fmt.Fprintf(app.Out, "generated %d skill files in %s\n", n, out)
			return nil
		},
	}
	cmd.Flags().StringVar(&out, "out", "skills", "output directory for the generated skills tree")
	return cmd
}

// countSkillFiles counts the SKILL.md files Generate wrote under dir, so
// the command can report how many files were produced.
func countSkillFiles(dir string) (int, error) {
	n := 0
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && d.Name() == "SKILL.md" {
			n++
		}
		return nil
	})
	return n, err
}
