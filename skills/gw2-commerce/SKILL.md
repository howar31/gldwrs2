---
name: gw2-commerce
description: Trading post prices, listings, and your orders
---

# gw2-commerce

Commands under `gw2 commerce`.

## Commands

- `gw2 commerce delivery` — Items and coins waiting in your delivery box
- `gw2 commerce exchange coins [flags]` — Exchange rate: commerce exchange coins
  - `--quantity <int>` — amount to exchange (required, > 0)
- `gw2 commerce exchange gems [flags]` — Exchange rate: commerce exchange gems
  - `--quantity <int>` — amount to exchange (required, > 0)
- `gw2 commerce listings [flags]` — Fetch commerce listings
  - `--ids <string>` — comma-separated ids (in addition to any positional ids)
- `gw2 commerce prices [flags]` — Fetch commerce prices
  - `--ids <string>` — comma-separated ids (in addition to any positional ids)
- `gw2 commerce transactions` — Your trading post transactions

See `gw2-shared` for auth setup and global flags.
