---
name: gw2-token
description: API key info and subtokens
---

# gw2-token

Commands under `gw2 token`.

## Commands

- `gw2 token info` — Info about the configured API key
- `gw2 token subtoken [flags]` — Create a restricted-scope subtoken from the configured key
  - `--expire <string>` — ISO8601 expiration timestamp (optional)
  - `--permissions <string>` — comma-separated permission subset of the parent key (optional)
  - `--urls <string>` — comma-separated endpoint prefixes to restrict the subtoken to (optional)

See `gw2-shared` for auth setup and global flags.
