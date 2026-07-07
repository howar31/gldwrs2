---
name: gw2-shared
description: Shared gw2 CLI conventions: auth, global flags, output modes, exit codes.
---

# gw2-shared (v0.1.0)

Conventions shared by every gw2 command.

## Global flags

Available on every command:

- `--profile <name>` — credential profile to use (default: whichever profile
  was most recently set via `gw2 auth set`)
- `--lang en|es|de|fr|zh` — response language for localized data (default: en)
- `--raw` — print the API's raw JSON response, unmodified
- `--json` — print the response as pretty-indented JSON

With neither `--raw` nor `--json`, output is the concise, human-readable form.

## Authentication

The GW2 API is read-only through this CLI: no gw2 command mutates account or
game state. Endpoints scoped to an account or guild (account, character,
commerce transactions/delivery, pvp standings/stats, token, and the guild
leader/officer subresources) need an API key with the right scope; everything
else works anonymously.

Generate a key at https://account.arena.net/applications, then store it:

    gw2 auth set <profile> --key <APIKEY>

Keys are encrypted (AES-256-GCM) and stored at `~/.config/gw2/config.toml`;
`gw2 auth list` never prints the key value itself. Pass `--profile <name>` to
select a stored key other than the default.

## Output modes

- concise (default) — a short, human-readable summary of the response
- `--raw` — the API's JSON response, unmodified
- `--json` — the same data, pretty-printed

## Exit codes

- `0` — success
- `3` — authentication/permission error (API returned 403)
- `4` — not found (API returned 404)
- `5` — rate-limited (API returned 429)
- `1` — any other error

## Listing vs. fetching

Endpoints that follow the "enumerate ids, fetch by ids" shape (most of
gw2-data, gw2-account, gw2-wvw, gw2-pvp, and similar) print only the id list
when called bare -- they never auto-dump every entry. Pass `--ids
<comma-separated ids>` to fetch specific entries, or `--all` to fetch every
entry (an explicit opt-in, since some of these lists are large).
