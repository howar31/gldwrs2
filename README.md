# gldwrs2 — the gw2 CLI

A read-only command-line client for the official [Guild Wars 2 API v2](https://wiki.guildwars2.com/wiki/API:Main): account assets and unlocks, characters, the trading post, World vs World, PvP, guilds, and the game's static reference data — with token-efficient output built for both terminal use and AI coding agents.

> **Naming note:** the GitHub repo is `gldwrs2`; the command you install and run is `gw2`. The repo name never appears on the command line.

## Install

```bash
# Homebrew (macOS/Linux)
brew install howar31/tap/gw2

# npm (downloads the platform binary, verifies SHA256)
npm install -g @howar31/gw2

# or build from source
git clone https://github.com/howar31/gldwrs2.git && cd gldwrs2
go build -o gw2 ./cmd/gw2
```

## Quick usage

```bash
gw2 data items --ids 24
gw2 commerce prices 24
gw2 account wallet
```

`--raw` prints the API's untouched JSON; `--json` pretty-prints; the default is a concise, token-efficient one-line-per-item format. `--help` at any level gives the full command reference.

## Auth setup

Most `gw2 account`/`gw2 character`/`gw2 guild <id>` commands need an API key. Create one at <https://account.arena.net/applications> (choose the scopes you need — `account` is mandatory), then store it:

```bash
gw2 auth set main --key <KEY>
gw2 token info          # confirms the key and its scopes
```

Keys are stored AES-256-GCM-encrypted at `~/.config/gw2/config.toml` (mode 0600) and are never printed or logged. Multiple named profiles are supported (`--profile <name>`); public endpoints (most of `gw2 data`, `gw2 commerce prices`, etc.) work with no key at all.

## Read-only

The GW2 v2 API has no write/mutation endpoints, so `gw2` is deliberately a query/lookup/monitoring client only — there is nothing here that changes account or game state. API keys can only be created on the ArenaNet website; `gw2` cannot create or manage them beyond storing one you already generated.

See the [official GW2 API wiki](https://wiki.guildwars2.com/wiki/API:Main) for endpoint reference, and [`skills/`](skills/) for the generated agent-facing command reference (for AI coding agents such as Claude Code).
