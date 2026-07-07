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
