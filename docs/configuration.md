# Configuration

rails-kit works with conventional Rails paths without configuration. Add an optional `.rails-kit.yml` at the Rails root for non-standard layouts.

```yaml
schema_path: db/schema.rb
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

plurals:
  curriculum: curricula
  syllabus: syllabi
```

All fields are optional. Relative paths resolve from the Rails root; absolute paths are supported. For SQL-format projects, set `schema_path: db/structure.sql`, although rails-kit also tries the alternate conventional schema format when the configured file is absent.

Additional `plurals` are merged with the built-in pluralization rules.

Unknown keys are rejected so configuration typos fail clearly instead of silently using defaults.
