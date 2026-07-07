// Package resolve maps human-friendly identifiers (e.g. a guild name) to
// the GW2 API's canonical id shape, so commands can accept either form
// instead of forcing the user to look up a GUID first.
package resolve

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"

	"github.com/howar31/gldwrs2/internal/api"
)

// guildIDPattern matches a guild id's GUID shape (8-4-4-4-12 hex digits),
// e.g. "4BBB0AC5-DA0D-E011-8709-406186D57C3C".
var guildIDPattern = regexp.MustCompile(`^[0-9A-Fa-f]{8}-([0-9A-Fa-f]{4}-){3}[0-9A-Fa-f]{12}$`)

// GuildID resolves nameOrID to a guild GUID. If nameOrID already looks like
// a GUID, it's returned unchanged with no HTTP call -- it's already the
// canonical id, and /v2/guild/search only accepts a name, not an id, so
// searching for it would be both wasteful and semantically wrong. Otherwise
// nameOrID is treated as a guild name and resolved via
// GET /v2/guild/search?name=<nameOrID>, which returns a JSON array of guild
// id strings for that name (occasionally more than one, since guild names
// aren't unique across worlds/regions); the first match is returned, or an
// error if the search comes back empty.
func GuildID(ctx context.Context, c *api.Client, nameOrID string) (string, error) {
	if guildIDPattern.MatchString(nameOrID) {
		return nameOrID, nil
	}
	raw, err := c.Get(ctx, "/v2/guild/search", url.Values{"name": {nameOrID}})
	if err != nil {
		return "", err
	}
	var ids []string
	if err := json.Unmarshal(raw, &ids); err != nil {
		return "", err
	}
	if len(ids) == 0 {
		return "", fmt.Errorf("no guild found named %q", nameOrID)
	}
	return ids[0], nil
}
