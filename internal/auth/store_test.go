package auth

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	return NewStore(dir, key)
}

func TestSetGetRoundTrip(t *testing.T) {
	s := newTestStore(t)
	if err := s.Set("main", "SECRET-KEY"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, err := s.Get("main")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != "SECRET-KEY" {
		t.Fatalf("got %q", got)
	}
}

func TestTokenNotStoredInPlaintext(t *testing.T) {
	dir := t.TempDir()
	key := make([]byte, 32)
	s := NewStore(dir, key)
	if err := s.Set("main", "PLAINTEXT-SECRET"); err != nil {
		t.Fatal(err)
	}
	data := readFile(t, filepath.Join(dir, "config.toml"))
	if contains(data, "PLAINTEXT-SECRET") {
		t.Fatal("token stored in plaintext")
	}
}

func TestListAndRemoveAndDefault(t *testing.T) {
	s := newTestStore(t)
	s.Set("a", "ka")
	s.Set("b", "kb")
	names, _ := s.List()
	if len(names) != 2 {
		t.Fatalf("names = %v", names)
	}
	def, _ := s.DefaultProfile()
	if def != "a" {
		t.Fatalf("default = %q, want first-set 'a'", def)
	}
	if err := s.Remove("a"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get("a"); err == nil {
		t.Fatal("expected error after remove")
	}
}

func readFile(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
func contains(hay, needle string) bool { return strings.Contains(hay, needle) }

func TestDefaultDirHonorsXDG(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	got, err := DefaultDir()
	if err != nil {
		t.Fatalf("DefaultDir: %v", err)
	}
	want := filepath.Join(tmp, "gw2")
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}

	t.Setenv("XDG_CONFIG_HOME", "")
	got, err = DefaultDir()
	if err != nil {
		t.Fatalf("DefaultDir: %v", err)
	}
	if !strings.HasSuffix(got, string(filepath.Separator)+filepath.Join(".config", "gw2")) {
		t.Fatalf("got %q, want suffix /.config/gw2", got)
	}
}

func TestShortKeyRejected(t *testing.T) {
	s := NewStore(t.TempDir(), make([]byte, 16))
	if err := s.Set("x", "y"); err == nil {
		t.Fatal("expected error for short key, got nil")
	}
}

func TestOrderStableAfterRemoveAndReAdd(t *testing.T) {
	s := newTestStore(t)
	if err := s.Set("a", "ka"); err != nil {
		t.Fatal(err)
	}
	if err := s.Set("b", "kb"); err != nil {
		t.Fatal(err)
	}
	if err := s.Set("c", "kc"); err != nil {
		t.Fatal(err)
	}
	if err := s.Remove("b"); err != nil {
		t.Fatal(err)
	}
	if err := s.Set("d", "kd"); err != nil {
		t.Fatal(err)
	}
	names, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"a", "c", "d"}
	if len(names) != len(want) {
		t.Fatalf("names = %v, want %v", names, want)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("names = %v, want %v", names, want)
		}
	}
	def, err := s.DefaultProfile()
	if err != nil {
		t.Fatal(err)
	}
	if def != "a" {
		t.Fatalf("default = %q, want %q", def, "a")
	}

	if err := s.Remove("a"); err != nil {
		t.Fatal(err)
	}
	def, err = s.DefaultProfile()
	if err != nil {
		t.Fatal(err)
	}
	if def != "c" {
		t.Fatalf("default after removing a = %q, want %q", def, "c")
	}
}
