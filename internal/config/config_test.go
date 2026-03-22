package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/janstol/rails-kit/internal/config"
)

func TestLoadDefaults(t *testing.T) {
	dir := t.TempDir()
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.SchemaPath != "db/schema.rb" {
		t.Errorf("SchemaPath = %q, want db/schema.rb", cfg.SchemaPath)
	}
	if cfg.FixturesPath != "test/fixtures" {
		t.Errorf("FixturesPath = %q, want test/fixtures", cfg.FixturesPath)
	}
	if cfg.LocalesPath != "config/locales" {
		t.Errorf("LocalesPath = %q, want config/locales", cfg.LocalesPath)
	}
	if cfg.ModelsPath != "app/models" {
		t.Errorf("ModelsPath = %q, want app/models", cfg.ModelsPath)
	}
	if cfg.ControllersPath != "app/controllers" {
		t.Errorf("ControllersPath = %q, want app/controllers", cfg.ControllersPath)
	}
	if cfg.ViewsPath != "app/views" {
		t.Errorf("ViewsPath = %q, want app/views", cfg.ViewsPath)
	}
	if cfg.ServicesPath != "app/services" {
		t.Errorf("ServicesPath = %q, want app/services", cfg.ServicesPath)
	}
	if cfg.JobsPath != "app/jobs" {
		t.Errorf("JobsPath = %q, want app/jobs", cfg.JobsPath)
	}
	if cfg.MailersPath != "app/mailers" {
		t.Errorf("MailersPath = %q, want app/mailers", cfg.MailersPath)
	}
	if cfg.SpecFixturesPath != "spec/fixtures" {
		t.Errorf("SpecFixturesPath = %q, want spec/fixtures", cfg.SpecFixturesPath)
	}
	if cfg.SpecRequestsPath != "spec/requests" {
		t.Errorf("SpecRequestsPath = %q, want spec/requests", cfg.SpecRequestsPath)
	}
	if cfg.SpecSystemPath != "spec/system" {
		t.Errorf("SpecSystemPath = %q, want spec/system", cfg.SpecSystemPath)
	}
	if cfg.SpecHelpersPath != "spec/helpers" {
		t.Errorf("SpecHelpersPath = %q, want spec/helpers", cfg.SpecHelpersPath)
	}
	if cfg.SpecJobsPath != "spec/jobs" {
		t.Errorf("SpecJobsPath = %q, want spec/jobs", cfg.SpecJobsPath)
	}
	if cfg.SpecMailersPath != "spec/mailers" {
		t.Errorf("SpecMailersPath = %q, want spec/mailers", cfg.SpecMailersPath)
	}
	if cfg.SpecServicesPath != "spec/services" {
		t.Errorf("SpecServicesPath = %q, want spec/services", cfg.SpecServicesPath)
	}
}

func TestLoadFromFile(t *testing.T) {
	dir := t.TempDir()
	content := `schema_path: db/custom_schema.rb
services_path: app/workflows
plurals:
  curriculum: curricula
`
	if err := os.WriteFile(filepath.Join(dir, ".rails-kit.yml"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.SchemaPath != "db/custom_schema.rb" {
		t.Errorf("SchemaPath = %q", cfg.SchemaPath)
	}
	if cfg.Plurals["curriculum"] != "curricula" {
		t.Errorf("plurals[curriculum] = %q", cfg.Plurals["curriculum"])
	}
	if cfg.ServicesPath != "app/workflows" {
		t.Errorf("ServicesPath = %q", cfg.ServicesPath)
	}
	// Unset fields use defaults
	if cfg.FixturesPath != "test/fixtures" {
		t.Errorf("FixturesPath = %q", cfg.FixturesPath)
	}
}

func TestLoadEmptyFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".rails-kit.yml"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("empty config file should not error, got: %v", err)
	}
	if cfg.SchemaPath != "db/schema.rb" {
		t.Errorf("SchemaPath = %q, want db/schema.rb", cfg.SchemaPath)
	}
}

func TestLoadCommentOnlyFile(t *testing.T) {
	dir := t.TempDir()
	content := "# This is just a comment\n# schema_path: db/schema.rb\n"
	if err := os.WriteFile(filepath.Join(dir, ".rails-kit.yml"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("comment-only config file should not error, got: %v", err)
	}
	if cfg.SchemaPath != "db/schema.rb" {
		t.Errorf("SchemaPath = %q, want db/schema.rb", cfg.SchemaPath)
	}
}

func TestLoadRejectsUnknownFields(t *testing.T) {
	dir := t.TempDir()
	content := "fixture_path: test/fixtures\n"
	if err := os.WriteFile(filepath.Join(dir, ".rails-kit.yml"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := config.Load(dir)
	if err == nil {
		t.Fatal("expected error")
	}
	if got := err.Error(); got == "" || !containsAll(got, "field", "fixture_path") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func containsAll(s string, want ...string) bool {
	for _, part := range want {
		if !strings.Contains(s, part) {
			return false
		}
	}
	return true
}

func TestResolvePath(t *testing.T) {
	root := filepath.Join(string(filepath.Separator), "tmp", "rails-app")

	t.Run("relative path", func(t *testing.T) {
		got := config.ResolvePath(root, "db/schema.rb")
		want := filepath.Join(root, "db", "schema.rb")
		if got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	})

	t.Run("absolute path", func(t *testing.T) {
		abs := filepath.Join(string(filepath.Separator), "tmp", "shared", "schema.rb")
		got := config.ResolvePath(root, abs)
		if got != abs {
			t.Fatalf("got %q, want %q", got, abs)
		}
	})
}
