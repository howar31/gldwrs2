package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestGenerateSkillsCommand runs the real "generate-skills" command through
// Root, exercising the actual gw2 command tree end to end (not a fake one),
// and asserts that two representative groups' generated SKILL.md files
// mention a known command.
func TestGenerateSkillsCommand(t *testing.T) {
	dir := t.TempDir()
	root := Root("9.9.9")
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"generate-skills", "--out", dir})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute generate-skills: %v", err)
	}

	dataBody, err := os.ReadFile(filepath.Join(dir, "gw2-data", "SKILL.md"))
	if err != nil {
		t.Fatalf("read gw2-data/SKILL.md: %v", err)
	}
	if !strings.Contains(string(dataBody), "gw2 data mounts skins") {
		t.Errorf("gw2-data/SKILL.md missing known command, got:\n%s", dataBody)
	}

	commerceBody, err := os.ReadFile(filepath.Join(dir, "gw2-commerce", "SKILL.md"))
	if err != nil {
		t.Fatalf("read gw2-commerce/SKILL.md: %v", err)
	}
	if !strings.Contains(string(commerceBody), "gw2 commerce prices") {
		t.Errorf("gw2-commerce/SKILL.md missing known command, got:\n%s", commerceBody)
	}
}
