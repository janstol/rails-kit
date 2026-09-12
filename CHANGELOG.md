# Changelog

All notable changes to `rails-kit` will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project aims to follow [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- `rails-kit validators [name]` lists validators, or shows one validator's parent class, included
  concerns, class-level constants, other class-level DSL calls (surfaced as macros, e.g.
  `validates`), and methods. Three shapes exist in the wild, distinguished by the header line
  alone: an `ActiveModel::EachValidator` subclass overriding `validate_each`, an
  `ActiveModel::Validator` subclass overriding `validate`, and a plain class (or module) that
  includes `ActiveModel::Validations` and drives itself with `validates`. Like `helpers`,
  `decorators`, `formers`, and `presenters`, each method renders as its full parameter signature
  rather than a bare name -- for a validator that signature is what tells an `EachValidator`
  apart from a `Validator` at a glance. Unlike `presenters`, there is no separate Attributes
  section: `attr_reader`/`attr_accessor`/`attr_writer` fall through to macros, since attribute
  readers are rare in real validators. Constants get their own section, and that's usually where
  the actual rule lives -- a format regexp or an allowed-value list. `app/validators/concerns` is
  listed like any other validator file rather than skipped, since nothing owns that directory the
  way `app/controllers/concerns` is owned by the `concerns` command. Validators are deliberately
  not wired into `related`: validator files are named after the rule they enforce
  (`phone_validator`, `email_format_validator`), not after the model they run against, so there
  is no name-based link worth drawing. The detail view is AST-backed and single-file only.

### Fixed

- `routes`/`routes --refresh` no longer publish a cache for a run whose route sources changed while `bundle exec rails routes` was booting, and `routes --watch` no longer misses an edit made during its first render. Booting Rails takes seconds, so an edit to `config/routes.rb` or `config/routes/` landing in that window used to get a cache mtime newer than the edit, which `CacheValid` read as fresh -- the stale output then stuck until the file was touched again. `Cache`/`Refresh` now re-fingerprint the route sources after the run and skip the cache write (with a warning on stderr) if they moved, and `--watch`'s baseline fingerprint is now captured by the caller before its own initial render instead of inside `Watch`, so an edit during that render is no longer folded into the baseline and lost.
- `routes --watch` now cancels an in-flight `bundle exec rails routes` subprocess on Ctrl-C/SIGTERM instead of only stopping the poll loop. The watch signal context built in `RunE` was never threaded through: the render closure called `runRoutes` with the root command's context, which nothing cancels, so `Run`/`Refresh`/`Cache` and the `exec.CommandContext` child they start outlived the interrupt. The signal context is now passed explicitly into `runRoutes` and `runRoutesWatch`, so both the initial render and every polled reprint run under it and a signal now tears down the whole render path, subprocess included.
- `locales` now normalizes non-string YAML keys (numbers, booleans, `null`, dates) to their
  string form before merging locale files. yaml.v3 decodes a mapping as `map[interface{}]interface{}`
  the moment even one key isn't a plain string -- an enum keyed by integer (`status: {0:
  draft, 1: published}`), an HTTP status page (`404:`), or a boolean label (`true: Yes`) all
  qualify -- and every consumer in the package only recognized `map[string]interface{}`. That
  mismatch meant a later file's version of such a subtree silently replaced the earlier one
  instead of merging, `locales en.status.404` failed with a misleading "key '404' is not a
  map", the subtree was invisible to scope listing, and it printed as a raw Go map instead of a
  tree. `--json` failed outright for some key types (booleans and `null`; integer keys happened
  to work) with `json: unsupported value: jsontext: object member name must be a string`.

## [0.6.0] - 2026-09-10

### Added

- `rails-kit helpers [name]` lists view helpers, or shows one helper's included concerns,
  class-level constants, and methods. Unlike the other readers, each method renders as its full
  parameter signature rather than a bare name -- a helper is an API surface consumed from views,
  so its parameters are the useful part. The detail view is AST-backed and single-file only.
- `rails-kit decorators [name]` lists decorators, or shows one decorator's parent class, included
  concerns, class-level constants, other class-level DSL calls (surfaced as macros, e.g.
  `delegate_all`), and methods. Like `helpers`, each method renders as its full parameter
  signature rather than a bare name. The reader targets the Draper gem convention but degrades
  gracefully for a custom decorator implementation. `app/decorators/concerns` is listed like any
  other decorator file rather than skipped, since nothing owns that directory the way
  `app/controllers/concerns` is owned by the `concerns` command. The detail view is AST-backed and
  single-file only.
- `rails-kit formers [name]` lists form objects, or shows one former's included concerns,
  class-level constants, attributes, validations, other class-level DSL calls (surfaced as
  macros, e.g. `delegate`), and methods. Attributes and validations get their own sections rather
  than being folded into macros, since they're the dominant signal in a former. A `with_options
  do ... end` block wrapping validations is expanded inline, rather than silently dropped. Like
  `helpers` and `decorators`, each method renders as its full parameter signature rather than a
  bare name. Former files follow two filename conventions across real apps (`_former.rb` and
  `_form.rb`) plus a handful of bare-named files, so both suffixes are tried before the name as
  given. `app/formers/concerns` is listed like any other former file rather than skipped, since
  nothing owns that directory the way `app/controllers/concerns` is owned by the `concerns`
  command. The detail view is AST-backed and single-file only.
- `rails-kit presenters [name]` lists presenters, or shows one presenter's parent class, included
  concerns, class-level constants, attributes, other class-level DSL calls (surfaced as macros,
  e.g. `delegate`), and methods. Attributes (`attr_accessor`/`attr_reader`/`attr_writer`) get
  their own section rather than being folded into macros -- a presenter's `attr_reader` line says
  what it wraps, which is the thing you actually want when you open one. Like `helpers`,
  `decorators`, and `formers`, each method renders as its full parameter signature rather than a
  bare name. `app/presenters/concerns` is listed like any other presenter file rather than
  skipped, since nothing owns that directory the way `app/controllers/concerns` is owned by the
  `concerns` command. `related` reports a `Presenter` category alongside `Decorator`. The detail
  view is AST-backed and single-file only.
- `related` gains five minitest categories -- `System test`, `Helper test`, `Job test`,
  `Mailer test`, and `Service test` -- closing the gap where a minitest-only app got
  meaningfully worse `related` coverage than an RSpec one. Each mirrors its existing RSpec
  counterpart's mechanism exactly: `System test` and `Helper test` walk `test/system` and
  `test/helpers` the same way `System spec`/`Helper spec` walk their `spec/` equivalents; `Job
  test` and `Mailer test` are exact-name lookups under `test/jobs`/`test/mailers`, like their spec
  counterparts; `Service test` uses the same segment-walk-plus-namespace-filter as `Service spec`.
  `test/integration` is deliberately not covered -- its files carry arbitrary flow names
  (`user_flows_test.rb`), not resource names, so a category for it would almost always be empty.
  Five new config fields back the new paths: `test_system_path` (`test/system`),
  `test_helpers_path` (`test/helpers`), `test_jobs_path` (`test/jobs`), `test_mailers_path`
  (`test/mailers`), and `test_services_path` (`test/services`).

### Changed

- `related` now searches a configurable `helpers_path` (default `app/helpers`) and reports a
  `Helper` category alongside the existing `Helper spec` one, so `rails-kit related user` finds
  `app/helpers/users_helper.rb` in addition to its spec.
- Retuned `skeleton`'s internal timeout for the pooled Prism parser introduced in v1.2.0: a hang
  or pathological file now fails within roughly 20 seconds worst case instead of up to 120
  seconds. No change to normal output; only how quickly a genuine hang is reported.
- `mailers`, `jobs`, and `services` now name the resolved absolute directory (rather than the
  configured relative path) in the "path ... not found"/"not a directory" error when their app
  directory is missing or not a directory, matching what `controllers` and `datagrids` already
  did. Only these error messages change; normal output is unaffected.

### Fixed

- `services` no longer discards a module's own methods in favor of a nested implementation class
  when both are present (e.g. `module Foo; def bar; end; class Helper; ...; end; end`). Previously
  the nested class's methods silently replaced the module's own, real API; now the module is
  recognized as the target and the nested class is skipped, not promoted, matching the existing
  rule against nested-class methods leaking into an outer summary. A module whose only content is
  the nested class (plus namespace-level constants) still descends into it unchanged.
- `controllers`, `mailers`, `jobs`, and `datagrids` no longer resolve to a class nested inside an
  unrelated content-bearing module when the file's real class is defined at top level (e.g. a
  leading `module Reportable; def x; end; class Internal; ...; end; end` followed by
  `class ReportsController < ApplicationController; ...; end` now correctly reports
  `ReportsController`, not `Internal`). A module whose only content is a nested class still
  resolves to that class unchanged, so a genuinely empty result never replaces an imperfect one.
  No output change was observed on any real application scanned during development.
- `controllers`, `mailers`, `jobs`, `services`, `datagrids`, and `helpers` no longer silently drop
  a dotted `include` such as `include Rails.application.routes.url_helpers` from their Concerns
  output. Prism parses a dotted chain as a `CallNode`, not a constant, so `IncludedConcern` was
  only ever recognizing a plain constant or constant-path include (`include Trackable`,
  `include ActionController::Cookies`) and silently returning nothing for anything else; it now
  falls back to the include argument's source text. Found while building `decorators`, where the
  gap is common (Draper decorators routinely `include Rails.application.routes.url_helpers` for
  URL helpers). No fixture under any of these readers' existing `testdata/` exercised a dotted
  include, so this fix caused no golden-file changes to any of them.
- `controllers`, `mailers`, `jobs`, `services`, `datagrids`, `helpers`, `decorators`, `formers`,
  `presenters`, and `skeleton` no longer silently drop a `class << self` block: every def, call,
  and constant assignment inside one is now walked exactly as if it appeared directly in the
  surrounding class or module body, with each def reported as a class method just like
  `def self.foo`. Previously the whole block matched no case in the shared walker and vanished
  with no parse diagnostic, no stderr warning, and no exit code -- confirmed as real data loss
  against several real Rails codebases, including large swaths of a Rails core module's own
  public API. `skeleton` also gains a `singleton` marker on each method entry (rendered as a
  `self.`-prefixed name in human output), so `def self.foo` and a `class << self` def are now
  reported consistently and distinguishably from an instance method -- previously both forms were
  silently conflated as plain instance methods. A def, call, or constant nested inside an
  `if`/`unless`/`begin` remains out of scope, unchanged from before.
- `related` now accepts an `app/helpers/…` path as input, resolving it back to its model the same
  way it already resolves `spec/helpers/…_helper_spec.rb`. Previously the mapping only worked in
  the other direction (model to helper); `app/helpers/users_helper.rb` reported "unsupported
  related path" instead of resolving to `user`.
- `related test/helpers/users_helper_test.rb`, and the other four new minitest paths, no longer
  report "unsupported related path" -- they resolve to their owning model the same way their
  `spec/` counterparts already did.
- `NormalizeNameWithPrefixes`'s suffix-strip list checked `_test`/`_spec` before any
  `_helper_test`/`_helper_spec`/`_job_test`/etc. compound, so a bare name like
  `users_helper_test` truncated to `users_helper` instead of `users`. The list is now ordered
  longest-compound-first, matching the invariant it was supposed to have all along.

## [0.5.0] - 2026-08-04

### Added

- `rails-kit controllers [name]` lists controllers, or shows a structural summary of one: filters
  (`before_action`/`after_action`/`around_action` and their `skip_*` variants, with
  `only:`/`except:`/`if:`/`unless:`), `rescue_from` handlers, `helper_method`, `layout`,
  class-level `respond_to`, strong params (`params.require(...).permit(...)`), and public action
  methods. AST-backed by Prism, following the same single-process, no-Rails-boot shape as `model`
  and `concerns`. Single-file only: a controller's own declarations are shown, not ones inherited
  from `ApplicationController` or any other superclass — `parent_class` says where to look next.
  Recoverable Ruby syntax errors produce line-specific warnings on stderr, matching `model`.
- `rails-kit mailers [name]` lists mailers, or shows one mailer's defaults, layout, included
  concerns, regular and inline attachments, public action methods, and `parent_class`. The detail
  view is AST-backed and single-file only; inherited declarations are not expanded.
- `rails-kit jobs [name]` lists jobs, or shows one job's queue, `retry_on` and `discard_on`
  declarations (including retry options and handler blocks), included concerns, public methods,
  and `parent_class`. The detail view is AST-backed and single-file only.
- `rails-kit services [name]` lists service objects without assuming a filename suffix, or shows
  one class- or module-style service's constants, included modules, public instance methods,
  singleton methods, and optional `parent_class`. The detail view is AST-backed and single-file
  only.
- `rails-kit datagrids [name]` lists datagrids, or shows one grid's filters, columns, scope,
  decorator, macros, included concerns, methods, and `parent_class`. It understands the `datagrid`
  gem's DSL while still producing a useful structural summary for custom grid implementations.
  The detail view is AST-backed and single-file only.

### Changed

- Dynamic positional-argument completion now includes controller, mailer, job, service, and
  datagrid names for the five new domain readers.
- `concerns` now parses Ruby through the embedded Prism AST instead of a hand-rolled line scanner with heuristic block-depth tracking. The old depth tracker was demonstrably approximate: an adversarial-fixture audit reproduced all five suspected bug classes against the pre-change binary (a stray `end` not alone on its line, an endless method, `class << self` bodies landing in the wrong list, heredoc bodies scanned as code, and nested-module name truncation), and dogfooding against two real applications turned up two more of the same kind — an off-by-one at the close of a `class_methods` block, and a nested `class ... < StandardError` body's own methods leaking into the concern's method list — plus a distinct pre-`ActiveSupport::Concern` idiom (`def self.included(base); base.class_eval do ... end; end`) that the old scanner handled only by accident, alongside emitting a bogus `self` method for it; `Parse` now recognizes that idiom explicitly. Recoverable Ruby syntax errors return a partial result with line-specific warnings on stderr, matching `model`. Nested keyword modules (`module A; module B; ...; end; end`) now report only the outermost module's name (`A`, not `A::B`) — a deliberate, documented divergence rather than a fix. `concerns <name>` accepts the same ~100-150 ms Prism cold start as `model`/`routes --static`, guarded by the startup-budget test; the no-arg `concerns` list stays on the tight sub-ms budget since it never parses Ruby.
- `model` now parses Ruby through the embedded Prism AST instead of a hand-rolled regex line scanner. Its text and JSON output remain unchanged for valid files, verified byte-for-byte against the pre-change binary across the fixture contract and two real applications; recoverable Ruby syntax errors return partial results with line-specific warnings on stderr. Like `routes --static`, each fresh process pays the accepted ~100-150 ms Prism cold start, guarded by the startup-budget test.
- `routes --static` now parses `config/routes.rb` (and drawn files) through the Prism AST (`go-ruby-prism` v1.2.0) instead of the hand-rolled regex line scanner. The semantic resolution core is reused unchanged; only the front-end is replaced. Text and `--json` output are byte-identical, including every warning message and line number (all 67 `static_test.go` cases and both `routes_static*.golden` files pass unchanged), and the parser was dogfooded against two real applications (Application A and Application B) with byte-identical `routes --static` and `routes --static --json` output versus the pre-change binary. The trade-off is a one-time ~100-150 ms WASM-compile cold start paid on first parse per process (accepted in place of today's sub-ms startup, for an 8-17x per-parse throughput win); subsequent parses in the same process are fast. The `TestStartupBudget` guard now carries a separate, larger budget for `routes --static` to accommodate the accepted cold start, while `schema`/`about` keep the tight budget.
- Bumped `go-ruby-prism` from v1.1.0 to v1.2.0, which fixes the parser-reuse memory bug (reusing one parser across distinct files intermittently trapped with a WASM "out of bounds memory access" in `pm_options_free`) and introduces a pool of WASM instances per parser, with Prism upgraded to 1.9.0. `skeleton` now shares one pooled parser across a batch instead of creating a fresh parser per file, so the cold start is paid once per pool instance rather than once per file. Output is unchanged; large batches are dramatically faster (a 444-file `skeleton app/models` run measured 16.3s before and 0.35s after, ~46x, byte-identical output).

## [0.4.0] - 2026-08-02

### Added

- `windows/amd64` release artifact, backed by a `go test ./...` run on `windows-latest` in CI. Path-bearing output fields (`RelPath`, `Path`, etc.) are now forward-slash normalized on all platforms for consistent `--json` shape across Unix and Windows.
- `rails-kit routes --watch` polls `config/routes.rb` and `config/routes/` mtimes and reprints on change; composes with `--static`, patterns, and `--json`. `--watch-interval` controls the poll interval (default `1s`, minimum `100ms`). Clears the screen on a TTY; appends timestamped output otherwise. A render error is reported but does not stop watching.
- `--color=auto|always|never` persistent flag. `schema` and `model` accent DDL keywords/table names and class names/section labels/macro names respectively; `structure.sql` output is never colored. `auto` (the default) disables color when stdout isn't a terminal; `NO_COLOR` (any non-empty value) disables color even under `--color=always`; `--json` output is never colored regardless of `--color`.
- Dynamic shell completion is now offered for the positional arguments of `model`, `related`, `skeleton`, `schema`, `locales`, `concerns`, `fixtures`, and `gem` — model names, table names, gem and concern names, and locale keys (drilling down one dotted segment at a time for `locales`). Completion honors `--root` and `.rails-kit.yml` and degrades to no candidates outside a Rails root. No caching layer; every source is sub-millisecond.

### Changed

- **BREAKING:** `--json` output is now versioned and uniformly shaped. Every successful invocation wraps its payload in an envelope — `{ "schema_version": 1, "command": "...", "data": {...} }` — instead of writing the payload directly. Every failing `--json` invocation now writes a JSON error object to stderr (`{ "schema_version": 1, "command": "...", "error": { "code": "...", "message": "..." } }`) with exit code 1, instead of plain text. `data` is now always a JSON object; commands that previously returned a bare array or switched between an object and an array based on argument count now return a fixed object shape with the array under a named key. For example:

  ```jsonc
  // schema (list mode), before: ["users", "orders"]
  // after:
  { "schema_version": 1, "command": "schema", "data": { "tables": [ { "name": "users" }, { "name": "orders" } ] } }

  // schema users (extract mode), before: { "users": "create_table ..." }
  // after: same "tables" key and shape as list mode, just with "definition" populated
  { "schema_version": 1, "command": "schema", "data": { "tables": [ { "name": "users", "definition": "create_table \"users\" ..." } ] } }

  // skeleton app/models/user.rb, before: one bare object; skeleton 'app/**/*.rb', before: an array
  // after: always an array under "files", regardless of count
  { "schema_version": 1, "command": "skeleton", "data": { "files": [ { "path": "...", "rel_path": "app/models/user.rb", "...": "..." } ] } }
  ```

  `routes`, `fixtures` (list mode), `locales` (list mode), and `gem` (list mode) get the same array-under-a-named-key treatment (`routes`, `files`, `scopes`, `gems` respectively). See [`docs/json.md`](docs/json.md) for the full contract, the error-code vocabulary, and the versioning policy.
- `skeleton` now parses via the embedded Ruby Prism parser running in-process on the pure-Go `wazero` WASM runtime, instead of shelling out to a `ruby` subprocess. This removes rails-kit's hard Ruby dependency for that command. `routes` (non-static) and `about --runtime` still shell out where a booted Rails is genuinely needed. Trade-off: the binary grows ~6 MB (to ~10 MB) from the embedded Prism WASM module.
- `skeleton` over a directory now parses files concurrently, bounded by CPU count, instead of serially. Each file still gets its own Prism parser instance (required since go-ruby-prism's parser isn't safe to reuse), but the per-file cold start now overlaps across files instead of stacking up. Measured ~4x faster on batches of 8–500 files; output is byte-identical. `locales` similarly parallelizes reading and parsing locale files, with a smaller win since that work isn't cold-start dominated.

## [0.3.0] - 2026-07-25

### Added

- `about` summarizes application, environment, dependency, and database metadata without booting Rails. `--runtime` uses a bounded Rails runner for active values and gracefully retains the static report when the application cannot boot.
- `routes --static` parses `config/routes.rb` directly in pure Go, without booting Rails or shelling out to bundler. It understands `resources`/`resource`, `namespace`, `root`, and verb routes, including nesting and `only:`/`except:`. It's an approximation — no internal engine route expansion, callable route concerns, full constraint evaluation, or gem-drawn routes (Devise, etc.) — intended as a fast, offline fallback for when `bundle exec rails routes` can't boot or isn't worth the wait. When the normal `routes` command fails, the error now hints at `--static`.
- `skeleton` accepts multiple model names, Ruby paths, and glob patterns, validates and deduplicates them, and parses the resulting files in one Prism process. Single-file JSON remains an object; multi-file JSON is an ordered array.
- `skeleton` accepts recursive directory inputs with repeatable root-relative `--exclude` patterns, including `**` matching. Directory batches are capped at 500 unique Ruby files.
- `skill install|uninstall` supports Claude Code, Codex, or both through `--target claude|codex|all`. Codex installations use `.agents/skills` and include `agents/openai.yaml` metadata.

### Changed

- `routes --static` now uses Rails-style `:id` member parameters; supports scalar, array, `%i[...]`, and `%w[...]` action filters (including empty filters); models resource `path`, `controller`, `as`, and `param` options; infers symbolic verb routes, controllers, actions, and helper names; and understands module scopes. Constraints remain approximate and produce line-specific warnings without changing JSON stdout.
- `routes --static` recursively expands safe `draw` declarations from `config/routes`, preserves surrounding route context and declaration order, detects cycles, and reports warnings with the originating file and line.
- `routes --static` registers and expands static block-defined route concerns across drawn files, including multiple-name and inline resource forms, while warning on missing, cyclic, callable, or parameterized concerns.
- `routes --static` models literal string redirects, including hash-rocket declarations and optional numeric status codes, using Rails inspector-style endpoint text. Dynamic, callable, block, and option-hash redirects remain unsupported and produce source-specific warnings.
- `routes --static` models single-line `match` routes with static `via:` symbols, arrays, `%i[...]`, `%w[...]`, or `:all`, including Rails-style combined verb output. Missing, dynamic, or unsupported methods produce source-specific warnings.
- `routes --static` combines comma-continued verb and `match` declarations before parsing, preserving explicit endpoints, helpers, methods, redirects, constraints, and source-line warnings.
- `routes --static` models parenthesized namespaces and single-declaration inline namespace, member, and collection blocks. Executable or multi-statement inline blocks are skipped with source-specific warnings.
- Generated `resources` and `resource` routes now display each shared helper prefix only on its first enabled route, matching Rails inspector table and JSON output.
- `routes --static` models inline regex constraints for named path parameters and appends them to endpoint text in Rails inspector style. Unsupported or partially modeled constraints retain their routes and produce source-specific warnings.
- `routes --static` represents static constant engine and Rack mount points using hash-rocket or `at:` syntax, including scoped paths and Rails-style inferred helpers. Internal engine routes and dynamic mount declarations remain unexpanded.
- `routes --static` represents constant-qualified, zero-argument mount receivers without inventing helpers. Postfix-conditional mounts are retained with warnings because their runtime conditions are not evaluated.
- `routes --static` composes positional or option-based static scope paths, controller modules/defaults, and helper prefixes across nested routes, resources, concerns, and drawn files. Dynamic scopes are skipped with source-specific warnings.
- `routes --static` models static `controller` blocks as inherited default endpoint context across scopes, resources, concerns, and drawn files. Dynamic controller blocks are skipped with source-specific warnings.
- Routes cache and metadata files are written through same-directory temporary files and atomically renamed, preventing interrupted writes from publishing partial cache contents.
- Skill installation and removal are now global by default. Use `--local` for repository-scoped installation; the former `--global` flag remains as a deprecated compatibility alias.

## [0.2.0] - 2026-04-26

### Added

- `skeleton` command for compact Ruby AST summaries via Prism. It accepts a model name or Rails-root-relative Ruby path and reports class/module structure, constants, includes, macros, methods, and line numbers without loading Rails.
- `skeleton` retries Ruby through the user's interactive shell when the direct Ruby on `PATH` cannot load Prism, avoiding accidental fallback to system Ruby in version-manager projects.
- `schema` command now supports `db/structure.sql` (PostgreSQL DDL generated by `pg_dump`). Both formats are auto-detected by file extension. If `schema.rb` is not found, `structure.sql` is tried automatically. Rails internal tables (`schema_migrations`, `ar_internal_metadata`) are excluded from output.
- `concerns` command for listing and inspecting model/controller concerns. Lists all concerns from `app/models/concerns/` and `app/controllers/concerns/` grouped by type. With a concern name, shows the module name, methods, class methods, and whether `included do` or `class_methods do` blocks are present. Supports `--json` output and qualified names (`model/name`, `controller/name`) for disambiguation. Concern directories are configurable via `model_concerns_path` and `controller_concerns_path` in `.rails-kit.yml`.
- `gem` command for inspecting gems from `Gemfile.lock` — lists all gems with versions, or shows detail (source, URL, git metadata, dependencies) for a named gem. Configurable via `gemfile_lock_path` in `.rails-kit.yml`.
- `related` command now discovers job files (`app/jobs/*_job.rb`) and mailer files (`app/mailers/*_mailer.rb`). Configurable via `jobs_path` and `mailers_path` in `.rails-kit.yml`. Reverse lookup from job/mailer file paths is also supported.
- `related` command now discovers spec files in `spec/requests`, `spec/system`, `spec/helpers`, `spec/jobs`, `spec/mailers`, and `spec/services`. All six paths are configurable and support reverse lookup from file path inputs.

## [0.1.0] - 2026-03-20

### Added

- Initial CLI release of `rails-kit`.
- `schema` command for extracting table definitions from `db/schema.rb`.
- `routes` command for cached and filtered `rails routes` output.
- `related` command for finding files related to a model.
- `fixtures` command for summarizing fixture entries.
- `locales` command for browsing and extracting locale scopes and values.
- `model` command for compact model structure summaries.
- `completion` command for generating shell completion scripts.
- `version` command with build metadata output.
- Bundled `skill install|uninstall` support for the included Claude Code skill.
- GitHub release packaging for macOS and Linux on `amd64` and `arm64`.
- Completion subcommands for `bash`, `zsh`, and `fish`.

[Unreleased]: https://github.com/janstol/rails-kit/compare/v0.6.0...HEAD
[0.6.0]: https://github.com/janstol/rails-kit/compare/v0.5.0...v0.6.0
[0.5.0]: https://github.com/janstol/rails-kit/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/janstol/rails-kit/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/janstol/rails-kit/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/janstol/rails-kit/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/janstol/rails-kit/releases/tag/v0.1.0
