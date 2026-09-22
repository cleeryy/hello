## Linked issue (required — create it first if missing)

Closes #

## What

<!-- One concern per PR. What changes, and why, in 2-3 sentences. -->

## How I verified

<!-- Commands run and their result, e.g. -->

- [ ] `go test -race -shuffle=on -count=1 ./...` green
- [ ] `go vet ./...`, `go build ./...` pass
- [ ] `gofmt -l .` and `gofumpt -l .` empty, `go mod tidy -diff` clean
- [ ] Manual check (route, dashboard, or `test-api.sh`): …

## Docs sync

- [ ] `README.md` / `CONTEXT.md` / `CHANGELOG.md` / wiki updated if behavior changed
- [ ] No runtime state committed (`.env`, local JSON files)
