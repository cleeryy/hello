# AGENTS.md — hello (Wake-on-LAN)

Backend Go (Gin) : registre de devices, magic packets (direct/cron), suivi
reachability, discovery LAN + adopt, WebSocket temps réel, OpenAPI + dashboard SSR.

## Domain language

Read `CONTEXT.md` first. Canonical terms: device, MAC, magic packet,
broadcast IP, reachability (`up`/`down`/`unknown`), last_seen, wake event,
trigger (`manual`/`schedule`), schedule/cron, discovery scan, adopt,
status change. No DB, no multi-user in v1.1 (file persistence, single
shared Bearer, TLS at the reverse proxy).

## Commands

- `go test -race -shuffle=on -count=1 ./...` — full suite, must be green
- `go vet ./...` and `go build ./...` — must pass
- `gofmt -l .` empty; `go run mvdan.cc/gofumpt@v0.5.0 -l .` empty; `go mod tidy -diff` clean
- `API_TOKEN=<16+ chars> DEFAULT_MAC=00:11:22:33:44:55 go run ./cmd/api`
- `API_TOKEN=$API_TOKEN ./test-api.sh` — manual walkthrough (needs `jq`)
- `go run ./cmd/api doctor [--json]` — preflight before restarting
- `go run ./cmd/api seed [--force]` — import `devices.example.json`

## API contract conventions

- Errors are RFC 9457 `application/problem+json`; validation fails with
  `422`, `DELETE` returns `204`.
- New endpoints are additive: keep legacy aliases (`GET /wake*`,
  `/discover/ignore` vs `/discover/ignored`, `/wake/at` vs
  `/wake/oneshots`) and document both.
- Every new route gets an OpenAPI entry (`internal/spec/openapi.yaml`),
  a `test-api.sh` step, and a README/wiki/CHANGELOG touch-up.
- Caps: history ring bounded by `HISTORY_CAP`, schedules by
  `SCHEDULES_CAP`; bulk endpoints cap ids (delete 50, import 200).
- New env vars: `API_TOKEN_FILE` (token from file when `API_TOKEN`
  empty), `HISTORY_CAP`, `SCHEDULES_CAP`, `WAKE_COOLDOWN_SEC`,
  `IGNORED_FILE`, `SCAN_FILE`, `DISCOVER_INTERVAL_SEC`,
  `DISCOVER_COOLDOWN_SEC`, `DISCOVER_CONCURRENCY`, `DISCOVER_PORTS`,
  `DISCOVER_RESOLVE`, `PING_TIMEOUT_SEC`, `PING_TCP_PORTS`,
  `STATUS_WEBHOOK_URL`, `WAKE_WEBHOOK_URL`. Document each in
  `.env.example`, README config table and `wiki/Operations.md`.

## Conventions

- Flow: feature branches → PR against `dev` (never `master` directly);
  releases go `dev` → `master` via PR titled `chore(release): vX.Y.Z`,
  then tag `vX.Y.Z` (rebuilds + scans the release image with
  `stable`/`latest` tags). Branches `dev` and `master` are protected
  (required checks, no force-push/delete, admins included).
- Every PR links an issue (`Closes #NNN`, created first if missing) —
  enforced by the `issue-link` CI check. Conventional Commits enforced
  on PR titles (`semantic-pr` workflow):
  `feat:`, `fix:`, `chore:`, `docs:` … Release notes come from PR labels
  (`release-drafter`): `feature`/`enhancement`, `bug`/`fix`/`bugfix`, `chore`,
  version with `major`/`minor`/`patch` (default `patch`).
- Tests live at HTTP seams (`internal/handlers/*_test.go`) in Given/When/Then
  style, plus unit tests for pure logic (`wol.BroadcastForIP`, storage,
  scheduler). Never assert internals; expected values are literals from the spec.
- Security invariants (do not regress): `API_TOKEN` mandatory ≥16 chars,
  `TRUSTED_PROXIES`/`CORS_ORIGINS` validated, WS origin closed-by-default,
  `?token=` deprecated, wake/adopt throttled with `429` + `Retry-After`,
  JSON stores `0600` + atomic + fail-fast, CSP/HSTS/body-limit on.
- Never commit runtime state: `.env`, `devices.json` stays default `[]`,
  `wake-history.json`/`schedules.json`, `scan.json`/`ignored.json` and `.omo/` are git-ignored.
