---
name: rails-kit
description: Use rails-kit CLI to explore Rails codebase -- schema, routes, models, fixtures, locales, and related files. Use before reading large files.
allowed-tools: Bash(rails-kit *)
model: haiku
---

`rails-kit` is a compiled Go binary for reading the codebase without loading Rails or large files. It is installed globally and should be invoked as `rails-kit`, not `bin/rails-kit`. It auto-detects the Rails root by walking up from the current directory. Use these commands before reaching for `cat`, `grep`, or `Read` on schema/routes/locales/fixtures.

**`--json` flag:** All data commands (`schema`, `routes`, `related`, `model`, `fixtures`, `locales`, `gem`) accept `--json` for machine-readable output, useful for piping or structured processing. JSON shapes:
- `schema` (no args) → `[]string`; with table args → object keyed by table name
- `routes` → `[{ prefix, verb, uri_pattern, controller_action }]`
- `related` → `{ model, plural, categories }`
- `model` → `{ class_name, parent_class?, rel_path, table_name?, concerns, associations, validations, scopes, callbacks, enums, delegates }`
- `fixtures` (no args) → `[]string`; with name → `{ file, entries }`
- `locales` (no args) → `[]string`; with scope → `{ scope, value }`
- `gem` (no args) → `[{ name, version }]`; with name → `{ name, version, source, source_url, revision?, branch?, tag?, ref?, dependencies? }`

## When to use these tools

| Need | Tool |
|------|------|
| Check what tables exist or inspect a table's columns | `rails-kit schema` |
| Find a route URL or verify a controller action exists | `rails-kit routes` |
| Find all files related to a model before starting work | `rails-kit related` |
| Inspect test fixtures for a model | `rails-kit fixtures` |
| Look up a locale key or browse translation scopes | `rails-kit locales` |
| Understand a model's associations, scopes, validations | `rails-kit model` |
| Check gem versions or find where a gem comes from | `rails-kit gem` |

---

## rails-kit schema

Extract `create_table` blocks from `db/schema.rb`.

```bash
rails-kit schema                    # list all table names
rails-kit schema users              # show users table columns, indexes, foreign keys
rails-kit schema users orders       # show multiple tables
```

Use this before reading a model file when you need to know column names and types.

---

## rails-kit routes

Filtered `rails routes` output, cached in `tmp/routes_cache.txt`.

```bash
rails-kit routes                    # all routes
rails-kit routes users              # routes matching "users"
rails-kit routes products orders    # routes matching either term
```

Use this to find path helpers, verify controller actions exist, or check what HTTP methods are available.

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
