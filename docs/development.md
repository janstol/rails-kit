# Development

## Build and test

The development environment uses [mise](https://mise.jdx.dev/). Building from source requires Go and [Just](https://github.com/casey/just).

```sh
mise install
mise exec -- just build
mise exec -- just install
mise exec -- just test
mise exec -- just test-coverage
mise exec -- just bench
mise exec -- just lint
```

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

Project history is maintained in the [changelog](../CHANGELOG.md). Release notes should call out static-parsing limitations and Rails runtime requirements where relevant.

## Behavioral differences

rails-kit originated as a replacement for several Ruby inspection scripts. Relevant differences include:

| Area | Behavior |
|------|----------|
| Key ordering | Locale and fixture keys are sorted alphabetically |
| ERB in fixtures | Scalar ERB becomes `__ERB__`; structural ERB is rejected; Rails metadata is hidden |
| Rails root detection | rails-kit walks up from the current working directory |
| Pluralization | The Go implementation includes suffix rules in addition to irregular mappings |
