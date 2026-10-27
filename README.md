# hello

[![Docker Build](https://github.com/cleeryy/hello/actions/workflows/docker-build.yml/badge.svg)](https://github.com/cleeryy/hello/actions/workflows/docker-build.yml)
[![CI](https://github.com/cleeryy/hello/actions/workflows/ci.yml/badge.svg)](https://github.com/cleeryy/hello/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/cleeryy/hello)](https://github.com/cleeryy/hello/releases)
[![Go Version](https://img.shields.io/badge/Go-1.26-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

A simple, fast, and containerized **wake on lan** built with Go and Gin framework. Send magic packets to wake up your devices over the network with ease.

## Prerequisites

- **Docker** (for containerized deployment)
- **Go 1.26+** (for local development)
- Network access to devices you want to wake

## Quick Start

### Using Docker Compose (Recommended)

```
cp .env.example .env
cp devices.example.json devices.json
docker compose up --build
```

> Magic packets use subnet broadcast, which does not cross Docker's bridge
> network. If wake-ups never arrive, use `network_mode: host` (Linux only).

### Using Docker

```
docker pull ghcr.io/cleeryy/hello:latest

docker run -p 8080:8080 \
  -e DEFAULT_MAC=00:11:22:33:44:55 \
  -e API_TOKEN=replace-with-32-char-random-token \
  ghcr.io/cleeryy/hello:latest
```

### Local Development

```
# Clone the repository
git clone https://github.com/cleeryy/hello.git
cd hello

# Install dependencies
go mod download

# Create .env file
cp .env.example .env

# Run the API
go run ./cmd/api
```

The API will be available at `http://localhost:8080`

## Feature matrix (v1.2.0 — F-01..F-102)

Every row below is implemented and covered by tests. `Where` is the surface:
`route` = HTTP endpoint, `env` = environment variable, `UI` = `/dashboard`,
`CLI` = command or script. All rows are `Done`.

### Devices power (F-01..F-17)

| ID | Feature | Where | Status |
|----|---------|-------|--------|
| F-01 | `PATCH /devices/:id` partial update, same rules as `PUT` | route | Done |
| F-02 | `GET /devices` filters `?status=up&search=name&tag=lan` | route | Done |
| F-03 | `GET /devices` sort `?sort=name\|status\|last_seen&order=asc\|desc` | route | Done |
| F-04 | `GET /devices` pagination `?page=&per_page=` (default 100, cap 500, `total` meta) | route | Done |
| F-05 | Bulk delete `DELETE /devices?ids=id1,id2` (cap 50, atomic) | route | Done |
| F-06 | Clone `POST /devices/:id/clone?new_id=` (counters reset) | route | Done |
| F-07 | `notes` field (max 500 chars, on POST/PUT/PATCH) | route | Done |
| F-08 | `tags` field (`[a-z0-9-]`, max 10, lowercased) | route | Done |
| F-09 | `wake_count` persisted, incremented on every wake | route | Done |
| F-10 | `last_wake_at` persisted on every wake | route | Done |
| F-11 | Wake-by-tag `POST /devices/wake?tag=x` (throttled, `dry_run` preview) | route | Done |
| F-12 | Export `GET /devices/export` (JSON download) | route | Done |
| F-13 | Import `POST /devices/import` (validate-all atomic, cap 200) | route | Done |
| F-14 | Dashboard sort + tag chips + tag filter | UI | Done |
| F-15 | Dashboard export + bulk wake down + detail notes/tags/counters | UI | Done |
| F-16 | `GET /devices/counts` (totals per status) | route | Done |
| F-17 | CLI seed imports `devices.example.json` (`--force` when non-empty) | CLI | Done |

### Wake, history, schedules (F-18..F-44)

| ID | Feature | Where | Status |
|----|---------|-------|--------|
| F-18 | Wake `dry_run` (`?dry_run=1` previews without sending) | route | Done |
| F-19 | Wake-all `POST /wake/all` (`status`/`tag` filters, throttled) | route | Done |
| F-20 | One-shot wake `POST /wake/at` (alias `POST /wake/oneshots`) + `GET /wake/oneshots` | route | Done |
| F-21 | Cancel one-shot `DELETE /wake/oneshots/:id` | route | Done |
| F-22 | Cooldown `WAKE_COOLDOWN_SEC` per device (`429` + `Retry-After`) | env | Done |
| F-23 | Wake note (`note` body/query, max 140, stored in history) | route | Done |
| F-24 | Broadcast override `?broadcast_ip=` per wake | route | Done |
| F-25 | Global `BROADCAST_IP` config | env | Done |
| F-26 | History filters `trigger`/`result`/`success`/`since`/`until`/`device_id` | route | Done |
| F-27 | History `?order=asc\|desc` | route | Done |
| F-28 | `GET /history/stats` | route | Done |
| F-29 | Purge `DELETE /history?before=` (RFC3339 timestamp) | route | Done |
| F-30 | History CSV `GET /history/export` | route | Done |
| F-31 | `HISTORY_CAP` retention bound | env | Done |
| F-32 | `attempts`/`note` exposed on history entries | route | Done |
| F-33 | `top_devices` in history stats | route | Done |
| F-34 | `POST /schedules/pause-all` and `/schedules/resume-all` | route | Done |
| F-35 | `next_run` computed in `GET /schedules` | route | Done |
| F-36 | `last_run_at` + `last_result` persisted per schedule | route | Done |
| F-37 | One-shot schedules (`once_at`) auto-disable after firing | route | Done |
| F-38 | Duplicate `POST /schedules/:id/duplicate` | route | Done |
| F-39 | `GET /schedules/:id/history` (fires of one schedule) | route | Done |
| F-40 | Schedules API exposes next/last fire fields | route | Done |
| F-41 | Bulk enable/disable `POST /schedules/enable\|/disable?ids=` | route | Done |
| F-42 | `SCHEDULES_CAP` bound (default 100) | env | Done |
| F-43 | Device detail links to its next scheduled fire | route + UI | Done |
| F-44 | Schedule create `dry_run` (`?dry_run=1` validates only) | route | Done |

### Discovery, monitor, health (F-45..F-65)

| ID | Feature | Where | Status |
|----|---------|-------|--------|
| F-45 | Scan custom CIDR `POST /discover?cidr=` | route | Done |
| F-46 | Adopt-all `POST /discover/adopt-all` (transactional) | route | Done |
| F-47 | Ignore-list CRUD `GET/POST/DELETE /discover/ignored` (alias `/discover/ignore`) | route | Done |
| F-48 | Ignored hosts excluded from scan results | route | Done |
| F-49 | Auto-scan `DISCOVER_INTERVAL_SEC` (`0` = off) | env | Done |
| F-50 | `GET /discover/status` (last scan state) | route | Done |
| F-51 | Adopt `dry_run` (preview without creating) | route | Done |
| F-52 | Adopt with `tags` | route | Done |
| F-53 | Dashboard adopt-all + CIDR input + scan status | UI | Done |
| F-54 | Scan state persisted (`SCAN_FILE`, default `scan.json`) | env | Done |
| F-55 | `POST /monitor/check` (all or `?device_id=`) | route | Done |
| F-56 | `GET /monitor/transitions` (last 20 changes) | route | Done |
| F-57 | `GET /devices/:id/uptime` (since boot) | route | Done |
| F-58 | `/health` details (version, uptime, device/schedule counts, history size) | route | Done |
| F-59 | `GET /version` | route | Done |
| F-60 | Dashboard footer version + health dot | UI | Done |
| F-61 | Flap detection + `GET /monitor/flapping` | route | Done |
| F-62 | `PING_TIMEOUT_SEC` reachability timeout | env | Done |
| F-63 | `PING_TCP_PORTS` TCP fallback ports | env | Done |
| F-64 | `GET /monitor/status` | route | Done |
| F-65 | `ping_enabled=false` respected everywhere + dashboard badge | route + UI | Done |

### Platform and ops (F-66..F-81)

| ID | Feature | Where | Status |
|----|---------|-------|--------|
| F-66 | `GET /backup` (single JSON download) | route | Done |
| F-67 | `POST /restore` (validate-all atomic + `dry_run`) | route | Done |
| F-68 | `STATUS_WEBHOOK_URL` (reachability changes) | env | Done |
| F-69 | `WAKE_WEBHOOK_URL` (wake reports) | env | Done |
| F-70 | Webhooks async, 5s timeout, failures logged only | env | Done |
| F-71 | `X-Request-ID` on every response | route | Done |
| F-72 | `API_TOKEN_FILE` (Docker secrets) | env | Done |
| F-73 | `GET /config` (redacted operational settings) | route | Done |
| F-74 | `SIGHUP` reloads the device registry | CLI | Done |
| F-75 | `hello doctor` preflight CLI | CLI | Done |
| F-76 | `hello doctor --json` machine-readable report | CLI | Done |
| F-77 | `--version` flag + ldflags build version | CLI | Done |
| F-78 | `ETag` on `GET /devices` (`304` on `If-None-Match`) | route | Done |
| F-79 | `X-RateLimit-*` headers on wake/adopt throttles | route | Done |
| F-80 | `test-api.sh` covers the new endpoints | CLI | Done |
| F-81 | `.env.example` documents every variable | env | Done |

### Dashboard UX (F-82..F-95)

| ID | Feature | Where | Status |
|----|---------|-------|--------|
| F-82 | Device table sorting | UI | Done |
| F-83 | Bulk "wake all down" button | UI | Done |
| F-84 | Export button (downloads registry JSON) | UI | Done |
| F-85 | Import dialog + result report | UI | Done |
| F-86 | Collapsible sections persisted in `localStorage` | UI | Done |
| F-87 | Auto-refresh toggle (off/10s/30s/60s) | UI | Done |
| F-88 | Theme system/light/dark (persisted, no flash) | UI | Done |
| F-89 | History filters in-page | UI | Done |
| F-90 | Per-device link to filtered history | UI | Done |
| F-91 | Schedules show `next_run`/`last_result` | UI | Done |
| F-92 | Tag chips + tag filter | UI | Done |
| F-93 | Footer version + health indicator | UI | Done |
| F-94 | Empty states with calls to action | UI | Done |
| F-95 | Shortcuts `/` (search) and `r` (refresh) | UI | Done |

### Docs and release (F-96..F-102)

| ID | Feature | Where | Status |
|----|---------|-------|--------|
| F-96 | README 102-feature matrix (this table) | docs | Done |
| F-97 | README ops section (webhooks/backup/metrics) | docs | Done |
| F-98 | Wiki API + Operations pages | docs | Done |
| F-99 | Wiki Dashboard page | docs | Done |
| F-100 | CHANGELOG `v1.2.0` entry | docs | Done |
| F-101 | `AGENTS.md` + `CONTRIBUTING.md` updated | docs | Done |
| F-102 | OpenAPI `1.2.0` covering all routes | docs | Done |

Endpoint details live in `wiki/API.md` and the machine-readable
`GET /openapi.yaml` (Swagger UI at `GET /docs`).

## Operations

### Auth

`API_TOKEN` is required (min 16 chars, `[A-Za-z0-9_-]` only, `change-me`
rejected, no surrounding whitespace). `API_TOKEN_FILE` (e.g.
`/run/secrets/api_token` for Docker secrets) supplies the token from a file
and is used **only when `API_TOKEN` is empty** — a set `API_TOKEN` always
wins. Every route except `GET /`, `/health`, `/docs`, `/openapi.yaml`,
`/dashboard` and `/manifest.json` requires
`Authorization: Bearer <token>`. `?token=` on `/ws` still works for compat
but leaks in proxy logs, so prefer the header.

### Proxy and CORS

`TRUSTED_PROXIES` (default loopback) allowlists the reverse proxies whose
`X-Forwarded-For` Gin trusts. `CORS_ORIGINS` is an explicit, comma-separated
browser-origin whitelist (same-origin only when empty; `*` rejected);
the WebSocket origin check uses the same list.

### Throttles and cooldowns

- Wake and discovery scans: one request per 30s per client (IP + token hash),
  with `429` + `Retry-After` and `X-RateLimit-Limit/Remaining/Reset` headers.
- Adopt: max 5/min/IP, same headers.
- `WAKE_COOLDOWN_SEC` (0 = off, max 3600): per-device manual-wake cooldown,
  answered with `429` + `Retry-After`.
- Discovery scans have a global cooldown (`DISCOVER_COOLDOWN_SEC`,
  default 30s, floor 5s).

### Webhooks

`STATUS_WEBHOOK_URL` receives reachability changes, `WAKE_WEBHOOK_URL`
receives wake reports. Delivery is async (never blocks the response),
each attempt times out after 5s, at most 8 deliveries are in flight
(further events are dropped with a log), and failures are only logged.
Both URLs must be operator-controlled `http(s)` URLs — the server POSTs
JSON to them, so never point them at unreviewed third-party hosts.

### Backup and restore

- `GET /backup` downloads one JSON document: devices, schedules, history,
  ignore list.
- `POST /restore` validates **everything first** (devices, schedules
  against `SCHEDULES_CAP`, history against `HISTORY_CAP`, ignore entries,
  and schedule→device references) and applies atomically — any failure
  leaves the registry untouched. `dry_run` validates only and returns the
  counts that would be restored.
- Caps: history is a ring buffer bounded by `HISTORY_CAP` (default 200);
  schedules are bounded by `SCHEDULES_CAP` (default 100).

### Observability

- `GET /metrics` returns Prometheus text (`hello_wake_total`,
  `hello_devices`, `hello_schedules`, …). It sits behind auth like any
  other guarded route — point Prometheus at it with a bearer token.
- `GET /health` is the public liveness probe and also reports version,
  uptime, device/schedule counts and history size. `GET /version`
  returns the build version.
- `hello doctor` runs preflight checks (config, files, token);
  `hello doctor --json` prints the machine-readable report.
  `go run ./cmd/api --version` (or `version`) prints the build version,
  baked in via ldflags on release builds.
- `SIGHUP` re-reads the registry file without restarting; a failed reload
  keeps the current registry and logs a warning.
- `GET /devices` sends an `ETag` (stable content hash); replays with
  `If-None-Match` get `304 Not Modified`.
- Every response carries `X-Request-ID` (echoed when the client sends one).

### Files

- `SCAN_FILE` (default `scan.json`) persists the last discovery scan across
  restarts; `IGNORED_FILE` (default `ignored.json`) persists the discovery
  denylist. All JSON stores are written `0600` and atomically — mount them
  all in Compose.
- Import/export: `GET /devices/export` downloads the registry,
  `POST /devices/import` validates the whole batch before creating
  anything (cap 200). `GET /history/export` downloads the wake log as CSV.
- `hello seed` imports `devices.example.json` into the registry file;
  `--force` rewrites a non-empty registry.

Full runbooks live in `wiki/Operations.md`; the endpoint reference in
`wiki/API.md`; the dashboard guide in `wiki/Dashboard.md`.

## Configuration

Every variable is documented in `.env.example`. Summary:

| Variable | Default | Purpose |
|----------|---------|---------|
| `DEFAULT_MAC` | — (required) | MAC woken by `GET /wake` |
| `API_TOKEN` | — (required) | Bearer token (min 16 chars) |
| `API_TOKEN_FILE` | — | Token file, used only when `API_TOKEN` is empty |
| `PORT` | `8080` | Listen port |
| `BROADCAST_IP` | `255.255.255.255` | Subnet broadcast for magic packets |
| `DEVICES_FILE` | `devices.json` | Device registry file |
| `HISTORY_FILE` | `wake-history.json` | Wake log file |
| `HISTORY_CAP` | `200` | Wake log retention (10..2000) |
| `SCHEDULES_FILE` | `schedules.json` | Wake schedules file |
| `SCHEDULES_CAP` | `100` | Max schedules (1..1000) |
| `IGNORED_FILE` | `ignored.json` | Discovery denylist file |
| `SCAN_FILE` | `scan.json` | Last scan state file |
| `MONITOR_INTERVAL_SEC` | `30` | Status poll interval in seconds |
| `WAKE_COOLDOWN_SEC` | `0` | Per-device wake cooldown (0 = off, max 3600) |
| `DISCOVER_INTERVAL_SEC` | `0` | Auto-scan cadence (0 = off, min 60 when set) |
| `DISCOVER_COOLDOWN_SEC` | `30` | Scan cooldown (5..600) |
| `DISCOVER_CONCURRENCY` | `128` | Parallel scan probes (1..1024) |
| `DISCOVER_PORTS` | `22,80,443,445,8080` | TCP probe ports for discovery |
| `DISCOVER_RESOLVE` | `true` | Reverse-DNS hostnames in scans |
| `PING_TIMEOUT_SEC` | `2` | Per-host reachability timeout (1..30) |
| `PING_TCP_PORTS` | `22,80` | TCP fallback ports for reachability |
| `STATUS_WEBHOOK_URL` | — | Reachability-change webhook (disabled when empty) |
| `WAKE_WEBHOOK_URL` | — | Wake-report webhook (disabled when empty) |
| `TRUSTED_PROXIES` | `127.0.0.1,::1` | Reverse-proxy IPs/CIDRs for `X-Forwarded-For` |
| `CORS_ORIGINS` | *(same-origin)* | Extra browser origins, comma-separated |

Or create a `.env` file in the project root (see `.env.example`).

## Project Structure

```
hello/
├── cmd/api/             # Binary entrypoint (serve, seed, doctor, version)
├── internal/
│   ├── config/          # Env-based configuration + validation
│   ├── handlers/        # HTTP routes + dashboard SSR + CORS/throttle
│   ├── history/         # Wake log (ring buffer + persistence)
│   ├── scheduler/       # Wake schedules (cron runner + file store)
│   ├── models/          # Device/Schedule/WakeEvent + validation
│   ├── monitor/         # Background ping monitor (+flapping)
│   ├── ping/            # ICMP/TCP ping helpers
│   ├── storage/         # File-backed device registry
│   ├── discover/        # LAN scan + adopt + ignore list
│   ├── notify/          # Webhook senders (async, bounded)
│   ├── websocket/       # Live-status hub (/ws, strict origin)
│   ├── spec/            # Embedded OpenAPI + Swagger UI
│   └── wol/             # Wake-on-LAN packet sender
├── wiki/                # API, Operations and Dashboard guides
├── devices.example.json # Sample registry (copy to devices.json)
├── Dockerfile           # Multi-stage Docker build
├── docker-compose.yml   # Local deployment
├── test-api.sh          # Manual API walkthrough
├── .github/workflows/   # CI, Docker build, security scans
└── README.md            # This file
```

## Built With

- **[Go 1.26](https://golang.org/)** - Programming language
- **[Gin](https://gin-gonic.com/)** - Web framework
- **[gowol](https://github.com/linde12/gowol)** - Wake-on-LAN implementation
- **[godotenv](https://github.com/joho/godotenv)** - .env file loader
- **[gorilla/websocket](https://github.com/gorilla/websocket)** - Realtime status
- **[Docker](https://www.docker.com/)** - Containerization
- **[GitHub Actions](https://github.com/features/actions)** - CI/CD

## Development

### Run tests locally

```
go test -race -shuffle=on -count=1 ./...
```

### Format code

```
gofmt -l .
```

### Lint code

```
golangci-lint run ./...
```

## Security Notes

- `API_TOKEN` is mandatory (min 16 chars); the server refuses to boot without it
- Keep your `.env` file private and never commit it; rotate the token by restart
- `?token=` on `/ws` is deprecated (leaks in proxy logs), prefer the `Authorization` header
- CORS defaults to same-origin; set `CORS_ORIGINS` only for explicit browser origins
- WebSocket origin is locked to same-origin + `CORS_ORIGINS`
- Wake/discover (30s per client) and adopt (`5/min/IP`) are throttled with `429` + `Retry-After` plus `X-RateLimit-*` headers
- JSON files are written `0600` mono-instance; mount all of them in Compose
- Webhook URLs are an SSRF surface: only operator-controlled `http(s)` URLs

## Reverse proxy (Caddy + generic)

TLS terminates at the proxy. Trust only its IPs via `TRUSTED_PROXIES`, forward
`X-Forwarded-For/Proto/Host`, and strip incoming forwarding headers from clients.

```caddy
# Caddyfile (same host)
lan.example.com {
  reverse_proxy 127.0.0.1:8080
}
# env: TRUSTED_PROXIES=127.0.0.1/32,::1/128 CORS_ORIGINS=https://lan.example.com
```

Generic Nginx/Traefik: same env pattern, allowlist `10.0.0.0/8,172.16.0.0/12,192.168.0.0/16`
via `TRUSTED_PROXIES` when the proxy is on LAN.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Author

**Cléry Arque-Ferradou**
- GitHub: [@cleeryy](https://github.com/cleeryy)
- Repository: [cleeryy/hello](https://github.com/cleeryy/hello)

## Acknowledgments

- Inspired by the need for a lightweight, Dockerized Wake-on-LAN service
- Built with love using Go and Gin

---

**Happy waking!**
