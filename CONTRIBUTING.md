# Contributing to hello

Thanks for helping out. Small, focused PRs beat big ones — one concern
per PR, green CI before review.

## Setup

Requirements: Go 1.26+, `jq` for the manual walkthrough.

```bash
export DEFAULT_MAC=00:11:22:33:44:55
export API_TOKEN=$(openssl rand -hex 16)  # min 16 chars, never empty
go run ./cmd/api
```

## Verify before pushing

```bash
go test -race -shuffle=on -count=1 ./...
go vet ./...
go build ./...
gofmt -l .                                              # must print nothing
go run mvdan.cc/gofumpt@v0.5.0 -l .                     # must print nothing
go mod tidy -diff                                       # must print nothing
API_TOKEN=$API_TOKEN ./test-api.sh                      # end-to-end walkthrough
```

CI runs the same on every PR (`test-and-build`, `test-and-cover`,
`golangci-lint`, `security`, CodeQL) plus title and coverage checks.

## Tests

Read `CONTEXT.md` for the domain language first.

- HTTP behavior lives in `internal/handlers/*_test.go`, Given/When/Then
  style, through public routes — never against internals.
- Pure logic gets unit tests (`wol.BroadcastForIP`, storage, scheduler).
- Expected values are literals from the spec, never recomputed by the
  code under test. One behavior per test (vertical slices, red → green).

## Pull requests

- Branch from `dev` (`feat/…`, `fix/…`, `docs/…`), open the PR **against
  `dev`** — never directly against `master`.
- **Every PR links an issue** (`Closes #NNN` in the body; create the
  issue first if missing). CI fails the PR otherwise.
- **Title must follow Conventional Commits** — enforced by CI
  (`feat:`, `fix:`, `chore:`, `docs:` …). Release PRs use
  `chore(release): vX.Y.Z`.
- Add labels so release notes land in the right bucket: `feature` /
  `enhancement`, `bug` / `fix` / `bugfix`, `chore`; version with
  `major` / `minor` / `patch` (default `patch`).
- Keep `README.md`, `CONTEXT.md`, `CHANGELOG.md` and the wiki in sync
  when behavior changes. Dashboard copy stays terse and functional.

## Security

Do not regress the invariants listed in `AGENTS.md` (mandatory
`API_TOKEN`, validated proxies/origins, closed-by-default WebSocket
origin, throttled wake/adopt, `0600` atomic fail-fast stores,
CSP/HSTS/body-limit). Report sensitive issues with the `security`
label and minimal public detail until fixed.

## Releases

Maintainers merge to `master` → full CI (including the Docker build) →
`release-drafter` updates the draft → publish the `vX.Y.Z` tag, which
rebuilds and scans the release image.
