# Development

## Build and test

The development environment uses [mise](https://mise.jdx.dev/). Building from source requires Go and [Just](https://github.com/casey/just).

```sh
mise install
mise exec -- just build
mise exec -- just install
mise exec -- just test
mise exec -- just test-coverage
mise exec -- just lint
```

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
