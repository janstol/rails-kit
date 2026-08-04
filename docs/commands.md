# Command Reference

All commands auto-detect the Rails root by walking up from the current directory. Override it with `--root /path/to/app`. Data commands support `--json`.

## JSON output

Every `--json` invocation writes one envelope object — `{ "schema_version": 1, "command": "...", "data": {...} }` on success, or an `error` object on stderr with exit code 1 on failure. `data` is always a JSON object; a command's shape depends only on its mode (list vs. detail), never on result count. See [`docs/json.md`](json.md) for the full contract: per-command `data` shapes, the error-code vocabulary, and the stability policy.

## `about`

```sh
rails-kit about
rails-kit about --json
rails-kit about --runtime
```

Static inspection reads application, lockfile, Ruby version-manager, literal database adapter, and schema metadata without evaluating ERB, credentials, initializers, or application code. Missing optional metadata produces warnings with a successful partial report.

`--runtime` runs a bounded `bundle exec rails runner` call to report active Rails, Ruby, RubyGems, Rack, Bundler, environment, and database adapter values. Boot noise is ignored. A boot failure retains the static report and adds a warning.

## `schema`

```sh
rails-kit schema
rails-kit schema users
rails-kit schema users posts
rails-kit schema users --json
```

Supports both Ruby schema files and PostgreSQL `structure.sql` output. If the configured format is absent, rails-kit tries the alternate conventional format.

## `routes`

```sh
rails-kit routes
rails-kit routes users posts
rails-kit routes --refresh
rails-kit routes --no-cache
rails-kit routes --static
rails-kit routes --json
rails-kit routes --watch
rails-kit routes --static --watch
rails-kit routes --watch --watch-interval 2s
```

The default command runs `bundle exec rails routes` and caches output in `tmp/routes_cache.txt`. Changes under `config/routes.rb` or `config/routes/` invalidate the cache. `--refresh` forces regeneration; `--no-cache` bypasses it.

`--static` reads route files without Ruby, Bundler, or a Rails boot. It is fast but approximate and cannot be combined with `--refresh` or `--no-cache`. See [Static Routes](static-routes.md) for supported DSL forms and limitations.

`--watch` keeps rails-kit running and reprints whenever `config/routes.rb` or any file under `config/routes/` changes, polling mtimes at `--watch-interval` (default `1s`, minimum `100ms`). It composes with `--static`, patterns, and `--json`. On a TTY the screen clears before each reprint; otherwise output is appended with a timestamped header, so it stays pipe-safe. A render error (e.g. a syntax error while editing routes) is reported on stderr but does not stop watching. Exit with Ctrl-C.

## `related`

```sh
rails-kit related user
rails-kit related order_item
rails-kit related app/models/user.rb
rails-kit related app/views/admin/users/show.html.erb
rails-kit related user --json
```

Accepts model names and supported Rails file paths, resolves the owning model, and searches configured model, controller, view, decorator, job, mailer, former, service, datagrid, test, spec, and fixture roots. Results remain within the requested namespace.

## `fixtures`

```sh
rails-kit fixtures
rails-kit fixtures users
rails-kit fixtures user --json
```

Rails metadata entries are hidden. Scalar ERB-derived values are represented as `__ERB__`; structural ERB that could change the fixture layout fails clearly instead of producing misleading records.

## `locales`

```sh
rails-kit locales
rails-kit locales en.views.users
rails-kit locales en.activerecord.models --json
```

With no scope, lists navigable nested map scopes. With a scope, returns the subtree or leaf value. Composite values use a YAML-like multiline format in human output.

## `model`

```sh
rails-kit model user
rails-kit model order_item
rails-kit model app/models/user.rb
rails-kit model user --json
```

Summarizes the class, parent, custom table name, concerns, associations, validations, scopes, callbacks, enums, and delegates. Parsing is static, AST-backed by Prism, and intentionally compact. Recoverable Ruby syntax errors produce line-specific warnings on stderr while successfully recovered fields remain on stdout, including in JSON mode.

## `skeleton`

```sh
rails-kit skeleton user
rails-kit skeleton app/services/user_export_service.rb
rails-kit skeleton user app/jobs/sync_user_job.rb
rails-kit skeleton 'app/jobs/*.rb' --json
rails-kit skeleton app/services
rails-kit skeleton app --exclude 'app/generated/**'
```

Produces AST-backed Ruby structure without evaluating code or loading Rails. Inputs can be model names, files, recursive directories, or quoted globs. Multiple inputs are validated and processed in one Ruby invocation; duplicate files are inspected once.

Repeat `--exclude` with Rails-root-relative glob patterns to prune directory discovery. Exclusions do not affect explicitly named files, models, or standalone glob matches. Batches are limited to 500 unique files.

Requires Ruby with Prism. rails-kit first tries Ruby on `PATH`, then the user's interactive shell so common version managers can activate.

## `gem`

```sh
rails-kit gem
rails-kit gem rails
rails-kit gem nokogiri --json
```

Parses `Gemfile.lock` without Ruby or Bundler. Detailed output includes version, source type and URL, git metadata, and dependencies.

## `concerns`

```sh
rails-kit concerns
rails-kit concerns searchable
rails-kit concerns model/searchable
rails-kit concerns controller/authenticatable --json
```

Inspects model and controller concerns, including methods, class methods, and `included` or `class_methods` blocks. Qualify the type when the same name exists in both directories.

## `controllers`

```sh
rails-kit controllers
rails-kit controllers users
rails-kit controllers admin/reports
rails-kit controllers Admin::ReportsController --json
```

Summarizes a controller's filters (`before_action`/`after_action`/`around_action` and their
`skip_*` variants, with `only:`/`except:`/`if:`/`unless:`), `rescue_from` handlers, helper
methods, `layout`, class-level `respond_to`, strong params (`params.require(...).permit(...)`),
and public action methods. Parsing is static, AST-backed by Prism, single-file only: a
controller's own declarations are shown, not ones inherited from `ApplicationController` or any
other superclass -- `parent_class` says where to look next. Recoverable Ruby syntax errors
produce line-specific warnings on stderr while successfully recovered fields remain on stdout,
including in JSON mode.

## `mailers`

```sh
rails-kit mailers
rails-kit mailers user
rails-kit mailers admin/notification
rails-kit mailers Admin::NotificationMailer --json
```

Summarizes a mailer's `default` headers, `layout`, included concerns, attachments (regular and
inline, collected from inside action methods regardless of visibility), and public action
methods. Parsing is static, AST-backed by Prism, single-file only: a mailer's own declarations
are shown, not ones inherited from `ApplicationMailer` or any other superclass -- `parent_class`
says where to look next. Recoverable Ruby syntax errors produce line-specific warnings on stderr
while successfully recovered fields remain on stdout, including in JSON mode.

## `jobs`

```sh
rails-kit jobs
rails-kit jobs sync_user
rails-kit jobs admin/export
rails-kit jobs Admin::ExportJob --json
```

Summarizes an ActiveJob's `queue_as`, `retry_on`/`discard_on` handlers (exception classes plus
`wait`/`attempts`/`wait_jitter`/`queue`/`priority` options, in a fixed order), included concerns,
and public methods -- `perform` is a public method like any other and surfaces in the methods
list, not a dedicated section. Parsing is static, AST-backed by Prism, single-file only: a job's
own declarations are shown, not ones inherited from `ApplicationJob` or any other superclass --
`parent_class` says where to look next. Recoverable Ruby syntax errors produce line-specific
warnings on stderr while successfully recovered fields remain on stdout, including in JSON mode.

## `completion`

```sh
rails-kit completion bash > /etc/bash_completion.d/rails-kit
rails-kit completion zsh > "${fpath[1]}/_rails-kit"
rails-kit completion fish > ~/.config/fish/completions/rails-kit.fish
```

Completions are dynamic. `model`, `related`, and `skeleton` complete model names;
`schema` completes table names; `locales` completes dotted scopes one level at a
time; `concerns`, `fixtures`, and `gem` complete their respective names — all read
from the current Rails project. `routes` and other flag-only commands are
unaffected.
