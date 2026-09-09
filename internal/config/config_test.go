package config_test

import (
	"os"
	"path/filepath"
	"reflect"
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
	if !reflect.DeepEqual(cfg, config.Defaults()) {
		t.Errorf("Load(emptyDir) = %#v, want %#v", cfg, config.Defaults())
	}
}

func TestLoadExplicitEmptyStringFallsBackToDefault(t *testing.T) {
	dir := t.TempDir()
	content := "schema_path: \"\"\n"
	if err := os.WriteFile(filepath.Join(dir, ".rails-kit.yml"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.SchemaPath != "db/schema.rb" {
		t.Errorf("SchemaPath = %q, want db/schema.rb", cfg.SchemaPath)
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
	if cfg.HelpersPath != "app/helpers" {
		t.Errorf("HelpersPath = %q", cfg.HelpersPath)
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

func TestResolveSchemaPath(t *testing.T) {
	t.Run("schema.rb exists", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.MkdirAll(filepath.Join(dir, "db"), 0755); err != nil {
			t.Fatal(err)
		}
		schemaRb := filepath.Join(dir, "db", "schema.rb")
		if err := os.WriteFile(schemaRb, []byte(""), 0644); err != nil {
			t.Fatal(err)
		}
		cfg, _ := config.Load(dir)
		got := config.ResolveSchemaPath(dir, cfg)
		if got != schemaRb {
			t.Errorf("got %q, want %q", got, schemaRb)
		}
	})

	t.Run("only structure.sql exists, falls back", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.MkdirAll(filepath.Join(dir, "db"), 0755); err != nil {
			t.Fatal(err)
		}
		structureSQL := filepath.Join(dir, "db", "structure.sql")
		if err := os.WriteFile(structureSQL, []byte(""), 0644); err != nil {
			t.Fatal(err)
		}
		cfg, _ := config.Load(dir)
		got := config.ResolveSchemaPath(dir, cfg)
		if got != structureSQL {
			t.Errorf("got %q, want %q", got, structureSQL)
		}
	})

	t.Run("neither exists, returns primary", func(t *testing.T) {
		dir := t.TempDir()
		cfg, _ := config.Load(dir)
		got := config.ResolveSchemaPath(dir, cfg)
		want := filepath.Join(dir, "db", "schema.rb")
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("explicit structure.sql in config", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.MkdirAll(filepath.Join(dir, "db"), 0755); err != nil {
			t.Fatal(err)
		}
		structureSQL := filepath.Join(dir, "db", "structure.sql")
		if err := os.WriteFile(structureSQL, []byte(""), 0644); err != nil {
			t.Fatal(err)
		}
		cfg := config.Defaults()
		cfg.SchemaPath = "db/structure.sql"
		got := config.ResolveSchemaPath(dir, cfg)
		if got != structureSQL {
			t.Errorf("got %q, want %q", got, structureSQL)
		}
	})
}

func TestResolvePath(t *testing.T) {
	root := t.TempDir()

	t.Run("relative path", func(t *testing.T) {
		got := config.ResolvePath(root, "db/schema.rb")
		want := filepath.Join(root, "db", "schema.rb")
		if got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	})

	t.Run("absolute path", func(t *testing.T) {
		// filepath.Join(t.TempDir(), ...) is a genuine absolute path on
		// every platform, unlike a hand-rolled filepath.Separator-prefixed
		// string, which is rooted but not absolute on Windows (no drive
		// letter), so filepath.IsAbs would reject it there.
		abs := filepath.Join(t.TempDir(), "shared", "schema.rb")
		got := config.ResolvePath(root, abs)
		if got != abs {
			t.Fatalf("got %q, want %q", got, abs)
		}
	})
}
