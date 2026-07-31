---
name: gw2-wvw
description: "World vs World matches, objectives, and rankings"
---

# gw2-wvw

Commands under `gw2 wvw`.

## Commands

- `gw2 wvw abilities [flags]` — Fetch wvw abilities
  - `--all` — fetch every entry (explicit; may be large)
  - `--ids <string>` — comma-separated ids (omit to list ids)
- `gw2 wvw guilds [flags]` — WvW guild claims by region
  - `--region <string>` — filter to one region (e.g. na, eu)
- `gw2 wvw matches [flags]` — WvW match ids, or full match data by id/world
  - `--ids <string>` — comma-separated match ids (in addition to any positional ids)
  - `--world <int>` — filter to the match containing this world id
- `gw2 wvw matches overview [flags]` — Fetch wvw matches overview
  - `--ids <string>` — comma-separated match ids
  - `--world <int>` — filter to the match containing this world id
- `gw2 wvw matches scores [flags]` — Fetch wvw matches scores
  - `--ids <string>` — comma-separated match ids
  - `--world <int>` — filter to the match containing this world id
- `gw2 wvw matches stats [flags]` — Fetch wvw matches stats
  - `--ids <string>` — comma-separated match ids
  - `--world <int>` — filter to the match containing this world id
- `gw2 wvw matches stats guilds` — One guild's stats within a WvW match
- `gw2 wvw matches stats top` — Top guilds by kdr/kills for one team in a WvW match
- `gw2 wvw objectives [flags]` — Fetch wvw objectives
  - `--all` — fetch every entry (explicit; may be large)
  - `--ids <string>` — comma-separated ids (omit to list ids)
- `gw2 wvw ranks [flags]` — Fetch wvw ranks
  - `--all` — fetch every entry (explicit; may be large)
  - `--ids <string>` — comma-separated ids (omit to list ids)
- `gw2 wvw rewardtracks [flags]` — Fetch wvw rewardtracks
  - `--all` — fetch every entry (explicit; may be large)
  - `--ids <string>` — comma-separated ids (omit to list ids)
- `gw2 wvw timers` — WvW map rotation and lockout timers
- `gw2 wvw timers lockout` — Fetch wvw timers lockout
- `gw2 wvw timers teamAssignment` — Fetch wvw timers teamAssignment
- `gw2 wvw upgrades [flags]` — Fetch wvw upgrades
  - `--all` — fetch every entry (explicit; may be large)
  - `--ids <string>` — comma-separated ids (omit to list ids)

See `gw2-shared` for auth setup and global flags.
