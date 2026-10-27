# Dashboard (v1.2.0)

`GET /dashboard` — single-page UI, no CDN, no build step. The page itself
is public; every API call it makes needs the token.

## Sections

Collapsible panels (state persisted in `localStorage`): devices, history,
schedules, discovery, add-device. Each device row offers wake (with
retry), edit, clone, delete and a link jumping to its filtered history.
Empty registries, histories and scan results show empty states with a
call to action instead of blank tables.

## Token

Paste the bearer token once into the header field. It lives in the input
only (memory, never `localStorage`, never the URL) and is sent as
`Authorization: Bearer`. Live `/ws` updates reuse it via the documented
`?token=` compat param.

## Filters and tags

Device table: text search, status filter, tag chips + tag filter, column
sorting. History panel: device/trigger/result filters. Schedules list
shows computed `next_run` plus `last_run_at`/`last_result` badges.

## Actions

- Bulk "wake all down" (with confirm), per-device wake with
  `?retries=2&interval=2` and an up/down verdict.
- Export downloads `devices-export.json`; import dialog posts to
  `POST /devices/import` and reports created/skipped.
- Schedules: create (cron or `once_at`), fire now, duplicate,
  enable/disable, delete. Creation dry-run validates first.
- Discovery: CIDR input, scan, scan-status line, adopt-all with preview.

## Auto-refresh, theme, shortcuts

- Auto-refresh select: off / 10s / 30s / 60s (persisted, pauses when the
  tab is hidden, refreshes on return).
- Theme select: system / light / dark (persisted as `hello.theme`,
  applied before first paint, no flash).
- `/` focuses device search (opens the devices section); `r` refreshes.
  Both are ignored while typing or editing.
- Footer shows build version, uptime, last-refresh time and a health dot.

## PWA

`/manifest.json` makes the dashboard installable (`start_url`:
`/dashboard`, standalone display). No service worker: offline use is not
supported, the API must be reachable.
