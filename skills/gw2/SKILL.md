---
name: gw2
description: "Guild Wars 2 API CLI — read-only client for accounts, trading post, WvW, PvP, guilds, and game data. Index of gw2-* skills."
---

# gw2 (v0.1.1)

Guild Wars 2 API CLI: a read-only command-line client for the official GW2
API, covering account assets and unlocks, characters, the trading post,
World vs World, PvP, guilds, and the game's static reference data. Each
gw2-<group> skill below documents one top-level command group.

## Command groups

- `gw2-account` — Your account assets and unlocks
- `gw2-achievements` — Achievements, categories, groups, and dailies
- `gw2-auth` — Manage stored API keys
- `gw2-build` — Current game build id
- `gw2-character` — Your characters and their details
- `gw2-commerce` — Trading post prices, listings, and your orders
- `gw2-data` — Static game data (items, colors, recipes, ...)
- `gw2-guild` — Guild info, roster, and treasury
- `gw2-pvp` — PvP stats, seasons, and reward tracks
- `gw2-token` — API key info and subtokens
- `gw2-wvw` — World vs World matches, objectives, and rankings

See `gw2-shared` for authentication setup, global flags, output modes, and
exit codes shared by every command above.
