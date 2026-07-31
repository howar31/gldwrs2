package auth

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// TestLoadOrCreateKeyRefusesUnreadableKey locks the destructive-overwrite
// fix: a key file that EXISTS but cannot be read (permissions, I/O) must
// surface an error -- silently generating a fresh key over it would
// permanently destroy the ability to decrypt every stored profile.
func TestLoadOrCreateKeyRefusesUnreadableKey(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "key")
	original := bytes.Repeat([]byte{0xAB}, 32)
	if err := os.WriteFile(keyPath, original, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(keyPath, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(keyPath, 0o600) })

	if _, err := LoadOrCreateKey(dir); err == nil {
		t.Fatal("LoadOrCreateKey with an unreadable existing key file must error, got nil")
	}

	if err := os.Chmod(keyPath, 0o600); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(after, original) {
		t.Fatal("key file was overwritten despite the read error")
	}
}

// TestLoadOrCreateKeyStillCreatesWhenMissing confirms the fix didn't break
// legitimate first-run creation (ErrNotExist is still the create path).
func TestLoadOrCreateKeyStillCreatesWhenMissing(t *testing.T) {
	dir := t.TempDir()
	key, err := LoadOrCreateKey(dir)
	if err != nil {
		t.Fatalf("first-run create: %v", err)
	}
	if len(key) != 32 {
		t.Fatalf("created key length = %d, want 32", len(key))
	}
}
