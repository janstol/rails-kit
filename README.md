# rails-kit

A fast, compiled CLI for inspecting Rails projects. Most commands read project files directly without loading Rails, making them useful for developers, scripts, editors, and AI agents.

## Installation

```sh
brew install --cask janstol/tap/rails-kit
```

Homebrew is macOS/Linux only. On Windows, download the `windows_amd64.zip` asset from the
[releases page](https://github.com/janstol/rails-kit/releases) (works well with Rails-on-WSL2
setups too).

Or install with Go:

```sh
go install github.com/janstol/rails-kit@latest
```

## Quick start

Run `rails-kit` anywhere inside a Rails project. It finds the Rails root automatically; use `--root` to override it.

```sh
rails-kit about
rails-kit schema users
rails-kit routes users
rails-kit related user
rails-kit model user
rails-kit skeleton app/services
```

Most inspection commands accept `--json` for structured output:

```sh
rails-kit model user --json
rails-kit --root /path/to/app schema users --json
```

`schema` and `model` accept `--color=auto|always|never` (default `auto`) for terminal accents. `auto` disables color when stdout isn't a terminal; `NO_COLOR` (any non-empty value) disables color even under `--color=always`; `--json` output is never colored.

## Commands

| Command | Description |
|---------|-------------|
| `rails-kit about` | Summarize application, dependency, runtime, and database metadata |
| `rails-kit schema [table...]` | Inspect `db/schema.rb` or `db/structure.sql` |
| `rails-kit routes [pattern...]` | Show cached Rails routes or use offline static parsing |
| `rails-kit related <name>` | Find files related to a model |
| `rails-kit fixtures [name]` | Inspect test fixtures |
| `rails-kit locales [scope]` | Browse locale keys and values |
| `rails-kit model <name>` | Summarize an Active Record model |
| `rails-kit skeleton <input...>` | Generate compact Ruby AST skeletons through Prism |
| `rails-kit gem [name]` | Inspect `Gemfile.lock` |
| `rails-kit concerns [name]` | Inspect model and controller concerns |
| `rails-kit skill install\|uninstall` | Manage the bundled Claude Code or Codex skill |
| `rails-kit completion bash\|zsh\|fish` | Generate shell completions |
| `rails-kit version` | Print version information |

See the [command reference](docs/commands.md) for examples, JSON shapes, requirements, and limitations.

## Documentation

- [Command reference](docs/commands.md)
- [`--json` output contract](docs/json.md)
- [Static routes parser](docs/static-routes.md)
- [Configuration](docs/configuration.md)
- [Agent skill installation](docs/agent-skill.md)
- [Development and releases](docs/development.md)
- [Changelog](CHANGELOG.md)

## Runtime behavior

Most commands are pure Go and do not boot Rails. The main exceptions are:

- `routes` runs `bundle exec rails routes`; use `routes --static` for a fast offline approximation.
- `about --runtime` boots Rails to obtain active runtime values; plain `about` is static.
- `skeleton` invokes Ruby with Prism but does not load the Rails application.

Static parsing is intentionally conservative. Consult the relevant reference page before using its output as a source of truth in CI or production audits.
