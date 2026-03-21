# rails-kit

A compiled CLI toolkit for Rails projects. Fast, single-binary tools for reading project structure. Most commands work without loading Rails; `routes` shells out to `bundle exec rails routes` and requires a working Rails environment.

## Commands

| Command | Description |
|---------|-------------|
| `rails-kit schema [table...]` | Extract table definitions from `db/schema.rb` |
| `rails-kit routes [pattern...]` | Cached, filtered `rails routes` output |
| `rails-kit related <name>` | List all files related to a model |
| `rails-kit fixtures [name]` | Summarize test fixture entries |
| `rails-kit locales [scope]` | Extract locale keys by scope |
| `rails-kit model <name>` | Compact model structure summary |
| `rails-kit skill install|uninstall` | Install or remove the bundled Claude Code skill |
| `rails-kit completion bash|zsh|fish` | Generate shell completion scripts |
| `rails-kit version` | Print version information |

## Installation

```sh
brew install janstol/tap/rails-kit
```

Or with Go:

```sh
go install github.com/janstol/rails-kit@latest
```

Or build from source:

```sh
mise install
mise exec -- just install
```

## Releases

Tagged releases publish prebuilt archives through GitHub Releases for:

- macOS `amd64` and `arm64`
- Linux `amd64` and `arm64`

Project history is tracked in [CHANGELOG.md](CHANGELOG.md). Until the tool has broader production mileage, release notes should call out parsing limitations and Rails-environment requirements explicitly.

## Usage

Run from anywhere inside a Rails project. `rails-kit` walks up from the current directory to find the Rails root (presence of `config/application.rb`). Override with `--root`:

```sh
rails-kit --root /path/to/rails/app schema users
```

Add `--json` to any command to get machine-readable JSON output suitable for scripts, editor tooling, and AI/agent workflows.

JSON shapes:

- `schema` with no table args returns `[]string`; with table args returns an object keyed by table name
- `routes` returns `[{ prefix, verb, uri_pattern, controller_action }]`
- `related` returns `{ model, plural, categories }`
- `model` returns `{ class_name, parent_class?, rel_path, table_name?, concerns, associations, validations, scopes, callbacks, enums, delegates }`
- `fixtures` with no name returns `[]string`; with a name returns `{ file, entries }`
- `locales` with no scope returns `[]string`; with a scope returns `{ scope, value }`
- `version` returns `{ version, commit, build_date }`

### schema

```sh
rails-kit schema                # list all table names
rails-kit schema users          # show users table definition
rails-kit schema users posts    # show multiple tables
rails-kit schema --json         # list table names as JSON array
rails-kit schema users --json   # extract tables as JSON object keyed by table name
```

### routes

```sh
rails-kit routes                # show all routes (uses cache)
rails-kit routes users          # show routes matching "users"
rails-kit routes users posts    # show routes matching either term
rails-kit routes --refresh      # force cache regeneration
rails-kit routes --no-cache     # skip cache entirely (don't read or write)
rails-kit routes --json         # all routes as JSON array
rails-kit routes users --json   # filtered routes as JSON array
```

Routes are cached in `tmp/routes_cache.txt`. Cache is invalidated when `config/routes.rb` or any file in `config/routes/` changes. `--refresh` and `--no-cache` are mutually exclusive.

`routes --json` parses the standard tabular `rails routes` output, skips leading boot noise before the header, and correctly handles rows with a blank route prefix. If Rails emits non-tabular output instead of the usual routes table, JSON output errors clearly instead of returning an ambiguous empty array.

### related

```sh
rails-kit related user          # find all files for User model
rails-kit related order_item    # works with underscored names
rails-kit related app/models/user.rb    # accepts file paths too
rails-kit related app/views/admin/billing/invoices/show.html.erb
rails-kit related app/services/admin/user_export_service.rb
rails-kit related user --json   # output as JSON
```

Supported path inputs include model, controller, view, decorator, former, service, datagrid, model/controller test and spec, and fixture paths. Paths are resolved back to the owning model before related-file lookup runs.

Matches stay within the exact namespace of the requested model. For example, `rails-kit related user` will not include `admin/users_controller.rb`, `app/views/admin/users/...`, or `app/services/admin/...`, and `rails-kit related admin/user` will not include `app/services/admin/reports/...`.

Related-file search roots are configurable for non-standard project layouts.

### fixtures

```sh
rails-kit fixtures              # list available fixture files
rails-kit fixtures users        # show all users fixtures
rails-kit fixtures user         # singular form works too
rails-kit fixtures --json       # list fixture files as JSON
rails-kit fixtures user --json  # show normalized fixture data as JSON
```

Rails fixture metadata entries like `_fixture` are hidden. ERB-derived scalar values, including multiline block scalars and list items, are shown as `__ERB__`.

`fixtures --json` returns the normalized visible fixture data after metadata removal and ERB placeholder normalization.

Fixture files that rely on structural ERB such as loops, conditionals, or ERB-generated fixture names error clearly instead of showing misleading synthetic records.

If `fixtures_path` is configured but missing, the command errors with the bad path instead of pretending the fixture name was not found.

### locales

```sh
rails-kit locales               # list nested scopes like en.views.users
rails-kit locales en.views.users        # show keys under scope
rails-kit locales en.activerecord.models
rails-kit locales --json                # list scopes as JSON
rails-kit locales en.views.users --json # show scoped subtree as JSON
```

Arrays and nested composite values are rendered in YAML-like multiline form.

`locales` with no scope lists navigable nested map scopes such as `en.views` and `en.views.users`. `locales --json` returns those scopes as a JSON array, and `locales <scope> --json` returns the resolved subtree or leaf value under that scope.

### model

```sh
rails-kit model user            # show User model structure
rails-kit model order_item      # underscored names work
rails-kit model app/models/user.rb     # file path also accepted
rails-kit model user --json     # output as JSON
```

The summary includes the model's parent class and any custom `self.table_name`, plus extracted concerns, associations, validations, scopes, callbacks, enums, and delegates.

Example output:

```
User < ApplicationRecord (app/models/user.rb)
========================================

Concerns:
  Searchable
  Auditable

Associations:
  belongs_to :account
  has_many :orders, dependent: destroy

Validations:
  validates :email, presence, uniqueness
  validate :email_not_banned (custom)

Scopes:
  active
  by_email(email)

Callbacks:
  before_validation :normalize_email
  after_save :sync_external_profile

Enums:
  role
```

### completion

```sh
rails-kit completion bash > /etc/bash_completion.d/rails-kit
rails-kit completion zsh > "${fpath[1]}/_rails-kit"
rails-kit completion fish > ~/.config/fish/completions/rails-kit.fish
```

## Configuration

Optional `.rails-kit.yml` at Rails root:

```yaml
# All fields optional, shown with defaults
schema_path: db/schema.rb
fixtures_path: test/fixtures
locales_path: config/locales
models_path: app/models
controllers_path: app/controllers
views_path: app/views
decorators_path: app/decorators
formers_path: app/formers
services_path: app/services
datagrids_path: app/datagrids
test_models_path: test/models
test_controllers_path: test/controllers
spec_models_path: spec/models
spec_controllers_path: spec/controllers
spec_fixtures_path: spec/fixtures

# Additional irregular plurals (merged with built-ins)
plurals:
  curriculum: curricula
  syllabus: syllabi
```

Relative paths are resolved from the Rails root. Absolute paths are supported for all configured directories/files.

Unknown config keys are rejected so typos in `.rails-kit.yml` fail fast instead of silently falling back to defaults.

## Build

Requires [Just](https://github.com/casey/just) and Go 1.26.1.

```sh
mise install
mise exec -- just build          # build ./rails-kit
mise exec -- just install        # build and install to ~/go/bin
mise exec -- just test           # run tests
mise exec -- just test-coverage  # test with coverage report
mise exec -- just lint           # run golangci-lint
```

## Claude Code Skill

```sh
rails-kit skill install
rails-kit skill install --global
rails-kit skill uninstall
rails-kit skill uninstall --global
```

Local install/uninstall targets the detected Rails root or `--root`. Local uninstall validates that `--root` is a Rails app before removing anything.

## Known Differences from Ruby Scripts

| Area | Behavior |
|------|----------|
| Key ordering in locales/fixtures | Go sorts keys alphabetically; Ruby preserves YAML insertion order |
| ERB in fixtures | Go replaces scalar ERB-derived values with `__ERB__`, rejects structural ERB that would change fixture layout, and skips Rails metadata entries like `_fixture`; Ruby evaluates ERB and uses metadata internally |
| Rails root detection | Go walks up from CWD; Ruby walked from binary location |
| Pluralization | Go handles more suffix rules than the original Ruby map |
