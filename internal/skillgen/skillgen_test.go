package skillgen_test

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/howar31/gldwrs2/internal/skillgen"
	"github.com/spf13/cobra"
)

// buildFakeRoot builds a minimal two-group command tree: "alpha" with leaf
// "leaf1", and "beta" with leaf "leaf2". Neither group carries a RunE of its
// own, matching the shape of most real gw2 groups (e.g. data, commerce).
func buildFakeRoot() *cobra.Command {
	root := &cobra.Command{Use: "gw2"}

	alpha := &cobra.Command{Use: "alpha", Short: "Alpha group"}
	alpha.AddCommand(&cobra.Command{
		Use:   "leaf1",
		Short: "Alpha leaf one",
		RunE:  func(*cobra.Command, []string) error { return nil },
	})

	beta := &cobra.Command{Use: "beta", Short: "Beta group"}
	leaf2 := &cobra.Command{
		Use:   "leaf2",
		Short: "Beta leaf two",
		RunE:  func(*cobra.Command, []string) error { return nil },
	}
	leaf2.Flags().String("name", "", "a name to filter by")
	beta.AddCommand(leaf2)

	root.AddCommand(alpha, beta)
	return root
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

// TestGenerateProducesGroupSkills verifies Generate writes the index skill,
// the shared skill, and one skill per group, each with the required
// name/description frontmatter, and that a group's skill body mentions its
// leaf's full command path.
func TestGenerateProducesGroupSkills(t *testing.T) {
	root := buildFakeRoot()
	dir := t.TempDir()

	if err := skillgen.Generate(root, "9.9.9", dir); err != nil {
		t.Fatalf("Generate: %v", err)
	}

	indexBody := readFile(t, filepath.Join(dir, "gw2", "SKILL.md"))
	if !strings.Contains(indexBody, "name: gw2\n") {
		t.Errorf("gw2/SKILL.md missing frontmatter name, got:\n%s", indexBody)
	}
	if !strings.Contains(indexBody, "description:") {
		t.Errorf("gw2/SKILL.md missing frontmatter description, got:\n%s", indexBody)
	}
	if !strings.Contains(indexBody, "gw2-alpha") || !strings.Contains(indexBody, "gw2-beta") {
		t.Errorf("gw2/SKILL.md should link both groups, got:\n%s", indexBody)
	}

	sharedBody := readFile(t, filepath.Join(dir, "gw2-shared", "SKILL.md"))
	if !strings.Contains(sharedBody, "name: gw2-shared\n") {
		t.Errorf("gw2-shared/SKILL.md missing frontmatter name, got:\n%s", sharedBody)
	}
	if !strings.Contains(sharedBody, "description:") {
		t.Errorf("gw2-shared/SKILL.md missing frontmatter description, got:\n%s", sharedBody)
	}

	alphaBody := readFile(t, filepath.Join(dir, "gw2-alpha", "SKILL.md"))
	if !strings.Contains(alphaBody, "name: gw2-alpha\n") {
		t.Errorf("gw2-alpha/SKILL.md missing frontmatter name, got:\n%s", alphaBody)
	}
	if !strings.Contains(alphaBody, "description: Alpha group") {
		t.Errorf("gw2-alpha/SKILL.md missing frontmatter description, got:\n%s", alphaBody)
	}
	if !strings.Contains(alphaBody, "gw2 alpha leaf1") {
		t.Errorf("gw2-alpha/SKILL.md should contain leaf command path, got:\n%s", alphaBody)
	}

	betaBody := readFile(t, filepath.Join(dir, "gw2-beta", "SKILL.md"))
	if !strings.Contains(betaBody, "name: gw2-beta\n") {
		t.Errorf("gw2-beta/SKILL.md missing frontmatter name, got:\n%s", betaBody)
	}
	if !strings.Contains(betaBody, "gw2 beta leaf2") {
		t.Errorf("gw2-beta/SKILL.md should contain leaf command path, got:\n%s", betaBody)
	}
	if !strings.Contains(betaBody, "--name") {
		t.Errorf("gw2-beta/SKILL.md should mention leaf2's local flag, got:\n%s", betaBody)
	}
}

// TestGenerateDeterministic verifies that generating from the same command
// tree twice, into two separate directories, produces byte-identical files
// (stable group/leaf ordering).
func TestGenerateDeterministic(t *testing.T) {
	root := buildFakeRoot()
	dir1 := t.TempDir()
	dir2 := t.TempDir()

	if err := skillgen.Generate(root, "1.2.3", dir1); err != nil {
		t.Fatalf("Generate (1st run): %v", err)
	}
	if err := skillgen.Generate(root, "1.2.3", dir2); err != nil {
		t.Fatalf("Generate (2nd run): %v", err)
	}

	var files []string
	if err := filepath.WalkDir(dir1, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir1, path)
		if err != nil {
			return err
		}
		files = append(files, rel)
		return nil
	}); err != nil {
		t.Fatalf("walk %s: %v", dir1, err)
	}
	if len(files) == 0 {
		t.Fatal("Generate produced no files")
	}

	for _, rel := range files {
		a, err := os.ReadFile(filepath.Join(dir1, rel))
		if err != nil {
			t.Fatalf("read %s (run 1): %v", rel, err)
		}
		b, err := os.ReadFile(filepath.Join(dir2, rel))
		if err != nil {
			t.Fatalf("read %s (run 2): %v", rel, err)
		}
		if !bytes.Equal(a, b) {
			t.Errorf("%s differs between the two runs", rel)
		}
	}
}
