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
  `wake-history.json`/`schedules.json` and `.omo/` are git-ignored.
