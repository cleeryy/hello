# Changelog

## v1.2.0 — 2026-10-27

- Devices power: `PATCH` partial update, filter/sort/pagination on
  `GET /devices`, atomic bulk delete (cap 50), clone, `notes`/`tags`
  fields, persisted `wake_count`/`last_wake_at`, wake-by-tag, JSON
  import/export, `GET /devices/counts`, `seed --force`
- Wake/history/schedules: `dry_run` previews, wake-all, delayed one-shots,
  per-device cooldown, wake notes, broadcast override, history
  trigger/result/since/until/order filters, stats with `top_devices`, CSV
  export, purge, `HISTORY_CAP`, pause-all/resume-all, bulk
  enable/disable, duplicate, fire-now, per-schedule history, one-shot
  schedules, `SCHEDULES_CAP`, create `dry_run`
- Discovery/monitor/health: custom-CIDR scans, transactional adopt-all,
  ignore-list CRUD (`/discover/ignored`), auto-scan, scan status,
  persisted `scan.json`, monitor check/transitions/uptime/flapping/status,
  detailed `/health`, `GET /version`
- Platform/ops: `GET /backup` + atomic `POST /restore` (`dry_run`),
  status/wake webhooks (async, 5s timeout), `X-Request-ID`, `ETag`/`304`,
  `X-RateLimit-*`, `API_TOKEN_FILE`, redacted `GET /config`, `SIGHUP`
  reload, `hello doctor [--json]`, `--version`, full `.env.example`
- Dashboard UX: sorting, bulk wake-down, export/import, collapsible
  sections, auto-refresh, system/light/dark theme, history filters,
  device→history links, schedule next/last display, tag chips, version +
  health footer, empty states, `/` and `r` shortcuts, PWA manifest
- Reliability: wake retry-until-up (`?retries=&interval=`), Prometheus
  `GET /metrics` (`hello_wake_total`, `hello_devices`, `hello_schedules`)
- Docs: README 102-feature matrix + ops section, `wiki/` (API,
  Operations, Dashboard), OpenAPI `1.2.0` covering all routes
- New env vars: `API_TOKEN_FILE`, `HISTORY_CAP`, `SCHEDULES_CAP`,
  `WAKE_COOLDOWN_SEC`, `IGNORED_FILE`, `SCAN_FILE`,
  `DISCOVER_INTERVAL_SEC`, `DISCOVER_COOLDOWN_SEC`,
  `DISCOVER_CONCURRENCY`, `DISCOVER_PORTS`, `DISCOVER_RESOLVE`,
  `PING_TIMEOUT_SEC`, `PING_TCP_PORTS`, `STATUS_WEBHOOK_URL`,
  `WAKE_WEBHOOK_URL`
- Release tag `v1.2.0` (`dev` → `master` via `chore(release): v1.2.0`)

## v1.1.0 — exposed hardening + dashboard

- `API_TOKEN` mandatory (min 16, `change-me` rejected), no open mode
- Gin `TRUSTED_PROXIES` (default loopback) + `ReadHeaderTimeout`
- CORS closed by default via `CORS_ORIGINS`, WS `CheckOrigin` strict (same-origin + whitelist)
- `?token=` WS deprecated, prefer `Authorization` header
- Throttle `wake` 10/min/IP and `adopt` 5/min/IP with `429` + `Retry-After`; discovery 30s cooldown kept
- Adopt all-or-nothing with rollback
- JSON files `0600`, Compose mounts all three, missing-file safe defaults
- `go mod tidy` (cron direct), OpenAPI `1.1.0` (`Host.mac` empty allowed, WS `device.status`)
- Docker Alpine `3.21` + `ca-certificates`, `.dockerignore` tightened, Trivy scans build digest
- `test-api.sh` requires `API_TOKEN`, README + `.env.example` + Caddy/generic reverse-proxy docs
- Minimal SSR `/dashboard` (Rams, no CDN, Bearer in-page, public page + protected API)
