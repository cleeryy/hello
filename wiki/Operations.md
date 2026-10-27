# Operations (v1.2.0)

## Deploy

```bash
cp .env.example .env
cp devices.example.json devices.json
docker compose up --build
```

Image `ghcr.io/cleeryy/hello:latest` (also `stable` on releases).
TLS terminates at the reverse proxy; the app only speaks HTTP.

## Environment

| Variable | Default | Purpose |
|----------|---------|---------|
| `DEFAULT_MAC` | — (required) | MAC woken by `GET /wake` |
| `API_TOKEN` | — (required) | Bearer token: min 16 chars, `[A-Za-z0-9_-]`, never `change-me` |
| `API_TOKEN_FILE` | — | Token file (Docker secrets); used only when `API_TOKEN` is empty |
| `PORT` | `8080` | Listen port |
| `BROADCAST_IP` | `255.255.255.255` | Subnet broadcast for magic packets |
| `DEVICES_FILE` | `devices.json` | Device registry file |
| `HISTORY_FILE` | `wake-history.json` | Wake log file |
| `HISTORY_CAP` | `200` | Wake log retention, 10..2000 |
| `SCHEDULES_FILE` | `schedules.json` | Wake schedules file |
| `SCHEDULES_CAP` | `100` | Max schedules, 1..1000 |
| `IGNORED_FILE` | `ignored.json` | Discovery denylist file |
| `SCAN_FILE` | `scan.json` | Last scan state file |
| `MONITOR_INTERVAL_SEC` | `30` | Status poll interval in seconds |
| `WAKE_COOLDOWN_SEC` | `0` | Per-device wake cooldown, 0 = off, max 3600 |
| `DISCOVER_INTERVAL_SEC` | `0` | Auto-scan cadence, 0 = off, min 60 when set |
| `DISCOVER_COOLDOWN_SEC` | `30` | Scan cooldown, 5..600 |
| `DISCOVER_CONCURRENCY` | `128` | Parallel scan probes, 1..1024 |
| `DISCOVER_PORTS` | `22,80,443,445,8080` | TCP probe ports for discovery |
| `DISCOVER_RESOLVE` | `true` | Reverse-DNS hostnames in scans |
| `PING_TIMEOUT_SEC` | `2` | Per-host reachability timeout, 1..30 |
| `PING_TCP_PORTS` | `22,80` | TCP fallback ports for reachability |
| `STATUS_WEBHOOK_URL` | — | Reachability-change webhook (empty = off) |
| `WAKE_WEBHOOK_URL` | — | Wake-report webhook (empty = off) |
| `TRUSTED_PROXIES` | `127.0.0.1,::1` | Proxy IPs/CIDRs trusted for `X-Forwarded-For` |
| `CORS_ORIGINS` | — | Explicit browser origins, comma-separated |

Precedence: real environment always wins over `.env`. The server refuses
to boot on missing/invalid `DEFAULT_MAC`, `API_TOKEN`, `TRUSTED_PROXIES`,
`CORS_ORIGINS` or webhook URLs — run `hello doctor` before restarting.

## Volumes and files

All JSON stores are written `0600` and atomically (temp file + rename).
Mount `devices.json`, `wake-history.json`, `schedules.json`,
`ignored.json` and `scan.json` in Compose. Never commit runtime state:
`.env`, `devices.json` content, `wake-history.json`, `schedules.json`,
`scan.json`, `ignored.json`, `.omo/`.

## Backup / restore runbook

1. Download: `GET /backup` (bearer token) → one JSON document with
   devices, schedules, history and ignore list. Store it off-box.
2. Trial: `POST /restore` with `{"dry_run": true, ...}` (or
   `?dry_run=1`) → `{dry_run: true, valid: true, counts}` without
   touching the registry.
3. Apply: `POST /restore` with the backup body. Everything is validated
   first (devices, schedules against `SCHEDULES_CAP`, history against
   `HISTORY_CAP`, schedule→device references); any failure aborts with
   `422` and the registry is left untouched.
4. Verify: `GET /devices/counts` and `GET /health`.

## Webhooks

`STATUS_WEBHOOK_URL` gets `{device_id, status, at}` on reachability
changes; `WAKE_WEBHOOK_URL` gets wake reports. Delivery is async in a
goroutine (never blocks API responses), 5s per-attempt timeout, max 8
in-flight deliveries (overflow is dropped with a log), failures only
logged. Both are SSRF surfaces: use operator-controlled `http(s)` URLs
only. `GET /config` shows webhook presence flags, never URLs or tokens.

## Metrics

`GET /metrics` returns Prometheus text (`hello_wake_total{trigger,result}`,
`hello_devices{status}`, `hello_schedules{...}`). It requires the bearer
token — configure Prometheus with an `Authorization: Bearer` header
(`bearer_token` / `bearer_token_file` in the scrape config).

## Doctor, version, reload

- `hello doctor` — preflight checks (required vars, token shape, files,
  proxy/CORS/webhook URL validity). Exits non-zero on failure.
- `hello doctor --json` — same report as JSON for scripting.
- `hello version` (or `--version`/`-v`) — build version (ldflags on
  releases, `dev` otherwise). Also served at `GET /version` and `/health`.
- `kill -HUP <pid>` — re-reads the registry file; failures keep the
  current registry and log a warning.
- `hello seed [--force]` — imports `devices.example.json`; without
  `--force` existing ids are kept, with `--force` the file is rewritten.

## Troubleshooting

- **Rootless ping**: ICMP needs privileges; without them the monitor
  falls back to TCP probes (`PING_TCP_PORTS`). Tune with
  `PING_TIMEOUT_SEC`.
- **Wakes never arrive from Docker**: magic packets use subnet broadcast,
  which does not cross the Docker bridge — use `network_mode: host`
  (Linux) or run the binary on the LAN host.
- **Discovery misses hosts**: check `DISCOVER_PORTS`, `DISCOVER_COOLDOWN_SEC`
  (429 = still cooling down), and the ignore list (`GET /discover/ignored`);
  ignored MACs/IPs never appear in scan results.
- **429 on wake/adopt**: per-IP throttles (wake 10/min, adopt 5/min) or
  `WAKE_COOLDOWN_SEC`; honor `Retry-After` / `X-RateLimit-Reset`.
- **TLS**: terminate at the reverse proxy (Caddy/Nginx/Traefik), trust
  only its IPs via `TRUSTED_PROXIES`, and set `CORS_ORIGINS` for the
  dashboard origin.
