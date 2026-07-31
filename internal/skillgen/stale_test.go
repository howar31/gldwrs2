package skillgen_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/howar31/gldwrs2/internal/skillgen"
)

// TestGenerateRemovesStaleDirs locks the orphan-cleanup fix: a previously
// generated gw2-<group> directory whose group no longer exists must be
// removed on regeneration, while unrelated directories in the output dir
// are left alone.
func TestGenerateRemovesStaleDirs(t *testing.T) {
	dir := t.TempDir()

	stale := filepath.Join(dir, "gw2-oldgroup")
	if err := os.MkdirAll(stale, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stale, "SKILL.md"), []byte("---\nname: gw2-oldgroup\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	foreign := filepath.Join(dir, "notes")
	if err := os.MkdirAll(foreign, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(foreign, "README.md"), []byte("keep me"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := skillgen.Generate(buildFakeRoot(), "9.9.9", dir); err != nil {
		t.Fatalf("Generate: %v", err)
	}

	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatal("stale gw2-oldgroup/ should have been removed")
	}
	if _, err := os.Stat(filepath.Join(foreign, "README.md")); err != nil {
		t.Fatalf("unrelated notes/ dir must be left alone: %v", err)
	}
}

// TestSharedBodyDocumentsFirstSetDefault locks the doc-accuracy fix: the
// generated gw2-shared skill must describe the default profile as the
// FIRST-set profile (matching store.Set's actual behavior), not the most
// recently set one.
func TestSharedBodyDocumentsFirstSetDefault(t *testing.T) {
	dir := t.TempDir()
	if err := skillgen.Generate(buildFakeRoot(), "9.9.9", dir); err != nil {
		t.Fatalf("Generate: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(dir, "gw2-shared", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(body)
	if strings.Contains(s, "most recently set") {
		t.Fatal("gw2-shared still claims most-recently-set default (the store uses first-set)")
	}
	if !strings.Contains(s, "FIRST profile") {
		t.Fatalf("gw2-shared should document the first-set default rule, got:\n%s", s)
	}
}
