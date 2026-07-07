package commands

import (
	"sort"
	"strings"
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
		ids = append(ids, "data "+strings.Join(res.segments, " "))
	}
	ids = append(ids, "auth set", "auth list", "auth remove")
	ids = append(ids, "account")
	for _, res := range accountResources {
		ids = append(ids, "account "+strings.Join(res.segments, " "))
	}
	ids = append(ids, "character")
	ids = append(ids, "commerce prices", "commerce listings", "commerce exchange coins", "commerce exchange gems", "commerce transactions", "commerce delivery")
	for _, res := range wvwListResources {
		ids = append(ids, "wvw "+res.name)
	}
	ids = append(ids, "wvw matches", "wvw matches overview", "wvw matches scores", "wvw matches stats")
	ids = append(ids, "wvw timers", "wvw timers lockout", "wvw timers teamAssignment")
	ids = append(ids, "wvw guilds")
	for _, res := range pvpByIDsResources {
		ids = append(ids, "pvp "+res.name)
	}
	ids = append(ids, "pvp seasons leaderboards", "pvp games", "pvp standings", "pvp stats")
	ids = append(ids, "achievements", "achievements categories", "achievements groups", "achievements daily", "achievements daily tomorrow")
	ids = append(ids, "guild", "guild search", "guild permissions", "guild upgrades")
	ids = append(ids, "build", "token info", "token subtoken")
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
