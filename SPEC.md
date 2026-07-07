# gldwrs2 — SPEC

## Purpose

`gw2` is a read-only command-line client for the official [Guild Wars 2 API v2](https://wiki.guildwars2.com/wiki/API:Main): a single static Go binary, no daemon, no persistent process — one command equals one (or a small bounded batch of) HTTP GET calls. Output defaults to a token-efficient concise format so both terminal users and AI coding agents pay minimal cost per call; `--raw`/`--json` are opt-in.

The GW2 v2 API has **no write endpoints** at all, so unlike sibling CLIs in this house pattern (e.g. `dscrd`), there is no `--dry-run`/write-CAUTION machinery — every command is inherently safe to run.

Naming (locked; see the original design spec for the full rationale/table):
- **Source identity** `gldwrs2` — GitHub repo, Go module path (`github.com/howar31/gldwrs2`), local project directory. Never typed by a user.
- **User identity** `gw2` — binary, Cobra root `Use:`, Homebrew cask, npm package (`@howar31/gw2`), generated skill names, config dir (`~/.config/gw2`), env var prefix (`GW2_*`).

Original design: [`docs/superpowers/specs/2026-07-07-gw2-cli-design.md`](docs/superpowers/specs/2026-07-07-gw2-cli-design.md).

## Architecture

Layered single binary, stateless per invocation:

1. **`cmd/gw2/main.go`** — builds the Cobra root command via `commands.Root(gldwrs2.Version)`, executes it, maps errors to exit codes via `api.ExitCode`.
2. **`internal/commands`** — the command tree: one file per command group (`data.go`, `account.go`, `character.go`, `commerce.go`, `wvw.go`, `pvp.go`, `guild.go`, `achievements.go`, `token.go`, `auth.go`, `app.go`), plus shared leaf-building helpers (`leaf.go`, `tree.go`) and the coverage meta-test (`zz_coverage_test.go`). `App` (in `app.go`) carries shared dependencies (`Out`, `BaseURL`, `Profile`, `Lang`, `Mode`, `Store`, `Limiter`) and builds per-request clients via `client()` (anonymous OK) or `authedClient()` (errors up front if no key is configured).
3. **`internal/api`** — hand-written REST client for `https://api.guildwars2.com`. `Get`/`GetByIDs`/`GetAllPages` inject the pinned schema (`v=`) and `lang=` params, chunk bulk-id requests at 200/request (`MaxIDsPerRequest`), and auto-page via `X-Page-Total`. Transient statuses (429, 502–504) are retried with backoff honoring `Retry-After`. Non-2xx responses become `*APIError{StatusCode, Body, Hint}`; `403` gets a hint distinguishing an invalid key from a missing scope.
4. **`internal/auth`** — profile store at `~/.config/gw2/config.toml` (mode 0600, TOML, atomic write-then-rename). API keys are AES-256-GCM-encrypted at rest; the 32-byte key lives at `<dir>/key` (mode 0600), generated on first use. `GW2_KEYRING_BACKEND=file:<path>` overrides the key file's location — there is no OS-keyring integration (unlike `dscrd`). `DefaultDir` honors `XDG_CONFIG_HOME` before falling back to `~/.config/gw2`.
5. **`internal/output`** — `Render(w, raw, mode, concise)`: `ModeConcise` (default; falls back to pretty JSON if no concise string), `ModeRaw` (API's untouched JSON), `ModeJSON` (pretty-printed). No `table`/`jsonl` modes.
6. **`internal/resolve`** — name→id resolution: `GuildID` passes a GUID-shaped string through unchanged, otherwise resolves via `/v2/guild/search?name=`. Character names need no resolution (the API's `:id` path segment for characters *is* the name).
7. **`internal/skillgen`** — renders the `skills/` tree from the live Cobra tree via the hidden `gw2 generate-skills` command; deterministic (sorted groups/leaves) so regenerating over an unchanged tree is byte-identical.

**Data flow (typical read):** flags → `app.client()`/`app.authedClient()` (token from the auth store, base URL overridable via `GW2_API_BASE`) → optional `resolve.*` (guild name→GUID) → `client.Get`/`GetByIDs`/`GetAllPages` → `output.Render`.

External dependencies (deliberately minimal): `spf13/cobra` (command tree), `BurntSushi/toml` (config). No GW2 SDK — the REST surface is hand-written.

## Layout

```
cmd/gw2/main.go          # entry point; error -> exit-code mapping
internal/
├── api/                 # REST client, schema/lang params, chunking, pagination, rate limiter, error/hint mapping
├── auth/                # encrypted profile store (config.toml + key file)
├── commands/             # command tree: one file per group + paired _test.go
│   ├── app.go            #   App, client()/authedClient(), Root() wiring
│   ├── leaf.go           #   newByIDsListCmd, newSimpleGetRunE (shared leaf shapes)
│   ├── tree.go           #   addToTree: nested-group registration helper
│   └── zz_coverage_test.go   # meta-test: every leaf must be exercised
├── output/               # Render: concise / raw / json
├── resolve/               # name -> id resolution (guild name -> GUID)
└── skillgen/              # skills/ generator, invoked by `gw2 generate-skills`
skills/                   # GENERATED agent skills (gw2, gw2-shared, gw2-<group> per group)
npm/                      # npm wrapper (@howar31/gw2): postinstall binary download + SHA256 verify
.goreleaser.yaml          # darwin/linux x amd64/arm64 archives + homebrew_casks (cask gw2)
VERSION / version.go      # version SSOT, embedded via go:embed
docs/superpowers/specs/2026-07-07-gw2-cli-design.md  # original design spec
```

## Command groups

9 API-facing groups plus `auth` (key management, not an API endpoint) and `build` (a single meta command) — 11 top-level Cobra commands, each generating one `gw2-<name>` skill:

- **`data`** (~64 endpoints, largest but cheapest) — static game catalogs (items, colors, recipes, skins, mounts, ...). One shared engine + a `catalogResources` registration table (`internal/commands/data.go`).
- **`account`** (46 endpoints, auth) — account assets/unlocks (wallet, bank, materials, inventory, achievements, ...). Same registration-table shape as `data` (`accountResources`), a handful with bespoke `Concise()` renderers.
- **`character`** (18 endpoints, auth) — `gw2 character <name> <subresource>`; `<name>` is the character's name (the API's own `:id`), no resolution needed.
- **`commerce`** (5 endpoints, mixed auth) — trading post: `prices`, `listings` (public); `exchange` (gem↔coin, public); `transactions`, `delivery` (auth). Bespoke renderers (buy/sell spread; delivery items+coins).
- **`wvw`** (17 endpoints, mostly public) — matches, objectives, abilities, ranks, upgrades, timers, guild-per-team links.
- **`pvp`** (13 endpoints, mixed auth) — games/stats/standings (auth); seasons/amulets/ranks/heroes/rewardtracks/runes/sigils (public).
- **`guild`** (12 endpoints, mixed auth) — `gw2 guild <id|name> <sub>`; `<id|name>` resolved via `resolve.GuildID`. Public: `search`, `permissions`, `upgrades`. Auth (leader key): bare invocation, `log`, `members`, `ranks`, `stash`, `storage`, `teams`, `treasury`, `upgrades`.
- **`achievements`** (5 endpoints, public) — bare (+ `--ids`), `categories`, `groups`, `daily`, `daily tomorrow`.
- **`token`** (2 endpoints) — `info` (current key's scopes), `subtoken` (`/v2/createsubtoken`).
- **`build`** (1 endpoint, meta) — current game build id.
- **`auth`** (not an API endpoint) — `set <profile> --key`, `list`, `remove` for the local encrypted credential store.

Full endpoint-to-command mapping: the design spec §4, or `gw2 <group> --help` / the generated `skills/gw2-<group>/SKILL.md`.

## Endpoint archetypes

Coverage of 184 endpoints reduces to 5 structural archetypes (design spec §5):

1. **Catalog (~64)** — `newCatalogResourceCmd`: enumerate ids → bulk-fetch-by-ids → optional `--lang`. One table row per endpoint.
2. **Account assets (46)** — same registration-table shape, ~6 bespoke renderers.
3. **`:id`-parameterized** (characters 18, guild 12) — shared `<id-or-name>/<subresource>` positional dispatch + `resolve/` for name→id where the API needs a different id shape (guild GUID).
4. **Domain endpoints with real rendering** (commerce 5, wvw 17, pvp 13, achievements 5 ≈ 40) — hand-written concise renderers and light logic; the bulk of genuine hand-authoring.
5. **Meta (3)** — build, token info, createsubtoken.

`newByIDsListCmd`/`newSimpleGetRunE` (`internal/commands/leaf.go`) are the shared extraction of the enumerate/by-ids and plain-GET leaf shapes, reused across `data`, `wvw`, and `pvp`; `addToTree` (`internal/commands/tree.go`) builds nested command groups from a flat `segments []string` registration.

## Auth & credentials

Static API key (created at <https://account.arena.net/applications>; the CLI never creates keys), sent as `Authorization: Bearer <key>`. Storage: `~/.config/gw2/config.toml` (0600), AES-256-GCM-encrypted, key at `<dir>/key` (0600); `GW2_KEYRING_BACKEND=file:<path>` relocates the key file. Multiple named profiles (`gw2 auth set <name> --key`); `--profile` selects one, otherwise the first-set profile is the default. `gw2 token info` reports the current key's scopes; a `403` response is mapped to a hint distinguishing "invalid key" from "missing scope".

## Cross-cutting query features

Implemented once in `internal/api`, used across applicable commands:

- **Bulk ids** — `--ids=a,b,c` (chunked at 200/request, merged) or `--ids=all`/`--all` (enumerate then fetch every entry — always explicit; a bare command with no ids/`--all` never auto-dumps a catalog).
- **Pagination** — `GetAllPages` auto-pages using `X-Page-Total`.
- **Localization** — `--lang` (root persistent flag, default `en`); applies only to localizable endpoints (`catalogResource.localized`).
- **Schema pin** — `api.SchemaVersion = "2026-07-07T00:00:00Z"`, sent as `v=` on every request so responses stay pinned against schema drift regardless of when the binary runs.
- **Output** — `--raw` (untouched API JSON) / `--json` (pretty JSON) / default concise.

## Rate limiting & error handling

- **Rate limiter** (`internal/api/limiter.go`): token bucket, burst 300, refill 5/sec, matching the GW2 API's per-IP limit; the client waits for a token before every request rather than reacting to 429s alone.
- **Retries**: 429 and 502/503/504 are retried with exponential backoff (honoring `Retry-After` when present), capped, up to `maxRetries` (5).
- **Exit codes** (`api.ExitCode`): `0` ok · `3` auth (`403`) · `4` not found (`404`) · `5` rate-limited (`429`) · `1` other.

## Skill generation

`gw2 generate-skills` (hidden command) walks the live Cobra tree and writes `skills/gw2/SKILL.md` (index), `skills/gw2-shared/SKILL.md` (shared conventions), and one `skills/gw2-<group>/SKILL.md` per top-level command (11 groups). Output is deterministic — re-running over an unchanged tree produces byte-identical files — so drift is mechanically detectable. Regenerate after any command/flag change; the command is excluded from `zz_coverage_test.go`'s leaf enumeration (it's a dev utility, not a data-fetching command).

## Testing

- **HTTP isolation** — commands are tested against `httptest` servers via the `GW2_API_BASE` env override; no live calls in the default suite.
- **Coverage meta-test** — `zz_coverage_test.go`'s `leafCommands()` enumerates every leaf that must be exercised via the package's `markCovered` hook; `TestZZAllLeafCommandsCovered` fails if any leaf was never invoked by a test. Never filter a `go test -run` invocation to exclude `TestZZ...` — the registry is only complete after the full package's tests have run.
- **Fixtures** — scrubbed/synthetic ids and names only; no real API keys or account/character/guild identifiers.
- **Unit coverage** — `internal/api`'s rate limiter, id-chunking (200 boundary), and pagination are unit-tested directly (`limiter_test.go`, `getbyids_test.go`, `pages_test.go`, `retry_test.go`).

Run: `go test ./...` (full suite, includes the coverage meta-test).

## Distribution

- **goreleaser** (`.goreleaser.yaml`): builds `gw2` for darwin/linux × amd64/arm64, tar.gz archives + SHA256 checksums; `homebrew_casks` publishes cask `gw2` to `howar31/homebrew-tap` (skipped on prereleases). Release repo is pinned explicitly (`release.github: {owner: howar31, name: gldwrs2}`) since the repo name (source identity) differs from the binary/cask name (user identity).
- **npm** (`npm/`): `@howar31/gw2` wrapper — `postinstall` (`install.js`) downloads the matching platform archive from GitHub Releases and verifies its SHA256 against `checksums.txt`; `run.js` execs the extracted binary, installing on first run if missing.
- **VERSION** — the version SSOT (embedded via `go:embed` in `version.go`); goreleaser and the npm package version should track it. Regenerate `skills/` after any command-surface change and before cutting a release.

Verification for this task: `goreleaser check` passes against `.goreleaser.yaml`; `npm/package.json` is valid JSON with the correct `name`/`bin`/`supportedPlatforms`. No `goreleaser release`, `npm publish`, or `git push` has been run as part of adding this config.
