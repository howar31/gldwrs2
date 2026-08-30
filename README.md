# gw2

<!-- Status -->
[![CI](https://img.shields.io/github/actions/workflow/status/howar31/gldwrs2/ci.yml?branch=main&label=CI)](https://github.com/howar31/gldwrs2/actions/workflows/ci.yml)
[![Go 1.25+](https://img.shields.io/badge/go-1.25+-00ADD8.svg)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Conventional Commits](https://img.shields.io/badge/conventional%20commits-1.0.0-yellow)](https://www.conventionalcommits.org)
[![Dependabot](https://img.shields.io/badge/dependabot-enabled-025E8C?logo=dependabot)](.github/dependabot.yml)

<!-- Release & distribution -->
[![GitHub release](https://img.shields.io/github/v/release/howar31/gldwrs2)](https://github.com/howar31/gldwrs2/releases)
[![GitHub release downloads](https://img.shields.io/github/downloads/howar31/gldwrs2/total?label=release%20downloads)](https://github.com/howar31/gldwrs2/releases)
[![npm version](https://img.shields.io/npm/v/@howar31/gw2)](https://www.npmjs.com/package/@howar31/gw2)
[![npm downloads](https://img.shields.io/npm/dm/@howar31/gw2?label=npm%20downloads)](https://www.npmjs.com/package/@howar31/gw2)

<!-- Activity & community -->
[![Last commit](https://img.shields.io/github/last-commit/howar31/gldwrs2)](https://github.com/howar31/gldwrs2/commits/main)
[![Open issues](https://img.shields.io/github/issues/howar31/gldwrs2)](https://github.com/howar31/gldwrs2/issues)
[![Stars](https://img.shields.io/github/stars/howar31/gldwrs2)](https://github.com/howar31/gldwrs2/stargazers)
[![Sponsor](https://img.shields.io/badge/Sponsor-donate.howar31.com-b4532c?logo=data:image/svg%2Bxml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHZpZXdCb3g9IjAgMCAyNCAyNCI+PHBhdGggZmlsbD0iI2ZmZiIgZD0iTTIwLjg0IDQuNjFhNS41IDUuNSAwIDAgMC03Ljc4IDBMMTIgNS42N2wtMS4wNi0xLjA2YTUuNSA1LjUgMCAwIDAtNy43OCA3Ljc4bDEuMDYgMS4wNkwxMiAyMS4yM2w3Ljc4LTcuNzggMS4wNi0xLjA2YTUuNSA1LjUgMCAwIDAgMC03Ljc4eiIvPjwvc3ZnPg==)](https://donate.howar31.com/)

**Read-only Guild Wars 2 CLI.** One command = one GET call against the official [Guild Wars 2 API v2](https://wiki.guildwars2.com/wiki/API:Main), with token-efficient output built for AI coding agents and humans who live in the terminal. Account assets, characters, the trading post, WvW, PvP, guilds, and every static game catalog.

> **Naming note:** the GitHub repo is `gldwrs2`; the command you install and run is `gw2`. The repo name never appears on the command line.

```console
$ gw2 data items --ids 19721
19721	Glob of Ectoplasm

$ gw2 commerce prices 19721
19721	buy 28s 81c x668716	sell 29s 29c x1774012	spread 48c

$ gw2 build
build 204489
```

## Why

- **Token-efficient by default.** The concise format renders one line per item, so a lookup costs an agent a few dozen tokens instead of a full JSON envelope. `--raw` (untouched API JSON) and `--json` (pretty-printed) are one flag away.
- **Stateless one-shot calls.** Every invocation is a single HTTP round-trip (plus a built-in rate limiter and automatic retries that honor `Retry-After`). No daemon, no session, no cache to go stale.
- **Full API coverage.** Every live `/v2` endpoint is wrapped as a command, including wiki-documented endpoints the API's own root listing omits (emblem layer catalogs, continent floors, the Wizard's Vault season).
- **Safe by construction.** The GW2 API v2 has no write endpoints at all, so there is nothing here that can change account or game state. API keys are AES-256-GCM-encrypted at rest and never printed.

## Read-only by design

Guild Wars 2's API only exposes reads, which makes `gw2` a query, lookup, and monitoring client with no mutation surface and no need for dry-run guards. API keys can only be created on the [ArenaNet account site](https://account.arena.net/applications); `gw2` stores one you already generated and does nothing else with it. Public data (all of `gw2 data`, trading post prices, WvW matches, and more) needs no key at all.

## Install

```bash
# Homebrew (macOS / Linux)
brew install howar31/tap/gw2

# npm (downloads the platform binary, verifies SHA256)
npm install -g @howar31/gw2

# Go
go install github.com/howar31/gldwrs2/cmd/gw2@latest

# or grab a binary from GitHub Releases, or build from source:
git clone https://github.com/howar31/gldwrs2.git && cd gldwrs2 && go build -o gw2 ./cmd/gw2
```

## API key setup (one-time)

Commands under `gw2 account`, `gw2 character`, `gw2 token`, and the authed parts of `gw2 commerce`, `gw2 pvp`, and `gw2 guild` need an API key. Create one at <https://account.arena.net/applications> (pick the scopes you want; `account` is the mandatory base), then store it under a profile name:

```bash
gw2 auth set main --key <KEY>
gw2 token info          # confirms the key works and lists its scopes
```

Keys live AES-256-GCM-encrypted in `~/.config/gw2/config.toml` (mode 0600). Multiple named profiles are supported; the first profile stored becomes the default and `--profile <name>` selects another. A `401`/`403` from the API comes back with a hint that tells you whether the key is invalid or just missing a scope.

## Command tour

```text
gw2 data          62 static catalogs: items, colors, recipes, skins, continents (+floors), ...
gw2 account       wallet, bank, materials, inventory, achievements, unlocks, ...
gw2 character     <name> [core|equipment|inventory|crafting|sab|...]
gw2 commerce      prices, listings, exchange, transactions, delivery
gw2 wvw           matches (+deep per-team stats), objectives, ranks, timers, guilds
gw2 pvp           stats, games, standings, seasons (+leaderboards), amulets, heroes
gw2 guild         <id|name> [log|members|ranks|stash|treasury|...], search, permissions
gw2 achievements  bare, categories, groups, daily
gw2 token         info, subtoken
gw2 build         current game build id
gw2 auth          set, list, remove (local key management)
```

`--help` on any level gives the full reference. Catalog commands share one shape: bare invocation lists ids only, `--ids a,b,c` fetches specific entries, `--all` explicitly fetches everything.

### Global flags

| Flag | Meaning |
|---|---|
| `--raw` | print the API's untouched JSON |
| `--json` | pretty-print the JSON |
| `--lang <code>` | localized names where supported (default `en`) |
| `--profile <name>` | which stored API key to use |

### Exit codes

`0` ok · `3` auth · `4` not found · `5` rate-limited · `1` other.

## For AI agents

The [`skills/`](skills/) directory contains a generated skill tree (an index, a shared reference, and one skill per command group) that teaches an agent the full command surface. It is generated from the live command tree (`gw2 generate-skills`) and CI fails if it drifts.

```bash
npx skills add https://github.com/howar31/gldwrs2
```

## Security

- API keys are stored AES-256-GCM-encrypted in `~/.config/gw2/config.toml` (mode 0600); the encryption key lives in a 0600 file next to it (`GW2_KEYRING_BACKEND=file:<path>` relocates it).
- Keys are never printed or logged. `gw2 auth list` and `gw2 token info` report names and scopes, never values.
- GW2 API keys are themselves read-only credentials: even a leaked key cannot change anything on the account, though it can read whatever its scopes allow.

## Development

```bash
go build -o gw2 ./cmd/gw2      # version comes from the committed VERSION file
go test ./...                  # includes the command-coverage meta-test
go run ./cmd/gw2 generate-skills
scripts/smoke.sh               # live smoke against the real API (authed part is key-gated)
```

Every leaf command must be exercised by at least one test; a meta-test walks the command tree and fails CI when coverage is missing.

Releases are driven by the `VERSION` file: bumping it on `main` tags `v<VERSION>`, builds binaries via goreleaser, updates the Homebrew cask, and publishes `@howar31/gw2`.

## License

[MIT](LICENSE)
