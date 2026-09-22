# Changelog

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
