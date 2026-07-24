---
name: rails-kit
description: Use rails-kit CLI to explore Rails codebase -- schema, routes, models, skeletons, fixtures, locales, and related files. Use before reading large files.
allowed-tools: Bash(rails-kit *)
model: haiku
---

`rails-kit` is a compiled Go binary for inspecting a Rails codebase without reading large files. Most commands parse project files directly without loading Rails. The default `routes` mode boots Rails through Bundler, while `routes --static` provides a fast, pure-Go approximation. The `skeleton` command uses Ruby and Prism without loading the Rails application. The binary is installed globally and should be invoked as `rails-kit`, not `bin/rails-kit`. It auto-detects the Rails root by walking up from the current directory. Use these commands before reaching for `cat`, `grep`, or `Read` on schema/routes/locales/fixtures or large Ruby files.

**`--json` flag:** All data commands (`schema`, `routes`, `related`, `model`, `skeleton`, `fixtures`, `locales`, `gem`, `concerns`) accept `--json` for machine-readable output, useful for piping or structured processing. JSON shapes:
- `schema` (no args) → `[]string`; with table args → object keyed by table name
- `routes` → `[{ prefix, verb, uri_pattern, controller_action }]`
- `related` → `{ model, plural, categories }`
- `model` → `{ class_name, parent_class?, rel_path, table_name?, concerns, associations, validations, scopes, callbacks, enums, delegates }`
- `skeleton` → one `{ path, rel_path, classes, modules, constants, calls, methods, parse_errors? }` object, or an array for multiple resolved files
- `fixtures` (no args) → `[]string`; with name → `{ file, entries }`
- `locales` (no args) → `[]string`; with scope → `{ scope, value }`
- `gem` (no args) → `[{ name, version }]`; with name → `{ name, version, source, source_url, revision?, branch?, tag?, ref?, dependencies? }`
- `concerns` (no args) → `{ model_concerns, controller_concerns }`; with name → `{ name, path, type, methods, class_methods, has_included_block, has_class_methods_block }`

## When to use these tools

| Need | Tool |
|------|------|
| Check what tables exist or inspect a table's columns | `rails-kit schema` |
| Find a route URL or identify its routed controller action | `rails-kit routes` |
| Find all files related to a model before starting work | `rails-kit related` |
| Inspect test fixtures for a model | `rails-kit fixtures` |
| Look up a locale key or browse translation scopes | `rails-kit locales` |
| Understand a model's associations, scopes, validations | `rails-kit model` |
| Inspect a large Ruby service/job/mailer/PORO without reading method bodies | `rails-kit skeleton` |
| Check gem versions or find where a gem comes from | `rails-kit gem` |
| List or inspect model/controller concerns | `rails-kit concerns` |

---

## rails-kit schema

Extract table definitions from `db/schema.rb` or `db/structure.sql`. Both formats are supported and auto-detected. Projects using `config.active_record.schema_format = :sql` have `structure.sql` (PostgreSQL DDL) instead of `schema.rb` — rails-kit handles both transparently.

```bash
rails-kit schema                    # list all table names
rails-kit schema users              # show users table columns, indexes, foreign keys
rails-kit schema users orders       # show multiple tables
```

Use this before reading a model file when you need to know column names and types.

---

## rails-kit routes

By default, runs and filters `bundle exec rails routes`, caching the output in `tmp/routes_cache.txt`. Use `--static` for fast offline parsing without booting Rails.

```bash
rails-kit routes                    # all routes
rails-kit routes users              # routes matching "users"
rails-kit routes products orders    # routes matching either term
rails-kit routes --static           # parse config/routes.rb directly, no Rails boot
```

Use this to find path helpers, verify controller actions exist, or check what HTTP methods are available.

`--static` skips `bundle exec rails routes` entirely and parses `config/routes.rb` in pure Go. Use it when Rails won't boot, or when a fast approximate answer beats a slow exact one. It understands `resources`/`resource`, `namespace`, `scope module:`, `root`, recursively drawn files under `config/routes`, static block-defined route concerns, and verb routes, including nesting, `member`/`collection` blocks, inferred or explicit controller actions and helper names, resource `path`/`controller`/`as`/`param`/`concerns` options, and scalar, array, `%i[...]`, or `%w[...]` action filters. Drawn files and concerns inherit their surrounding scope; invalid or cyclic expansions produce source-specific warnings. Custom runtime `draw_paths`, callable or parameterized concerns, and concern option propagation are not modeled. The parser does not expand engine mounts, redirects, or gem-drawn routes. Constraints are retained as approximate routes and produce warnings because their conditions are not modeled.

---

## rails-kit related

List all files related to a model: model, controller, views, decorator, job, mailer, former, service, datagrid, tests, specs (model, controller, request, system, helper, job, mailer, service), fixtures.

```bash
rails-kit related user
rails-kit related order
rails-kit related app/models/user.rb   # accepts file paths too
rails-kit related app/views/users/show.html.erb
rails-kit related app/services/user_export_service.rb
```

Run this first when starting work on a model to get a complete map of relevant files. Supported path inputs include model, controller, view, decorator, job, mailer, former, service, datagrid, test/spec (including request, system, helper, job, mailer, service specs), and fixture paths; each path is resolved back to its owning model first. Matches stay in the exact namespace you asked for, so `rails-kit related user` will not include `admin/users_controller.rb`, `app/views/admin/users/...`, or `app/services/admin/...`.

---

## rails-kit fixtures

Compact summary of fixture entries.

```bash
rails-kit fixtures                  # list available fixture files
rails-kit fixtures users            # show all users fixtures (name: ..., email: ...)
rails-kit fixtures user             # singular form works too
```

Use this to find fixture names for tests or check what test data exists.

Note: ERB-derived scalar values, including multiline block scalars and list items, are replaced with `__ERB__`; Rails metadata entries like `_fixture` are hidden from the summary; and fixture files that rely on structural ERB fail with a clear error instead of a best-effort summary.

---

## rails-kit locales

Extract locale keys by scope from `config/locales/*.yml`. Deep-merges all locale files.

```bash
rails-kit locales                          # list all depth-2 scopes (en.views, en.time, ...)
rails-kit locales en.views.users           # show all keys under en.views.users
rails-kit locales en.activerecord.models   # show model name translations
rails-kit locales en.activerecord.attributes.user  # show User attribute labels
rails-kit locales en.time.formats          # show time format strings
```

Keys are sorted alphabetically (differs from YAML insertion order). Arrays and nested composite values are shown in YAML-like multiline output.

**Typical workflow when adding I18n keys:**
1. `rails-kit locales` -- see what scopes exist
2. `rails-kit locales en.views.users` -- check existing keys in the target scope
3. Add the new key in the correct place

---

## rails-kit gem

Inspect gems from `Gemfile.lock` — versions and source information.

```bash
rails-kit gem                   # list all gems with versions
rails-kit gem rails             # show rails gem details (source, URL, dependencies)
rails-kit gem nokogiri          # show nokogiri details
```

Use this to quickly check what version of a gem is locked, where it comes from (rubygems, git, or local path), and what its declared dependencies are.

---

## rails-kit skeleton

Compact Ruby AST skeleton via Prism. It shows class/module nesting, superclass names, constants, includes, Rails macros, generic DSL calls, method signatures, and line numbers while omitting method bodies and comments.

```bash
rails-kit skeleton user
rails-kit skeleton app/services/user_export_service.rb
rails-kit skeleton app/jobs/sync_user_job.rb
rails-kit skeleton user app/services/user_export_service.rb
rails-kit skeleton 'app/jobs/*.rb' --json
rails-kit skeleton app/services
rails-kit skeleton app --exclude 'app/generated/**' --exclude '**/*_generated.rb'
```

Use this before reading large non-model Ruby files such as services, jobs, mailers, decorators, POROs, and `lib/` files. `rails-kit model` is still the smaller Rails-specific summary for Active Record models.

Accepts multiple model names, Ruby paths, directories, and quoted glob patterns, processing all resolved files in one Ruby invocation. Directories are recursive; repeatable Rails-root-relative `--exclude` patterns support `**` and apply to directory discovery. Unmatched globs, empty directories, invalid inputs, and batches over 500 unique files fail before output. Requires Ruby with the `prism` library available. `skeleton` first tries the Ruby on `PATH`; if Prism is unavailable, it retries through the user's interactive shell from the Rails root so common Ruby version managers can activate normally. Other rails-kit commands do not require Prism, although the default `routes` mode still requires Ruby, Bundler, and a bootable Rails application.

---

## rails-kit model

Compact structural summary of a model file -- associations, validations, scopes, callbacks, concerns, enums, and delegates. Regex-based, no Rails boot.

```bash
rails-kit model user
rails-kit model order
rails-kit model product
rails-kit model app/models/user.rb   # file path also accepted
```

**Example output:**
```
User < ApplicationRecord (app/models/user.rb)
========================================

Concerns:
  Searchable
  Auditable

Associations:
  belongs_to :account
  belongs_to :role
  has_many :orders, dependent: destroy
  has_many :products, through: orders

Validations:
  validates :first_name, presence
  validates :email, format
  validate :email_not_banned (custom)

Scopes:
  active
  admins
  by_name(param)

Callbacks:
  before_validation :normalize_email
  after_save :sync_external_profile

Enums:
  role
  status

Delegates:
  name (to: :account)
```

**Limitations:**
- Dynamically generated associations (e.g. via `concern` macros) are not shown
- Validations inside `with_options` blocks are captured but without the shared options context
- Lambda bodies are not shown for scopes, only the name and whether it takes args

---

## rails-kit concerns

List or inspect Rails concerns from `app/models/concerns/` and `app/controllers/concerns/`.

```bash
rails-kit concerns                      # list all concerns grouped by type
rails-kit concerns searchable           # show Searchable concern details
rails-kit concerns model/searchable     # qualify type to disambiguate
rails-kit concerns controller/authenticatable
```

Use this to understand what reusable modules are available in the project before adding new behaviour to a model or controller.
