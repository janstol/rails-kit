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

The default command runs `bundle exec rails routes` and caches output in `tmp/routes_cache.txt`. Changes under `config/routes.rb` or `config/routes/` invalidate the cache. `--refresh` forces regeneration; `--no-cache` bypasses it. If those sources change while `rails routes` is running, the result is printed but not cached, so the next invocation regenerates instead of serving stale output.

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

Accepts model names and supported Rails file paths, resolves the owning model, and searches configured model, controller, view, helper, decorator, presenter, job, mailer, former, service, datagrid, test, spec, and fixture roots. Results remain within the requested namespace.

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

Non-string YAML keys (numbers, booleans, `null`, dates) are addressed by their text form, so `locales en.status.404` works.

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

Produces AST-backed Ruby structure without evaluating code or loading Rails. Inputs can be model names, files, recursive directories, or quoted globs. Multiple inputs are validated and processed in one in-process batch; duplicate files are inspected once.

Repeat `--exclude` with Rails-root-relative glob patterns to prune directory discovery. Exclusions do not affect explicitly named files, models, or standalone glob matches. Batches are limited to 500 unique files.

The Prism parser is embedded in the rails-kit binary through pure-Go WASM, so no Ruby installation
is required.

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

## `services`

```sh
rails-kit services
rails-kit services user_export_service
rails-kit services admin/billing_service
rails-kit services Admin::BillingService --json
```

Summarizes a service's parent class (if any), included concerns, class-level constants, and
methods. Services have no universal naming convention (no `_controller`/`_job` suffix) and no
conventional macros, so this reader is thinner than the others: it strips no suffix from file
names and matches the name as given. Both public instance methods (`def call`) and class methods
(`def self.call` -- the common `Service.call` pattern -- or a def inside a `class << self` block)
are collected; a service defined as a module (`module Foo; def self.bar; end; end`) is reported
with `kind: "module"` and no `parent_class`. Parsing is static, AST-backed by Prism, single-file
only: a service's own declarations are shown, not ones inherited from a superclass --
`parent_class` says where to look next. Recoverable Ruby syntax errors produce line-specific
warnings on stderr while successfully recovered fields remain on stdout, including in JSON mode.

## `datagrids`

```sh
rails-kit datagrids
rails-kit datagrids example
rails-kit datagrids admin/report
rails-kit datagrids Admin::ReportDatagrid --json
```

Summarizes a datagrid's parent class, included concerns, decorator (`decorate { X }`), scope
(`scope do…end`, noted as `(block)`), `filter` calls, `column` calls, other class-level DSL
calls (surfaces as macros), and methods (public instance methods plus class methods -- `def
self.x`, or a def inside a `class << self` block). The reader targets the `datagrid` gem DSL
(`filter`/`column`/`scope`/`decorate` on a `BaseDatagrid` subclass, files named `*_datagrid.rb`)
but degrades gracefully: a custom grid implementation or a different grid library in
`app/datagrids/` still resolves (the `_datagrid` suffix is tried first, then the name as given)
and reports a useful summary -- parent class, concerns, methods, and the class-level calls it
does make -- just without the datagrid-gem-specific `filters`/`columns`/`decorate`/`scope`
structure. Filter and column entries render their arguments in source order with whitespace
collapsed; a trailing block literal is noted as ` (block)`, while a block-pass (`&:sym`) is
folded into the argument list. Parsing is static, AST-backed by Prism, single-file only: a
datagrid's own declarations are shown, not ones inherited from a superclass -- `parent_class`
says where to look next. Recoverable Ruby syntax errors produce line-specific warnings on
stderr while successfully recovered fields remain on stdout, including in JSON mode.

## `helpers`

```sh
rails-kit helpers
rails-kit helpers users
rails-kit helpers admin/reports
rails-kit helpers Admin::ReportsHelper --json
```

Summarizes a view helper's included concerns, class-level constants, and methods. Unlike the
other readers, `methods` entries are full signatures (`user_badge(user, size =
DEFAULT_AVATAR_SIZE)`), not bare names -- a helper is an API surface consumed from views, so its
parameters are the useful part; a multi-line parameter list is collapsed onto one line, and a
no-parameter method has no trailing `()`. Both public instance methods and class methods (`def
self.x`, or a def inside a `class << self` block) are collected. Helper files follow the
`_helper.rb` naming convention without exception, so the suffix is tried first and the raw
filename falls back. Parsing is static, AST-backed by Prism, single-file only. Recoverable Ruby
syntax errors produce line-specific warnings on stderr while successfully recovered fields
remain on stdout, including in JSON mode.

## `decorators`

```sh
rails-kit decorators
rails-kit decorators user
rails-kit decorators admin/report
rails-kit decorators Admin::ReportDecorator --json
```

Summarizes a decorator's parent class, included concerns, class-level constants, other
class-level DSL calls (surfaced as macros), and methods. Like `helpers`, each method renders as
its full signature, not a bare name -- a decorator's parameters are the useful part. The reader
targets the Draper gem convention (`delegate_all` on an `ApplicationDecorator`/
`Draper::Decorator` subclass) but degrades gracefully: a custom decorator implementation still
resolves (the `_decorator` suffix is tried first, then the name as given) and reports a useful
summary -- parent class, concerns, methods, and the class-level calls it does make -- just
without the Draper-specific accent. `app/decorators/concerns` is listed like any other decorator
file rather than skipped, since nothing owns that directory the way `app/controllers/concerns` is
owned by the `concerns` command. Parsing is static, AST-backed by Prism, single-file only: a
decorator's own declarations are shown, not ones inherited from a superclass -- `parent_class`
says where to look next. Recoverable Ruby syntax errors produce line-specific warnings on stderr
while successfully recovered fields remain on stdout, including in JSON mode.

## `formers`

```sh
rails-kit formers
rails-kit formers user
rails-kit formers admin/report
rails-kit formers Admin::ReportFormer --json
```

Summarizes a form object's included concerns, class-level constants, attributes, validations,
other class-level DSL calls (surfaced as macros), and methods. Attributes (`attr_accessor`/
`attr_reader`/`attr_writer`) and validations (`validate`/`validates`/`validates_*`) get their own
sections rather than being folded into the macros catch-all, since they're the dominant signal in
a former. A `with_options do ... end` block wrapping validations is expanded inline: the
`with_options` call becomes a group header under `validations`, followed by each nested
validation, indented one level deeper. A `with_options` block holding no validations is treated
like any other class-level call and reported as a macro instead. Like `helpers` and `decorators`,
each method renders as its full signature, not a bare name. Former files follow two filename
conventions across real apps -- `_former.rb` and `_form.rb` -- plus a handful of bare-named files
(mostly under `app/formers/concerns`), so both suffixes are tried before the name as given.
`app/formers/concerns` is listed like any other former file rather than skipped, since nothing
owns that directory the way `app/controllers/concerns` is owned by the `concerns` command.
Parsing is static, AST-backed by Prism, single-file only: a former's own declarations are shown,
not ones inherited from a superclass -- `parent_class` says where to look next. Recoverable Ruby
syntax errors produce line-specific warnings on stderr while successfully recovered fields remain
on stdout, including in JSON mode.

## `presenters`

```sh
rails-kit presenters
rails-kit presenters user
rails-kit presenters users/work/overall
rails-kit presenters Users::Work::OverallPresenter --json
```

Summarizes a presenter's parent class, included concerns, class-level constants, attributes,
other class-level DSL calls (surfaced as macros), and methods. Attributes (`attr_accessor`/
`attr_reader`/`attr_writer`) get their own section rather than being folded into the macros
catch-all -- a presenter's `attr_reader` line says what it wraps, which is the thing you actually
want when you open one. `delegate` and anything else fall through to macros. Like `helpers`,
`decorators`, and `formers`, each method renders as its full signature, not a bare name.
`app/presenters/concerns` is listed like any other presenter file rather than skipped, since
nothing owns that directory the way `app/controllers/concerns` is owned by the `concerns` command.
Parsing is static, AST-backed by Prism, single-file only: a presenter's own declarations are
shown, not ones inherited from a superclass -- `parent_class` says where to look next. Recoverable
Ruby syntax errors produce line-specific warnings on stderr while successfully recovered fields
remain on stdout, including in JSON mode.

## `validators`

```sh
rails-kit validators
rails-kit validators email_format
rails-kit validators admin/access
rails-kit validators Admin::AccessValidator --json
```

Summarizes a validator's parent class, included concerns, class-level constants, other
class-level DSL calls (surfaced as macros), and methods. Three shapes exist in the wild,
distinguished by the header line alone: an `ActiveModel::EachValidator` subclass overriding
`validate_each`, an `ActiveModel::Validator` subclass overriding `validate`, and a plain class (or
module) that includes `ActiveModel::Validations` and drives itself with `validates`. Like
`helpers`, `decorators`, `formers`, and `presenters`, each method renders as its full signature --
for a validator that signature is what tells an `EachValidator` apart from a `Validator` at a
glance, not boilerplate to hide. There is no separate Attributes section like `presenters`: an
`attr_reader`/`attr_accessor`/`attr_writer` line falls through to macros along with `validates`
and everything else, since attribute readers are rare in real validators and don't carry the same
signal a presenter's do. Constants get their own section, and that's usually where the actual
rule lives -- a format regexp or an allowed-value list. `app/validators/concerns` is listed like
any other validator file rather than skipped, since nothing owns that directory the way
`app/controllers/concerns` is owned by the `concerns` command. Validators are not wired into
`related`: validator files are named after the rule they enforce (`phone_validator`,
`email_format_validator`), not after the model they run against, so there is no name-based link
worth drawing. Parsing is static, AST-backed by Prism, single-file only: a validator's own
declarations are shown, not ones inherited from a superclass -- `parent_class` says where to look
next. Recoverable Ruby syntax errors produce line-specific warnings on stderr while successfully
recovered fields remain on stdout, including in JSON mode.

## `completion`

```sh
rails-kit completion bash > /etc/bash_completion.d/rails-kit
rails-kit completion zsh > "${fpath[1]}/_rails-kit"
rails-kit completion fish > ~/.config/fish/completions/rails-kit.fish
```

Completions are dynamic. `model`, `related`, and `skeleton` complete model names;
`schema` completes table names; `locales` completes dotted scopes one level at a
time; `concerns`, `fixtures`, `gem`, `controllers`, `mailers`, `jobs`, `services`,
`datagrids`, `helpers`, `decorators`, `formers`, `presenters`, and `validators` complete their
respective names — all read from the current Rails project. `routes` and other
flag-only commands are unaffected.
