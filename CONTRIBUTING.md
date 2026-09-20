# Contributing to rails-kit

## Prerequisites

- [mise](https://mise.jdx.dev/) — manages Go and tool versions, and defines the build tasks

## Setup

```sh
mise install
```

This installs the pinned Go and golangci-lint versions from `mise.toml`.

## Development Commands

```sh
mise run build          # build ./rails-kit binary
mise run test           # run all tests
mise run test-verbose   # run tests with verbose output
mise run test-coverage  # run tests with coverage report
mise run bench          # benchmarks + the startup budget check
mise run lint           # run golangci-lint
mise run fmt            # format code
mise run tidy           # go mod tidy
mise run install        # build and install to $GOBIN
```

These are mise tasks defined in `mise.toml`; `mise tasks` lists them all. The pinned Go and golangci-lint versions are on PATH inside a task, so no `mise exec --` prefix is needed. The bare shorthand (`mise test`) works for every task except `fmt` and `install`, which collide with mise's own subcommands.

The enabled linter set is pinned in `.golangci.yml`, so `mise run lint` and CI check the same thing.

## Testing

- Tests live alongside source in each package (`*_test.go`)
- Integration tests are in `cmd/`, unit tests in `internal/`
- Use `t.TempDir()` for filesystem isolation
- Shared test helpers live in `cmd/config_paths_test.go`
- The `testdata/` directory contains a sample Rails project used by tests

## Pull Requests

- Run `mise run test`, `MISE_OFFLINE=1 mise exec -- go test -race ./...`, and `mise run lint` before submitting
- CI also runs `govulncheck ./...` and blocks reachable known vulnerabilities
- CI also runs the startup budget check (`TestStartupBudget`) as a blocking job — see `mise run bench` to run it locally
- CI runs the test suite on Linux, macOS, and Windows; race detector, benchmarks, and lint run on Linux only
- Keep PRs focused — one concern per PR
- Add tests for new behavior
