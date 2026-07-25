package about

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/janstol/rails-kit/internal/config"
)

func TestInspectStaticProject(t *testing.T) {
	root := makeProject(t)
	t.Setenv("RAILS_ENV", "test")

	report := Inspect(root, config.Defaults())

	if report.Application != "ExampleApp" {
		t.Errorf("application = %q, want ExampleApp", report.Application)
	}
	if report.Environment != "test" || report.Source != "static" {
		t.Errorf("environment/source = %q/%q", report.Environment, report.Source)
	}
	if report.Versions.Rails != "7.2.2" || report.Versions.Rack != "3.1.8" {
		t.Errorf("versions = %+v", report.Versions)
	}
	if report.Versions.Ruby != "3.3.6p108" || report.Versions.Bundler != "2.6.2" {
		t.Errorf("versions = %+v", report.Versions)
	}
	if got := strings.Join(report.Database.Adapters, ","); got != "postgresql,sqlite3" {
		t.Errorf("adapters = %q", got)
	}
	if report.Database.SchemaFormat != "ruby" {
		t.Errorf("schema format = %q, want ruby", report.Database.SchemaFormat)
	}
	if len(report.Warnings) != 0 {
		t.Errorf("warnings = %v, want none", report.Warnings)
	}
}

func TestInspectUsesRubyFallbackAndPartialWarnings(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "config/application.rb", "module ExampleApp\n  class Application < Rails::Application\n  end\nend\n")
	writeFile(t, root, ".ruby-version", "ruby-3.2.4\n")

	report := Inspect(root, config.Defaults())

	if report.Versions.Ruby != "3.2.4" {
		t.Errorf("ruby = %q, want 3.2.4", report.Versions.Ruby)
	}
	if len(report.Warnings) != 3 {
		t.Errorf("warnings = %v, want dependency, database, and schema warnings", report.Warnings)
	}
}

func TestEnvironmentFallsBackToRackThenDevelopment(t *testing.T) {
	t.Setenv("RAILS_ENV", "")
	t.Setenv("RACK_ENV", "staging")
	if got := environment(); got != "staging" {
		t.Errorf("environment = %q, want staging", got)
	}
	t.Setenv("RACK_ENV", "")
	if got := environment(); got != "development" {
		t.Errorf("environment = %q, want development", got)
	}
}

func TestRunnerIgnoresBootNoise(t *testing.T) {
	root := t.TempDir()
	bundle := filepath.Join(root, "bundle")
	script := "#!/bin/sh\nprintf '%s\\n' 'initializer noise' '" + sentinel + `{"rails":"8.0.1","ruby":"ruby 3.4.1","rubygems":"3.6.2","rack":"3.1.8","bundler":"2.6.2","environment":"test","database_adapter":"postgresql"}'` + "\n"
	if err := os.WriteFile(bundle, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	info, err := (Runner{Bundle: bundle}).Inspect(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if info.Rails != "8.0.1" || info.DatabaseAdapter != "postgresql" {
		t.Errorf("runtime info = %+v", info)
	}
}

func TestRunnerTimesOut(t *testing.T) {
	root := t.TempDir()
	bundle := filepath.Join(root, "bundle")
	if err := os.WriteFile(bundle, []byte("#!/bin/sh\nsleep 2\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := (Runner{Bundle: bundle}).Inspect(ctx, root); err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("error = %v, want timeout", err)
	}
}

func TestEnrichOverwritesRuntimeValues(t *testing.T) {
	report := Report{Source: "static", Environment: "development", Versions: Versions{Rails: "7"}, Database: Database{Adapters: []string{"sqlite3"}}}
	report = Enrich(report, RuntimeInfo{Rails: "8", RubyGems: "3.6", Environment: "production", DatabaseAdapter: "postgresql"})
	if report.Source != "runtime" || report.Versions.Rails != "8" || report.Versions.RubyGems != "3.6" {
		t.Errorf("report = %+v", report)
	}
	if report.Environment != "production" || strings.Join(report.Database.Adapters, ",") != "postgresql" {
		t.Errorf("report = %+v", report)
	}
}

func TestFormatOmitsUnavailableFieldsAndShowsWarnings(t *testing.T) {
	output := Format(Report{Root: "/example", Environment: "development", Source: "static", Warnings: []string{"metadata unavailable"}})
	if strings.Contains(output, "Rails version") {
		t.Errorf("output contains unavailable version:\n%s", output)
	}
	if !strings.Contains(output, "Warnings:\n- metadata unavailable") {
		t.Errorf("output missing warning:\n%s", output)
	}
}

func makeProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeFile(t, root, "config/application.rb", "module ExampleApp\n  class Application < Rails::Application\n  end\nend\n")
	writeFile(t, root, "config/database.yml", "default: &default\n  adapter: postgresql\nsecondary:\n  adapter: sqlite3\nignored:\n  adapter: <%= ENV.fetch(\"DB_ADAPTER\") %>\n")
	writeFile(t, root, "db/schema.rb", "ActiveRecord::Schema[7.2].define do\nend\n")
	writeFile(t, root, "Gemfile.lock", "GEM\n  remote: https://rubygems.org/\n  specs:\n    rack (3.1.8)\n    rails (7.2.2)\n\nRUBY VERSION\n   ruby 3.3.6p108\n\nBUNDLED WITH\n   2.6.2\n")
	return root
}

func writeFile(t *testing.T, root, relative, content string) {
	t.Helper()
	path := filepath.Join(root, relative)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
