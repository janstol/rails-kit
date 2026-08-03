# `--json` output contract

`--json` is a persistent flag accepted by every data command. This page defines the contract:
the envelope, the shape of each command's `data`, the error-code vocabulary, and the stability
policy that governs future changes.

## Envelope

Every successful `--json` invocation writes exactly one JSON object to **stdout**:

```json
{ "schema_version": 1, "command": "model", "data": { "class_name": "User", "...": "..." } }
```

Every failing `--json` invocation writes exactly one JSON object to **stderr**, leaves stdout
empty, and exits with status 1:

```json
{ "schema_version": 1, "command": "model", "error": { "code": "not_found", "message": "model \"nope\" not found" } }
```

`command` is the invoked subcommand's name (e.g. `model`, `schema`, `gem` — not `rails-kit`).

**Known limitation:** if cobra's own flag parsing fails before `--json` can be read (e.g. an
unknown flag), the error is plain text on stderr regardless of intent, because `--json` was
never parsed from the arguments.

## Invariants

1. **`data` is always a JSON object** — never an array, never a scalar. Arrays live under a
   named key (e.g. `{"routes": [...]}`), not at the top level.
2. **A command's `data` shape depends only on its mode** (list vs. detail), never on how many
   results came back. A one-file `skeleton` invocation and a hundred-file one both return
   `{"files": [...]}`.
3. **Unknown fields must be ignored by consumers.** Adding a field to `data` or `error`, adding
   an error code, or adding a new command is not a breaking change.

## Stability policy

- `schema_version` bumps only on a breaking change: a field removed or renamed, a field's type
  changed, or an existing error code's meaning changed.
- Additive changes — new fields, new error codes, new commands — do **not** bump it.
- `omitempty` fields may be absent from the JSON entirely; absent means empty or zero, not
  "unknown."
- **Pre-1.0 caveat:** while rails-kit is below 1.0, `schema_version` may bump in a minor
  release. From 1.0 on, a bump is a major-version event.

## Error codes

A small, closed vocabulary. Adding a code is additive; changing what an existing one means is
breaking. Anything not explicitly listed below falls through to `internal` — an honest fallback
rather than a fabricated taxonomy.

| Code | Meaning |
|---|---|
| `not_a_rails_root` | `--root` or auto-detection did not find a Rails project (`config/application.rb` missing) |
| `not_found` | A named lookup (model, gem, concern, fixture, locale scope) had no match |
| `invalid_argument` | Conflicting or malformed flags/values (e.g. `--refresh` with `--no-cache`, a bad `--color` value) |
| `parse_error` | A schema, `Gemfile.lock`, routes, or Ruby/Prism parse failure |
| `internal` | Fallback for any error not covered above |

## Commands and their `data` shapes

### `about`

```json
{ "application": "MyApp", "root": "/path", "environment": "development", "source": "static", "versions": {...}, "database": {...}, "warnings": [...] }
```

- `application` (string, omitempty), `root` (string), `environment` (string), `source` (string — `"static"` or `"runtime"`)
- `versions`: `{ "rails", "ruby", "rubygems", "rack", "bundler" }` — all strings, all omitempty
- `database`: `{ "adapters": [string], "schema_format": string }` — both omitempty
- `warnings` ([string], omitempty)

Not covered by a golden file: `root` is an absolute, machine-dependent path.

### `schema`

Single shape for both list and extract modes — `tables` is always an array:

```json
{ "tables": [ { "name": "users" } ] }
{ "tables": [ { "name": "users", "definition": "create_table \"users\" ...\n" } ] }
```

- `name` (string)
- `definition` (string, omitempty) — the raw DDL block text (`create_table` plus its indexes and
  foreign keys), not structured columns. Present only when tables were named as arguments;
  absent in list mode.

### `routes`

```json
{ "routes": [ { "prefix": "users", "verb": "GET", "uri_pattern": "/users", "controller_action": "users#index" } ] }
```

Each entry: `prefix`, `verb`, `uri_pattern`, `controller_action` (all strings, no omitempty —
Rails' own `rails routes` output can have blank prefixes/verbs, e.g. for `match ... via: :all`
or mount points).

### `related`

```json
{ "model": "user", "plural": "users", "categories": [ { "label": "Model", "files": ["app/models/user.rb"] } ] }
```

- `model`, `plural` (strings)
- `categories`: array of `{ "label": string, "files": [string] }`

### `model`

```json
{ "class_name": "User", "parent_class": "ApplicationRecord", "rel_path": "app/models/user.rb", "table_name": "accounts", "concerns": [...], "associations": [...], "validations": [...], "scopes": [...], "callbacks": [...], "enums": [...], "delegates": [...] }
```

`class_name` and `rel_path` are always present; every other field is `omitempty` (absent when
the model has none of that kind). Recoverable Ruby parse errors are reported as warnings on
stderr; partial model data retains this schema and remains valid JSON on stdout.

### `fixtures`

List mode:

```json
{ "files": ["users", "orders"] }
```

Detail mode (distinct key — genuinely a different operation, still an object):

```json
{ "file": "users.yml", "entries": { "alice": { "name": "Alice", "email": "__ERB__" } } }
```

`entries` is a map keyed by fixture name; values are the fixture's visible fields (Rails
metadata entries hidden, ERB-derived scalars replaced with `"__ERB__"`).

### `locales`

List mode:

```json
{ "scopes": ["en.views.users", "en.time.formats"] }
```

Detail mode:

```json
{ "scope": "en.views.users", "value": { "title": "Users" } }
```

`value` mirrors whatever the locale tree holds at that scope — an object, a string, or another
nested structure. No fixed schema beyond "valid JSON."

### `gem`

List mode:

```json
{ "gems": [ { "name": "rails", "version": "7.1.3" } ] }
```

Detail mode returns the full gem object directly under `data` (not list-wrapped, since it's
already an object):

```json
{ "name": "rails", "version": "7.1.3", "source": "rubygems", "source_url": "https://rubygems.org/", "dependencies": [ { "name": "activesupport", "constraint": ">= 6.0" } ] }
```

- `name`, `version`, `source` (`"rubygems"|"git"|"path"`), `source_url` — always present
- `revision`, `branch`, `tag`, `ref` (strings, omitempty) — git source metadata
- `dependencies` (array of `{ "name", "constraint" (omitempty) }`, omitempty)

### `concerns`

List mode:

```json
{ "model_concerns": ["searchable"], "controller_concerns": ["authenticatable"] }
```

Detail mode returns the full concern object directly under `data`:

```json
{ "name": "Searchable", "path": "app/models/concerns/searchable.rb", "type": "model", "methods": [...], "class_methods": [...], "has_included_block": true, "has_class_methods_block": false }
```

`methods` and `class_methods` are `omitempty`; `has_included_block` and
`has_class_methods_block` are always present.

### `controllers`

List mode:

```json
{ "controllers": ["admin/reports", "application", "users"] }
```

Detail mode returns the full controller object directly under `data`:

```json
{ "class_name": "UsersController", "parent_class": "ApplicationController", "rel_path": "app/controllers/users_controller.rb", "filters": [...], "rescue_from": [...], "helper_methods": [...], "layout": "\"users\"", "respond_to": [...], "strong_params": [...], "actions": [...] }
```

`class_name` and `rel_path` are always present; every other field is `omitempty` (absent when
the controller has none of that kind). Only the controller's own file is parsed -- filters and
other declarations inherited from a superclass are not resolved; `parent_class` names it.

### `skeleton`

Always an array under `files`, regardless of how many inputs resolved — this is the shape this
contract exists to fix (previously a bare object for one file, an array for many):

```json
{ "files": [ { "path": "/abs/path/app/models/user.rb", "rel_path": "app/models/user.rb", "classes": [...], "modules": [...], "constants": [...], "calls": [...], "methods": [...], "parse_errors": [...] } ] }
```

Every field but `path` is `omitempty`. `parse_errors` (strings) is present only when Prism
reported syntax errors for that file; the rest of the summary is best-effort in that case.

### `version`

```json
{ "version": "0.3.0", "commit": "abc1234", "build_date": "2026-07-25T00:00:00Z" }
```

Not covered by a golden file: all three fields are build-time metadata.
