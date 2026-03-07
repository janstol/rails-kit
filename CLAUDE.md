# rails-kit

A CLI tool for exploring Rails codebases — schema, routes, models, fixtures, locales, and related files.

## Commands

All commands run via `mise exec -- just <target>` (Go version is pinned via mise):

```
just build          # build the binary
just test           # run tests
just test-verbose   # run tests with verbose output
just test-coverage  # run tests with coverage report
just lint           # run golangci-lint
just fmt            # format code
just tidy           # go mod tidy
just clean          # remove built binary
```

## Project structure

- `cmd/` — Cobra commands (CLI entry points)
- `internal/` — business logic (test directly for unit tests)
- `testdata/` — test fixtures (sample Rails projects)
- `main.go` — entry point

## Test conventions

- Use `t.TempDir()` for filesystem isolation
- Shared test helpers live in `cmd/config_paths_test.go`
- Unit test `internal/` packages directly; integration tests go in `cmd/`

## Dependencies

- No `vendor/` — uses Go modules (`go.mod` / `go.sum`)
- Go version pinned in `mise.toml`
