# Changelog

All notable changes to `rails-kit` will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project aims to follow [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

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

[0.1.0]: https://github.com/janstol/rails-kit/releases/tag/v0.1.0
