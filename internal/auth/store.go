// Package auth provides an encrypted credential store with named profiles.
// API tokens are encrypted with AES-256-GCM before being persisted to
// ~/.config/gw2/config.toml; the encryption key is stored separately.
package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/BurntSushi/toml"
)

type profile struct {
	Enc   string `toml:"enc"`   // base64(nonce||ciphertext)
	Order int    `toml:"order"` // set-order, for stable DefaultProfile
}

type config struct {
	Default  string             `toml:"default"`
	Profiles map[string]profile `toml:"profiles"`
}

// Store persists API keys encrypted with AES-256-GCM in <dir>/config.toml.
type Store struct {
	dir string
	key []byte // 32 bytes
}

func NewStore(dir string, key []byte) *Store { return &Store{dir: dir, key: key} }

// DefaultDir returns the default config directory for gw2, following the
// project's XDG convention (~/.config/gw2), not the platform-native config
// dir (e.g. macOS's ~/Library/Application Support).
func DefaultDir() (string, error) {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "gw2"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "gw2"), nil
}

// LoadOrCreateKey returns the 32-byte encryption key, creating one at
// <dir>/key (0600) on first use. GW2_KEYRING_BACKEND=file:<path> overrides.
func LoadOrCreateKey(dir string) ([]byte, error) {
	path := filepath.Join(dir, "key")
	if b := os.Getenv("GW2_KEYRING_BACKEND"); len(b) > 5 && b[:5] == "file:" {
		path = b[5:]
	}
	if data, err := os.ReadFile(path); err == nil {
		if len(data) != 32 {
			return nil, fmt.Errorf("key file %s is not 32 bytes", path)
		}
		return data, nil
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, key, 0o600); err != nil {
		return nil, err
	}
	return key, nil
}

func (s *Store) path() string { return filepath.Join(s.dir, "config.toml") }

func (s *Store) load() (config, error) {
	c := config{Profiles: map[string]profile{}}
	data, err := os.ReadFile(s.path())
	if errors.Is(err, os.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return c, err
	}
	if err := toml.Unmarshal(data, &c); err != nil {
		return c, err
	}
	if c.Profiles == nil {
		c.Profiles = map[string]profile{}
	}
	return c, nil
}

// save writes c to disk atomically: it encodes into a temp file in the same
// directory, then renames it over config.toml. This avoids leaving a
// truncated/corrupt config.toml if the process is interrupted mid-write.
func (s *Store) save(c config) error {
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(s.dir, "config-*.toml.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	if err := toml.NewEncoder(tmp).Encode(c); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return err
	}
	if err := os.Chmod(tmpPath, 0o600); err != nil {
		os.Remove(tmpPath)
		return err
	}
	if err := os.Rename(tmpPath, s.path()); err != nil {
		os.Remove(tmpPath)
		return err
	}
	return nil
}

func (s *Store) gcm() (cipher.AEAD, error) {
	if len(s.key) != 32 {
		return nil, fmt.Errorf("encryption key must be 32 bytes for AES-256, got %d", len(s.key))
	}
	block, err := aes.NewCipher(s.key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

// maxOrder returns the highest Order value among profiles, or -1 if empty.
// New profiles get maxOrder+1 so Order stays monotonically increasing even
// after a Remove+Set cycle (avoiding collisions that len(c.Profiles) would
// produce).
func maxOrder(profiles map[string]profile) int {
	max := -1
	for _, p := range profiles {
		if p.Order > max {
			max = p.Order
		}
	}
	return max
}

func (s *Store) Set(name, token string) error {
	g, err := s.gcm()
	if err != nil {
		return err
	}
	nonce := make([]byte, g.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return err
	}
	ct := g.Seal(nonce, nonce, []byte(token), nil)
	c, err := s.load()
	if err != nil {
		return err
	}
	order := maxOrder(c.Profiles) + 1
	if existing, ok := c.Profiles[name]; ok {
		order = existing.Order
	}
	c.Profiles[name] = profile{Enc: base64.StdEncoding.EncodeToString(ct), Order: order}
	if c.Default == "" {
		c.Default = name
	}
	return s.save(c)
}

func (s *Store) Get(name string) (string, error) {
	c, err := s.load()
	if err != nil {
		return "", err
	}
	p, ok := c.Profiles[name]
	if !ok {
		return "", fmt.Errorf("profile %q not found", name)
	}
	raw, err := base64.StdEncoding.DecodeString(p.Enc)
	if err != nil {
		return "", err
	}
	g, err := s.gcm()
	if err != nil {
		return "", err
	}
	ns := g.NonceSize()
	if len(raw) < ns {
		return "", errors.New("ciphertext too short")
	}
	pt, err := g.Open(nil, raw[:ns], raw[ns:], nil)
	if err != nil {
		return "", err
	}
	return string(pt), nil
}

func (s *Store) List() ([]string, error) {
	c, err := s.load()
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(c.Profiles))
	for n := range c.Profiles {
		names = append(names, n)
	}
	sort.Slice(names, func(i, j int) bool {
		return c.Profiles[names[i]].Order < c.Profiles[names[j]].Order
	})
	return names, nil
}

func (s *Store) Remove(name string) error {
	c, err := s.load()
	if err != nil {
		return err
	}
	if _, ok := c.Profiles[name]; !ok {
		return fmt.Errorf("profile %q not found", name)
	}
	delete(c.Profiles, name)
	if c.Default == name {
		c.Default = ""
		best := int(^uint(0) >> 1)
		for n, p := range c.Profiles {
			if p.Order < best {
				best, c.Default = p.Order, n
			}
		}
	}
	return s.save(c)
}

func (s *Store) DefaultProfile() (string, error) {
	c, err := s.load()
	if err != nil {
		return "", err
	}
	if c.Default == "" {
		return "", errors.New("no profiles configured")
	}
	return c.Default, nil
}
