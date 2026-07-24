# rails-kit

A compiled CLI toolkit for inspecting Rails projects. Most commands parse project files directly without loading Rails. The default `routes` mode runs `bundle exec rails routes`, while `routes --static` provides a fast, pure-Go approximation without booting Rails. The `skeleton` command uses Ruby and Prism but does not load the Rails application.

## Commands

| Command | Description |
|---------|-------------|
| `rails-kit schema [table...]` | Extract table definitions from `db/schema.rb` or `db/structure.sql` |
| `rails-kit routes [pattern...]` | Cached Rails routes or fast offline static parsing |
| `rails-kit related <name>` | List all files related to a model |
| `rails-kit fixtures [name]` | Summarize test fixture entries |
| `rails-kit locales [scope]` | Extract locale keys by scope |
| `rails-kit model <name>` | Compact model structure summary |
| `rails-kit skeleton <path-or-model> [...]` | Compact Ruby AST skeletons via Prism |
| `rails-kit gem [name]` | Inspect gems from `Gemfile.lock` |
| `rails-kit concerns [name]` | List or inspect Rails concerns |
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

Most inspection commands support `--json` for machine-readable output suitable for scripts, editor tooling, and AI/agent workflows.

JSON shapes:

- `schema` with no table args returns `[]string`; with table args returns an object keyed by table name
- `routes` returns `[{ prefix, verb, uri_pattern, controller_action }]`
- `related` returns `{ model, plural, categories }`
- `model` returns `{ class_name, parent_class?, rel_path, table_name?, concerns, associations, validations, scopes, callbacks, enums, delegates }`
- `fixtures` with no name returns `[]string`; with a name returns `{ file, entries }`
- `locales` with no scope returns `[]string`; with a scope returns `{ scope, value }`
- `gem` with no name returns `[{ name, version }]`; with a name returns `{ name, version, source, source_url, revision?, branch?, tag?, ref?, dependencies? }`
- `skeleton` returns `{ path, rel_path, classes, modules, constants, calls, methods, parse_errors? }` for one resolved file, or an array of those objects for multiple files
- `concerns` with no name returns `{ model_concerns, controller_concerns }`; with a name returns `{ name, path, type, methods, class_methods, has_included_block, has_class_methods_block }`
- `version` returns `{ version, commit, build_date }`

### schema

```sh
rails-kit schema                # list all table names
rails-kit schema users          # show users table definition
rails-kit schema users posts    # show multiple tables
rails-kit schema --json         # list table names as JSON array
rails-kit schema users --json   # extract tables as JSON object keyed by table name
```

Both `db/schema.rb` (Ruby DSL) and `db/structure.sql` (PostgreSQL DDL, from `pg_dump`) are supported. The format is detected by file extension. If `db/schema.rb` is not found, `db/structure.sql` is tried automatically. Override via `schema_path` in `.rails-kit.yml`.

### routes

```sh
rails-kit routes                # show all routes (uses cache)
rails-kit routes users          # show routes matching "users"
rails-kit routes users posts    # show routes matching either term
rails-kit routes --refresh      # force cache regeneration
rails-kit routes --no-cache     # skip cache entirely (don't read or write)
rails-kit routes --json         # all routes as JSON array
rails-kit routes users --json   # filtered routes as JSON array
rails-kit routes --static       # parse config/routes.rb directly, no Rails boot
```

Routes are cached in `tmp/routes_cache.txt`. Cache is invalidated when `config/routes.rb` or any file in `config/routes/` changes. `--refresh` and `--no-cache` are mutually exclusive.

`routes --json` parses the standard tabular `rails routes` output, skips leading boot noise before the header, and correctly handles rows with a blank route prefix. If Rails emits non-tabular output instead of the usual routes table, JSON output errors clearly instead of returning an ambiguous empty array.

#### `--static`: offline routes without booting Rails

`--static` parses `config/routes.rb` directly in pure Go — no `bundle exec`, no Rails boot. It's fast and works even when the app can't boot (wrong Ruby version, missing gems, a broken initializer). If the normal `routes` command fails because Rails won't boot, the error suggests `--static` as a fallback.

This is an **approximation**, not a replacement for `rails routes`. It understands `resources`/`resource`, `namespace`, `scope module:`, `root`, and individual verb routes (`get`/`post`/`put`/`patch`/`delete`), including nesting, `member`/`collection` blocks, explicit or inferred controller actions and helper names, resource `path`/`controller`/`as`/`param` options, and scalar, array, `%i[...]`, or `%w[...]` action filters. It does **not** expand engine mounts, `draw`/`concern` macros, redirects, or routes drawn by gems (e.g. Devise). Constraints are retained as approximate routes and produce warnings because the condition is not modeled. Unsupported or partially modeled route DSL is reported as a warning on stderr while successfully parsed routes remain on stdout, including in JSON mode. Use it to get a quick answer for "what routes exist for X" when Rails can't boot or a fast answer is more valuable than a complete one — not as a source of truth for CI or production route audits.

`--static` cannot be combined with `--refresh` or `--no-cache` (it never touches the cache).

### related

```sh
rails-kit related user          # find all files for User model
rails-kit related order_item    # works with underscored names
rails-kit related app/models/user.rb    # accepts file paths too
rails-kit related app/views/admin/billing/invoices/show.html.erb
rails-kit related app/services/admin/user_export_service.rb
rails-kit related user --json   # output as JSON
```

Supported path inputs include model, controller, view, decorator, job, mailer, former, service, datagrid, model/controller test and spec, request/system/helper/job/mailer/service spec, and fixture paths. Paths are resolved back to the owning model before related-file lookup runs.

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

### skeleton

```sh
rails-kit skeleton user                         # Prism skeleton for app/models/user.rb
rails-kit skeleton app/services/user_export_service.rb
rails-kit skeleton app/jobs/sync_user_job.rb
rails-kit skeleton lib/custom_importer.rb --json
rails-kit skeleton user app/services/user_export_service.rb
rails-kit skeleton 'app/jobs/*.rb' --json       # rails-kit expands quoted globs
rails-kit skeleton app/services                 # recursively inspect Ruby files
rails-kit skeleton app --exclude 'app/generated/**' --exclude '**/*_generated.rb'
```

Shows a compact AST-backed structure for Ruby files without evaluating Ruby or loading Rails. Output includes class/module nesting, superclass names, constants, includes/extends/prepends, Rails macros, generic top-level DSL calls, method signatures, and source line numbers. Method bodies and comments are omitted.

Multiple model names, Ruby paths, directories, and glob patterns can be combined. Relative globs are expanded from the Rails root, directory files are discovered recursively in lexical order, and duplicate files are inspected once. Directory traversal does not follow directory symlinks.

Repeat `--exclude` with Rails-root-relative patterns to prune directory discovery. Patterns support `*`, `?`, character classes, and `**` across directories. Exclusions do not affect explicitly named files, models, or standalone glob matches, and there are no implicit ignored directories. At most 500 unique files can be inspected in one command.

All inputs are validated before Ruby starts; an invalid input, empty directory, unmatched glob, or oversized batch fails the command without partial output.

The command processes all resolved files in one Ruby invocation and requires the `prism` library. Prism is bundled with modern CRuby releases; older projects may need the `prism` gem available. `skeleton` first tries the Ruby on `PATH`; if Prism is unavailable, it retries through the user's interactive shell from the Rails root so common Ruby version managers can activate normally. Other `rails-kit` commands do not require Prism.

Use `skeleton` for larger non-model Ruby files such as services, jobs, mailers, decorators, POROs, and `lib/` files. `model` remains the smaller Rails-specific summary for Active Record models.

### gem

```sh
rails-kit gem                   # list all gems with versions
rails-kit gem rails             # show rails gem details
rails-kit gem nokogiri --json   # gem detail as JSON
rails-kit gem --json            # all gems as JSON array
```

Parses `Gemfile.lock` directly — no Ruby or Bundler required. Shows each gem's version, source type (`rubygems`, `git`, or `path`), source URL, git metadata (revision, branch, tag, ref), and dependencies.

The lock file path defaults to `Gemfile.lock` at the Rails root and can be overridden with `gemfile_lock_path` in `.rails-kit.yml`.

### concerns

```sh
rails-kit concerns                          # list all concerns grouped by type
rails-kit concerns searchable               # show Searchable concern details
rails-kit concerns model/searchable         # qualify type to disambiguate
rails-kit concerns controller/authenticatable
rails-kit concerns --json                   # list concerns as JSON
rails-kit concerns searchable --json        # concern detail as JSON
```

Shows concerns from `app/models/concerns/` and `app/controllers/concerns/`. Parsed details include the module name, methods, class methods, and whether the concern uses an `included do` or `class_methods do` block.

When a concern name exists in both model and controller directories, qualify it with `model/` or `controller/` to disambiguate.

The concern directories default to `app/models/concerns` and `app/controllers/concerns` and can be overridden with `model_concerns_path` and `controller_concerns_path` in `.rails-kit.yml`.

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
schema_path: db/schema.rb  # or db/structure.sql for SQL-format projects
fixtures_path: test/fixtures
locales_path: config/locales
models_path: app/models
controllers_path: app/controllers
views_path: app/views
decorators_path: app/decorators
jobs_path: app/jobs
mailers_path: app/mailers
formers_path: app/formers
services_path: app/services
datagrids_path: app/datagrids
test_models_path: test/models
test_controllers_path: test/controllers
spec_models_path: spec/models
spec_controllers_path: spec/controllers
spec_fixtures_path: spec/fixtures
spec_requests_path: spec/requests
spec_system_path: spec/system
spec_helpers_path: spec/helpers
spec_jobs_path: spec/jobs
spec_mailers_path: spec/mailers
spec_services_path: spec/services
gemfile_lock_path: Gemfile.lock
model_concerns_path: app/models/concerns
controller_concerns_path: app/controllers/concerns

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
