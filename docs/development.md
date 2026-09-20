# Development

## Build and test

The development environment uses [mise](https://mise.jdx.dev/), which pins the Go and golangci-lint versions and defines the build tasks. `mise install` is the only setup step.

```sh
mise install            # install the pinned toolchain
mise run build
mise run install
mise run test
mise run test-coverage
mise run bench
mise run lint
```

`mise tasks` lists every task. Tasks run with the pinned toolchain on PATH, so no `mise exec --` prefix is needed. The bare shorthand (`mise test`) works for every task except `fmt` and `install`, which collide with mise's own subcommands — spell those `mise run fmt` / `mise run install`.

CI runs the same startup budget check (`TestStartupBudget`) on every push and PR, in its own
`startup-budget` job. Since 2026-09-13 it is a blocking job — it fails the run if a budget is
exceeded, and logs the median and delta-over-floor for every case either way. `mise run bench`
locally is where you would act on a regression it flags.

## Golden files

`cmd/golden_test.go` pins the whole stdout (and, when non-empty, stderr) of `schema`,
`model`, `routes --static`, `concerns`, `locales`, and `skeleton` against files under
`cmd/testdata/golden/`. They exist to catch output-shape regressions — reordering, dropped
fields, reworded text — that fragment-based `strings.Contains` assertions would miss.

Run `go test ./cmd/ -run TestGolden -update` to regenerate them. A diff means the output
shape changed; review it like any other behavioral change before committing, rather than
reflexively re-running `-update` to make the test pass.

## Releases

Tagged releases publish prebuilt archives for:

- macOS `amd64` and `arm64`
- Linux `amd64` and `arm64`
- Windows `amd64`

Project history is maintained in the [changelog](../CHANGELOG.md). Release notes should call out static-parsing limitations and Rails runtime requirements where relevant.

## Behavioral differences

rails-kit originated as a replacement for several Ruby inspection scripts. Relevant differences include:

| Area | Behavior |
|------|----------|
| Key ordering | Locale and fixture keys are sorted alphabetically |
| ERB in fixtures | Scalar ERB becomes `__ERB__`; structural ERB is rejected; Rails metadata is hidden |
| Rails root detection | rails-kit walks up from the current working directory |
| Pluralization | The Go implementation includes suffix rules in addition to irregular mappings |
