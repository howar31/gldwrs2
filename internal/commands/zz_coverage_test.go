package commands

import (
	"sort"
	"sync"
	"testing"
)

var (
	coverMu   sync.Mutex
	coverSeen = map[string]bool{}
)

func init() { markCovered = func(id string) { coverMu.Lock(); coverSeen[id] = true; coverMu.Unlock() } }

// leafCommands enumerates every leaf command that must be exercised by a test.
// Extend this as command groups are added.
func leafCommands() []string {
	var ids []string
	for _, res := range catalogResources {
		ids = append(ids, "data "+res.name)
	}
	sort.Strings(ids)
	return ids
}

func TestZZAllLeafCommandsCovered(t *testing.T) {
	coverMu.Lock()
	defer coverMu.Unlock()
	for _, id := range leafCommands() {
		if !coverSeen[id] {
			t.Errorf("leaf command %q was never exercised by a test", id)
		}
	}
}
