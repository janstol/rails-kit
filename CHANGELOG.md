# Changelog

All notable changes to `rails-kit` will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project aims to follow [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed

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

[Unreleased]: https://github.com/janstol/rails-kit/compare/v0.4.0...HEAD
[0.4.0]: https://github.com/janstol/rails-kit/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/janstol/rails-kit/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/janstol/rails-kit/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/janstol/rails-kit/releases/tag/v0.1.0
