# rails-kit

A CLI tool for exploring Rails codebases — schema, routes, models, fixtures, locales, and related files.

## Commands

All commands run as mise tasks, defined in `mise.toml`. The pinned Go and golangci-lint versions are on PATH automatically, so no `mise exec --` prefix is needed:

```
mise run build          # build the binary
mise run test           # run tests
mise run test-verbose   # run tests with verbose output
mise run test-coverage  # run tests with coverage report
mise run bench          # benchmarks + the startup budget check
mise run lint           # run golangci-lint
mise run fmt            # format code
mise run tidy           # go mod tidy
mise run clean          # remove built binary
```

`mise tasks` lists them all. The bare shorthand (`mise test`) works for every task except `fmt` and `install`, which collide with mise's own subcommands — always spell those two as `mise run fmt` / `mise run install`.

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
