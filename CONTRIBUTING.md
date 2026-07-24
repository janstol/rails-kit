# Contributing to rails-kit

## Prerequisites

- [mise](https://mise.jdx.dev/) — manages Go and tool versions
- [Just](https://github.com/casey/just) — task runner

## Setup

```sh
mise install
```

This installs the pinned Go and golangci-lint versions from `mise.toml`.

## Development Commands

```sh
mise exec -- just build          # build ./rails-kit binary
mise exec -- just test           # run all tests
mise exec -- just test-verbose   # run tests with verbose output
mise exec -- just test-coverage  # run tests with coverage report
mise exec -- just lint           # run golangci-lint
mise exec -- just fmt            # format code
mise exec -- just tidy           # go mod tidy
```

## Testing

- Tests live alongside source in each package (`*_test.go`)
- Integration tests are in `cmd/`, unit tests in `internal/`
- Use `t.TempDir()` for filesystem isolation
- Shared test helpers live in `cmd/config_paths_test.go`
- The `testdata/` directory contains a sample Rails project used by tests

## Pull Requests

- Run `mise exec -- just test`, `MISE_OFFLINE=1 mise exec -- go test -race ./...`, and `mise exec -- just lint` before submitting
- CI also runs `govulncheck ./...` and blocks reachable known vulnerabilities
- Keep PRs focused — one concern per PR
- Add tests for new behavior
