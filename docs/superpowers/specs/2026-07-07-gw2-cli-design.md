# gw2 — Guild Wars 2 API CLI — Design Spec

Date: 2026-07-07
Status: Draft for review
Repo (source): `github.com/howar31/gldwrs2`
Command (user-facing): `gw2`

## 1. Goal

A command-line client that provides **complete coverage of the Guild Wars 2 public API v2** (184 endpoints), following the established house pattern of the `dscrd` / `slk` CLIs: a Go binary with Cobra command groups, token-efficient concise output, encrypted local credentials, auto-generated agent skills, and Homebrew + npm distribution via goreleaser.

The GW2 v2 API is **read-only** (no write endpoints), so this tool is a query / lookup / monitoring client. There is deliberately no write / mutation surface, and therefore none of the `--dry-run` / write-CAUTION machinery that `dscrd` needs.

## 2. Naming (locked)

Two identities, cleanly separated. `gldwrs2` (devoweled, matches the `slk` / `dscrd` family, distinctive and greppable) is the **source identity**; `gw2` (the universal community abbreviation, short to type) is the **user-facing identity**.

| Surface | Name | Side |
| --- | --- | --- |
| GitHub repo | `gldwrs2` | source |
| Go module path (`go.mod` + every internal import) | `github.com/howar31/gldwrs2` | source |
| Local project directory | `/opt/projects/gldwrs2` | source |
| git remote / goreleaser `owner`+`name` | `gldwrs2` | source |
| Binary / command | `gw2` | user |
| Binary entry package | `cmd/gw2/main.go` | user |
| Cobra root command (`Use:`) | `gw2` | user |
| Homebrew cask (own tap `howar31/homebrew-tap`) | `gw2` | user |
| npm package | `@howar31/gw2` | user |
| Generated skills tree | `skills/gw2/`, `gw2-<group>` | user |
| Config directory | `~/.config/gw2/config.toml` | user |
| Env var prefix | `GW2_*` (e.g. `GW2_API_BASE`, `GW2_KEYRING_BACKEND`) | user |

Rule for maintainers: `gldwrs2` appears **only** on the source side (repo URL, Go import path, local dir). Everything a user installs, runs, or reads at runtime is `gw2`.

Bridging (so repo ≠ command never confuses):
- `gw2 --version` and `gw2 --help` footer print `Source: github.com/howar31/gldwrs2`.
- README H1: `gldwrs2 — the gw2 CLI`.
- Homebrew cask and npm package are both named `gw2`, so install and run commands match; `gldwrs2` is never typed by a user.

Precedent for repo ≠ command: `ripgrep`→`rg`, `fd-find`→`fd`, `kubernetes`→`kubectl`, `neovim`→`nvim`.

## 3. Architecture

Mirror the `dscrd` layout.

```
cmd/gw2/main.go                 # entry; wires cobra root + version
internal/
  api/                          # HTTP client, rate limiter, errors, bulk/pagination, schema+lang params
  auth/                         # API key storage (encrypted config.toml), Bearer injection, key profiles
  commands/                     # one file per command group (+ paired _test.go)
  output/                       # Concise() rendering, --raw / --json, tables
  resolve/                      # name→id resolution (character name, guild name→GUID, item name→id)
  skillgen/                     # `gw2 generate-skills` → skills/ tree
VERSION                         # version SSOT (goreleaser reads it)
.goreleaser.yaml                # homebrew_casks (cask gw2) + build (binary gw2)
npm/                            # @howar31/gw2 wrapper (run.js)
skills/                         # generated: gw2/ index + gw2-<group> per group
SPEC.md CLAUDE.md README.md PLAN.md
```

Each command group is one file under `internal/commands/`, with a paired `_test.go`, and every leaf command must be exercised via a coverage meta-test (`zz_coverage_test.go`) — same discipline as `dscrd`.

## 4. Command tree — 9 groups

184 endpoints collapse into 9 top-level command groups. Invocation shapes below.

### 4.1 `gw2 data <resource> [--ids …] [--lang] [--page]` — static game data (~64 endpoints)

The largest group but the cheapest to build: every endpoint here follows the identical `enumerate ids → bulk fetch by ids → localizable` pattern. Implemented as **one shared engine + a registration table**; each resource is one row (name + path + optional concise renderer).

Resources (each a subcommand of `data`): `items`, `itemstats`, `skins`, `colors`, `recipes` (+ `recipes/search`), `skills`, `traits`, `specializations`, `professions`, `races`, `pets`, `legends`, `masteries`, `materials`, `currencies`, `titles`, `minis`, `finishers`, `emotes`, `outfits`, `gliders`, `mailcarriers`, `novelties`, `quaggans`, `quests`, `raids`, `dungeons`, `dailycrafting`, `worldbosses`, `worlds`, `maps`, `continents`, `mapchests`, `backstory` (`answers` / `questions`), `stories` (+ `seasons`), `mounts` (`skins` / `types`), `home` (`cats` / `nodes`), `homestead` (`decorations` [+ `categories`] / `glyphs`), `wizardsvault` (`listings` / `objectives`), `gemstore-catalog`, `jadebots`, `skiffs`, `legendaryarmory`, `files`, `logos`, `emblem`, `vendors`, `adventures` (+ `:id/leaderboards/:board/:region`), `events`, `events-state`.

Example: `gw2 data items --ids=24,68` · `gw2 data recipes search --input=19684` · `gw2 data colors --lang=zh`.

### 4.2 `gw2 account <subresource>` — account assets / unlocks (46 endpoints, auth)

Mostly "GET the path, print concise". Implemented as a registration table too; a handful get bespoke `Concise()` renderers (`wallet`, `bank`, `materials`, `inventory`, `achievements`, `masteries`).

Subresources: bare `account`, `achievements`, `bank`, `buildstorage`, `dailycrafting`, `dungeons`, `dyes`, `emotes`, `finishers`, `gliders`, `home` (`cats` / `nodes`), `homestead` (`decorations` / `glyphs`), `inventory`, `jadebots`, `legendaryarmory`, `luck`, `mail`, `mailcarriers`, `mapchests`, `masteries`, `mastery-points`, `materials`, `minis`, `mounts` (`skins` / `types`), `novelties`, `outfits`, `progression`, `pvp-heroes`, `raids`, `recipes`, `skiffs`, `skins`, `titles`, `wallet`, `wizardsvault` (`daily` / `weekly` / `special` / `listings`), `worldbosses`, `wvw`.

Example: `gw2 account wallet` · `gw2 account materials` · `gw2 account bank`.

### 4.3 `gw2 character <name> <subresource>` — 18 endpoints, auth

`:id` is the character **name** (may contain spaces → quote it; `resolve/` URL-encodes). Shared `<name>/<subresource>` handler.

Subresources: bare list (`gw2 character` / `gw2 character list`), `core`, `backstory`, `crafting`, `dungeons`, `equipment`, `equipmenttabs` (+ `active`), `buildtabs` (+ `active`), `heropoints`, `inventory`, `quests`, `recipes`, `sab`, `skills`, `specializations`, `training`.

Example: `gw2 character "Alara Nightbreeze" equipment`.

### 4.4 `gw2 commerce <sub>` — trading post, 5 endpoints (mixed auth)

`prices`, `listings` (public); `exchange` (gem↔coin); `transactions`, `delivery` (auth). High-value → bespoke concise renderers (buy/sell/spread; delivery items+coins).

Example: `gw2 commerce prices 24` · `gw2 commerce exchange coins --quantity=100000`.

### 4.5 `gw2 wvw <sub>` — 17 endpoints (mostly public)

`matches` (+ `overview` / `scores` / `stats` [+ per-guild, per-team top kdr/kills]), `objectives`, `abilities`, `ranks`, `upgrades`, `rewardtracks`, `guilds` (+ `:region`), `timers` (+ `lockout` / `teamAssignment`).

### 4.6 `gw2 pvp <sub>` — 13 endpoints (mixed auth)

`games`, `stats`, `standings` (auth); `seasons` (+ `:id/leaderboards/:board/:region`), `amulets`, `ranks`, `heroes`, `rewardtracks`, `runes`, `sigils` (public).

### 4.7 `gw2 guild <id|name> <sub>` — 12 endpoints (mixed auth)

`:id` is a guild GUID; `resolve/` maps a guild name via `guild/search`. Public: `search`, `permissions`, `upgrades`. Auth (guild leader key): bare `guild <id>`, `log`, `members`, `ranks`, `stash`, `storage`, `teams`, `treasury`, `upgrades`.

Example: `gw2 guild search "Edge of the Mists"` · `gw2 guild <guid> members`.

### 4.8 `gw2 achievements <sub>` — 5 endpoints (public)

bare `achievements` (+ `--ids`), `categories`, `groups`, `daily`, `daily tomorrow`.

### 4.9 `gw2 token <sub>` + build info — meta (3 endpoints)

`gw2 token info` (`/v2/tokeninfo` — shows the current key's scopes), `gw2 token subtoken` (`/v2/createsubtoken`), `gw2 build` (`/v2/build` — current game build id).

Plus auth management (not API endpoints): `gw2 auth set` / `gw2 auth list` / `gw2 auth remove` for storing keys.

## 5. Endpoint archetypes → implementation strategy

Full coverage is **not** 184 hand-written commands. The endpoints reduce to 5 structural archetypes:

1. **Catalog (~64)** — one shared engine (`enumerate → bulk-by-ids → --lang`) + a registration table. Auto-generatable; each endpoint is one table row. (§4.1)
2. **Account assets (46)** — registration table of `(subcommand, path)`; ~6 bespoke `Concise()` renderers. (§4.2)
3. **`:id`-parameterized (characters 18, guild 12, some wvw/pvp/adventures)** — shared `<id>/<subresource>` handler + `resolve/` for name→id. (§4.3, §4.7)
4. **Domain endpoints with real rendering (commerce 5, wvw 17, pvp 13, achievements 5 ≈ 40)** — the bulk of hand-written work: concise renderers and light logic. (§4.4–4.8)
5. **Meta (3)** — build / tokeninfo / createsubtoken. (§4.9)

So genuine hand-authoring concentrates on archetype 4 (~40 endpoints) plus a few account renderers; the other ~110 endpoints are covered by two shared engines (catalog engine + account registry).

## 6. Authentication & credentials

- **Model**: GW2 uses a static API key (created by the user at `account.arena.net/applications`; the CLI cannot create keys — that is a website action). Sent as `Authorization: Bearer <key>`.
- **Storage**: `~/.config/gw2/config.toml` (0600), key encrypted at rest AES-256-GCM, keyring backend per `GW2_KEYRING_BACKEND` — same mechanism as `dscrd`. Never print or log the key.
- **Profiles**: support multiple named keys (`gw2 auth set <name>`, `--profile <name>`, default profile), since a user may hold keys with different scopes.
- **Scopes**: an API key carries fixed scopes chosen at creation (`account` mandatory, plus `characters`, `inventories`, `wallet`, `tradingpost`, `pvp`, `wvw`, `guilds`, `progression`, `unlocks`, `builds`). Commands needing a scope the key lacks will get `403`; the error layer maps this to a hint naming the missing permission. `gw2 token info` reports the current key's scopes.
- **Subtokens**: `gw2 token subtoken` wraps `/v2/createsubtoken` (expiring, scope-limited derived key).

## 7. Cross-cutting query features

Implemented once in `internal/api`, available across all applicable commands:

- **Bulk ids**: `--ids=a,b,c` (client chunks at the API max of 200 per request, merges results). `--ids=all` where supported.
- **Pagination**: `--page` / `--page-size`; the client can auto-page (`--all`) using `X-Page-Total` / `X-Result-Total` headers.
- **Localization**: `--lang` ∈ {`en`,`es`,`de`,`fr`,`zh`} (default `en`); applies only to localizable endpoints.
- **Schema version**: `--schema <ISO8601|latest>` → `v=` param, so responses are pinned against schema drift. A sensible default schema date is baked in and documented.
- **Output**: default is token-efficient `Concise()` per resource type; `--raw` passes the API's JSON through untouched; `--json` pretty-prints. Consistent with `dscrd`'s `Concise()` convention.

## 8. Rate limiting & error handling

- **Rate limit**: per-IP token bucket, burst 300, refill 5/sec (300/min). The client throttles proactively to stay under it and retries `429` with backoff. Bulk-by-ids and auto-paging are the primary request-count reducers.
- **Transient invalid key**: per GW2 best practices, a key can spuriously read as invalid; retry with backoff before surfacing an auth error.
- **HTTP mapping → exit codes** (mirror `dscrd`): `0` ok · `3` auth (`403`/invalid key) · `4` not found (`404`) · `5` rate-limited (`429`) · `1` other. `502`/`503`/`504` (upstream / endpoint disabled / timeout) are transient → retried, then exit `1` with a clear message. Mappings live in `internal/api/errors.go`.
- Enum tolerance: parsing must not hard-fail on unknown enum values (the API adds them without schema bumps).

## 9. Skill generation

`gw2 generate-skills` produces `skills/gw2/` (index) linking `gw2-shared` + one `gw2-<group>` skill per command group (`gw2-data`, `gw2-account`, `gw2-character`, `gw2-commerce`, `gw2-wvw`, `gw2-pvp`, `gw2-guild`, `gw2-achievements`, `gw2-token`). Generated from the command tree + annotations; never hand-edited. A CI job guards drift. Command annotations (e.g. `gw2Endpoint`, `scope`, `auth`) drive both skills and behavior.

## 10. Testing

- **HTTP isolation**: tests run against `httptest` servers via a `GW2_API_BASE` override; no live calls in unit tests.
- **Coverage meta-test**: `zz_coverage_test.go` fails if any leaf command is not exercised via `runCmd`/`markCovered`.
- **Fixtures**: scrubbed identifiers only — no real API keys, no real account/character/guild ids. Use placeholder names and synthetic ids.
- **Live smoke**: `scripts/smoke.sh`, env-gated on a real key, run manually / in a gated CI job — not in the default suite.
- **Rate-limiter / chunker / pagination**: unit-tested against the 200-id and 300/min boundaries.

## 11. Distribution

- **goreleaser**: builds binary `gw2`; `homebrew_casks` publishes cask `gw2` to `howar31/homebrew-tap`.
- **npm**: `@howar31/gw2` wrapper (`bin: { gw2: run.js }`).
- **VERSION** file is the version SSOT; bumping it (its own `chore(release): X.Y.Z` change) drives tag + goreleaser + npm publish. Regenerate `skills/` after any command change.

## 12. Out of scope

- Any write / mutation (the v2 API has none).
- v1 (legacy) endpoints.
- Rendering game assets / maps into images; the `render` service and MumbleLink/local-client integration are not part of this CLI.
- Creating API keys (website-only action).

## 13. Verification approach

1. Unit tests per command group against `httptest` fixtures (default `go test ./...`), including the coverage meta-test.
2. `internal/api` unit tests for the rate limiter, 200-id chunking, auto-pagination, and error→exit-code mapping.
3. Manual live smoke (`scripts/smoke.sh`) against a real key across one endpoint per archetype: a `data` catalog fetch, an authed `account` fetch, a `character` sub-resource, a `commerce` price, a public `wvw`/`achievements` fetch.
4. `goreleaser release --snapshot --clean` dry-run to verify the cask + binary build before any real release.

## 14. Resolved decisions

- **Schema pin (`v=`)** — pinned to `2026-07-07T00:00:00Z` (the development date), stored as the documented constant `GW2_SCHEMA_VERSION`. Verified against the live API: a literal date is accepted (`/v2/build?v=2026-07-07T00:00:00Z` → 200) and resolves to the schema current as of that date. Because the date is fixed, later schema publishes by ArenaNet are ignored, keeping struct parsing stable regardless of when the user runs the tool. Bumping it is a deliberate change paired with struct updates.
- **`data` naming** — mirror the API's own nesting: nest where the path nests (`gw2 data mounts skins`, `gw2 data homestead decorations categories`), hyphenate only flat leaf paths (`gw2 data mastery-points`, `gw2 data gemstore-catalog`).
- **No implicit full dumps** — a bare `gw2 data <resource>` (no ids) never auto-fetches the entire catalog. It shows the id list / a hint; fetching all entries requires explicit `--ids=all` / `--all`. Rationale: catalogs range from ~600 entries (`colors`) to 90,000+ (`items`); an accidental bare command must not fire hundreds of requests or flood the terminal.
