# API reference (v1.2.0)

The machine-readable contract is `GET /openapi.yaml` (version `1.2.0`,
embedded from `internal/spec/openapi.yaml`); human browsing at
`GET /docs` (Swagger UI). This page summarizes conventions and lists every
endpoint by domain. When in doubt, the OpenAPI document wins.

## Auth

`Authorization: Bearer <token>` on everything except `GET /`, `/health`,
`/docs`, `/openapi.yaml`, `/dashboard`, `/manifest.json`. `API_TOKEN` needs
min 16 chars (`[A-Za-z0-9_-]`, `change-me` rejected); `API_TOKEN_FILE`
supplies it from a file only when `API_TOKEN` is empty. `/ws` accepts
`?token=` for compat, but prefer the header.

## Errors

Failures use `application/problem+json` (RFC 9457):

```json
{"type": "about:blank", "title": "not found", "status": 404,
 "detail": "storage: device not found"}
```

Validation failures return `422` with an `errors` map; `DELETE` returns
`204 No Content`; throttles and cooldowns return `429` with `Retry-After`.

## Conventions

- `POST /wake` and `POST /wake/:macAddress` are canonical; the `GET`
  variants remain as deprecated legacy.
- `PUT /devices/:id` is a full replacement; `PATCH /devices/:id` is partial.
- List endpoints paginate with `?page=&per_page=` and return a `total`.
- `?dry_run=1` previews without side effects (wake, adopt, schedule
  create, restore).
- Every response carries `X-Request-ID`. Throttle rejections add
  `X-RateLimit-Limit/Remaining/Reset`.
- `GET /devices` sends an `ETag`; replays with `If-None-Match` get `304`.

## System

| Method | Path | Notes |
|--------|------|-------|
| GET | `/` | Welcome + `defaultMac` (public) |
| GET | `/health` | Liveness + version/uptime/counts (public) |
| GET | `/version` | Build version |
| GET | `/config` | Redacted operational settings (no token) |
| GET | `/metrics` | Prometheus text (behind auth) |
| GET | `/docs` | Swagger UI (public) |
| GET | `/openapi.yaml` | OpenAPI 3.1 document (public) |
| GET | `/dashboard` | Web UI (public page, API calls need token) |
| GET | `/manifest.json` | PWA manifest |
| GET | `/ws` | Realtime `{type: device.status, device}` events |

## Devices

| Method | Path | Notes |
|--------|------|-------|
| GET | `/devices` | Filters `?status=&search=&tag=`, sort `?sort=name\|status\|last_seen&order=`, pagination, `ETag` |
| POST | `/devices` | Create (`notes` max 500, `tags` max 10) |
| DELETE | `/devices?ids=a,b` | Bulk delete, cap 50, atomic |
| GET | `/devices/counts` | Totals per status |
| GET | `/devices/export` | Registry JSON download |
| POST | `/devices/import` | Validate-all atomic import, cap 200 |
| GET | `/devices/:id` | Device detail (incl. counters) |
| PUT | `/devices/:id` | Full replacement |
| PATCH | `/devices/:id` | Partial update |
| DELETE | `/devices/:id` | Remove (`204`) |
| POST | `/devices/:id/wake` | Wake one device (`?retries=&interval=&note=&broadcast_ip=&dry_run=`) |
| POST | `/devices/:id/clone?new_id=` | Copy (counters/status reset) |
| GET | `/devices/:id/uptime` | Reachability since boot |
| POST | `/devices/wake?tag=x` | Wake-by-tag (alias of wake-all selection) |
| POST | `/devices/wake-batch` | Wake by body `{ids}` (+ `tag`/`status`/`dry_run`) |

## Wake

| Method | Path | Notes |
|--------|------|-------|
| POST / GET | `/wake` | Wake `DEFAULT_MAC` (`GET` deprecated) |
| POST / GET | `/wake/:macAddress` | Wake one MAC (`GET` deprecated) |
| POST | `/wake/all` | Wake-all with `tag`/`status` selection |
| POST | `/wake/at` | Schedule a delayed one-shot wake |
| POST | `/wake/oneshots` | Alias of `/wake/at` |
| GET | `/wake/oneshots` | List pending one-shots |
| DELETE | `/wake/oneshots/:id` | Cancel one-shot |

Retry: `?retries=0..5&interval=1..5s` re-pings the device IP until it
answers `up`. `WAKE_COOLDOWN_SEC` bounds manual wakes per device.

## History

| Method | Path | Notes |
|--------|------|-------|
| GET | `/history` | Filters `device_id`/`trigger`/`result`/`success`/`since`/`until`, `?order=`, `?limit=` |
| GET | `/history/stats` | Totals + `top_devices` |
| GET | `/history/export` | CSV download |
| DELETE | `/history?before=` | Purge entries older than unix time |

## Schedules

| Method | Path | Notes |
|--------|------|-------|
| GET / POST | `/schedules` | List (with computed `next_run`) / create (`?dry_run=1`) |
| POST | `/schedules/validate` | Check a cron expression (`{valid, next_run}`) |
| POST | `/schedules/pause-all`, `/schedules/resume-all` | Flip all schedules |
| POST | `/schedules/enable`, `/schedules/disable?ids=` | Bulk flip |
| GET | `/schedules/:id/history` | Fires of one schedule |
| PUT / DELETE | `/schedules/:id` | Replace / remove |
| POST | `/schedules/:id/duplicate` | Copy under a new id |
| POST | `/schedules/:id/fire` | Fire now |

Cron is 5-field standard or descriptors (`@daily`). `once_at` (RFC3339)
creates a one-shot schedule that disables itself after firing.

## Discovery

| Method | Path | Notes |
|--------|------|-------|
| POST | `/discover?cidr=` | Scan (throttled, global cooldown) |
| GET | `/discover/status` | Last scan state |
| POST | `/discover/adopt` | Adopt hosts (`?tags=`, `?dry_run=`) |
| POST | `/discover/adopt-all` | Adopt all unknown, transactional |
| GET / POST / DELETE | `/discover/ignored` | Denylist CRUD (alias `/discover/ignore`) |

Adopt is all-or-nothing: one duplicate MAC fails the whole batch.

## Monitor

| Method | Path | Notes |
|--------|------|-------|
| POST | `/monitor/check?device_id=` | Probe now (all or one) |
| GET | `/monitor/transitions` | Last 20 status changes |
| GET | `/monitor/uptime` | Monitor-level view |
| GET | `/monitor/flapping` | Flapping devices |
| GET | `/monitor/status` | Monitor state |

## Backup

| Method | Path | Notes |
|--------|------|-------|
| GET | `/backup` | Single JSON download |
| POST | `/restore` | Validate-all atomic restore (`dry_run`) |
