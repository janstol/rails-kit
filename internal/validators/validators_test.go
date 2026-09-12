package validators_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/janstol/rails-kit/internal/term"
	"github.com/janstol/rails-kit/internal/validators"
)

const testdataRoot = "../../testdata"

func TestParse_EmailFormatValidator(t *testing.T) {
	path := testdataRoot + "/app/validators/email_format_validator.rb"
	s, err := validators.Parse(path, testdataRoot, "app/validators")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if s.ClassName != "EmailFormatValidator" {
		t.Errorf("ClassName = %q, want EmailFormatValidator", s.ClassName)
	}
	if s.Kind != "class" {
		t.Errorf("Kind = %q, want class", s.Kind)
	}
	if s.ParentClass != "ActiveModel::EachValidator" {
		t.Errorf("ParentClass = %q, want ActiveModel::EachValidator", s.ParentClass)
	}

	if want := []string{`  FORMAT = /\A[^@\s]+@[^@\s]+\z/`}; !reflect.DeepEqual(s.Constants, want) {
		t.Errorf("Constants = %#v, want %#v", s.Constants, want)
	}

	wantMethods := []string{
		"  validate_each(record, attribute, value)",
		"  default_message",
	}
	if !reflect.DeepEqual(s.Methods, wantMethods) {
		t.Errorf("Methods = %#v, want %#v", s.Methods, wantMethods)
	}
	// The private helper must be excluded.
	for _, m := range s.Methods {
		if strings.Contains(m, "normalized") {
			t.Errorf("private method leaked into Methods: %q", m)
		}
	}
}

func TestParse_RecordStateValidator(t *testing.T) {
	path := testdataRoot + "/app/validators/record_state_validator.rb"
	s, err := validators.Parse(path, testdataRoot, "app/validators")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.ClassName != "RecordStateValidator" {
		t.Errorf("ClassName = %q, want RecordStateValidator", s.ClassName)
	}
	if s.Kind != "class" {
		t.Errorf("Kind = %q, want class", s.Kind)
	}
	if s.ParentClass != "ActiveModel::Validator" {
		t.Errorf("ParentClass = %q, want ActiveModel::Validator", s.ParentClass)
	}
	if want := []string{"  validate(record)"}; !reflect.DeepEqual(s.Methods, want) {
		t.Errorf("Methods = %#v, want %#v", s.Methods, want)
	}
}

func TestParse_NamespacedAdminAccess(t *testing.T) {
	path := testdataRoot + "/app/validators/admin/access_validator.rb"
	s, err := validators.Parse(path, testdataRoot, "app/validators")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.ClassName != "Admin::AccessValidator" {
		t.Errorf("ClassName = %q, want Admin::AccessValidator", s.ClassName)
	}
	if s.Kind != "class" {
		t.Errorf("Kind = %q, want class", s.Kind)
	}
	if s.ParentClass != "" {
		t.Errorf("ParentClass = %q, want empty", s.ParentClass)
	}

	want := []string{"  ActiveModel::Validations"}
	if !reflect.DeepEqual(s.Concerns, want) {
		t.Errorf("Concerns = %#v, want %#v", s.Concerns, want)
	}

	// attr_reader has no dedicated section here (unlike presenters) --
	// it falls through to Macros along with the two validates calls.
	wantMacros := []string{
		"  attr_reader :role",
		"  validates :role, presence: true",
		"  validates :role, inclusion: { in: %w[admin superadmin] }",
	}
	if !reflect.DeepEqual(s.Macros, wantMacros) {
		t.Errorf("Macros = %#v, want %#v", s.Macros, wantMacros)
	}
	if want := []string{"  initialize(role)"}; !reflect.DeepEqual(s.Methods, want) {
		t.Errorf("Methods = %#v, want %#v", s.Methods, want)
	}
}

func TestParse_ConcernsModule(t *testing.T) {
	// Nothing owns app/validators/concerns the way app/controllers/concerns
	// is owned by the concerns command, so it must resolve and parse cleanly
	// as a module like any other validator file.
	path := testdataRoot + "/app/validators/concerns/rule_helpers.rb"
	s, err := validators.Parse(path, testdataRoot, "app/validators")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.ClassName != "Concerns::RuleHelpers" {
		t.Errorf("ClassName = %q, want Concerns::RuleHelpers", s.ClassName)
	}
	if s.Kind != "module" {
		t.Errorf("Kind = %q, want module", s.Kind)
	}
	want := []string{"  blank_or_matches?(value, format)"}
	if !reflect.DeepEqual(s.Methods, want) {
		t.Errorf("Methods = %#v, want %#v", s.Methods, want)
	}
}

func TestParse_DottedIncludeNotDropped(t *testing.T) {
	content := strings.Join([]string{
		"class Custom < ActiveModel::EachValidator",
		"  include Rails.application.routes.url_helpers",
		"end",
		"",
	}, "\n")
	s := parseTempValidator(t, "custom_validator.rb", content)

	want := []string{"  Rails.application.routes.url_helpers"}
	if !reflect.DeepEqual(s.Concerns, want) {
		t.Fatalf("Concerns = %#v, want %#v (dotted include must not be silently dropped)", s.Concerns, want)
	}
}

func TestParse_OnlyOutermostClass(t *testing.T) {
	content := strings.Join([]string{
		"class Broken < ActiveModel::EachValidator",
		"  def validate_each(record, attribute, value)",
		"  end",
		"",
		"  class InlineError < StandardError",
		"    def message",
		"      \"nope\"",
		"    end",
		"  end",
		"end",
		"",
	}, "\n")
	s := parseTempValidator(t, "broken.rb", content)

	if want := []string{"  validate_each(record, attribute, value)"}; !reflect.DeepEqual(s.Methods, want) {
		t.Fatalf("Methods leaked nested class methods: %#v", s.Methods)
	}
}

func TestParse_ReturnsPartialSummaryWithParseDiagnostics(t *testing.T) {
	s := parseTempValidator(t, "broken_validator.rb", "class Broken < ActiveModel::EachValidator\n  DEFAULT = 1\n  def validate_each(\nend\n")

	if want := []string{"  DEFAULT = 1"}; !reflect.DeepEqual(s.Constants, want) {
		t.Errorf("Constants = %#v, want %#v", s.Constants, want)
	}
	if len(s.ParseErrors) == 0 {
		t.Fatal("expected a Prism parse diagnostic")
	}
	if s.ParseErrors[0].Line < 1 || s.ParseErrors[0].Message == "" {
		t.Fatalf("invalid parse diagnostic: %#v", s.ParseErrors[0])
	}
}

func TestResolve(t *testing.T) {
	t.Run("by resource name", func(t *testing.T) {
		path, err := validators.Resolve(testdataRoot, "app/validators", "email_format")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.HasSuffix(filepath.ToSlash(path), "app/validators/email_format_validator.rb") {
			t.Errorf("unexpected path: %s", path)
		}
	})

	t.Run("by basename", func(t *testing.T) {
		path, err := validators.Resolve(testdataRoot, "app/validators", "email_format_validator")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.HasSuffix(filepath.ToSlash(path), "app/validators/email_format_validator.rb") {
			t.Errorf("unexpected path: %s", path)
		}
	})

	t.Run("by CamelCase class name", func(t *testing.T) {
		path, err := validators.Resolve(testdataRoot, "app/validators", "EmailFormatValidator")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.HasSuffix(filepath.ToSlash(path), "app/validators/email_format_validator.rb") {
			t.Errorf("unexpected path: %s", path)
		}
	})

	t.Run("namespaced by path", func(t *testing.T) {
		path, err := validators.Resolve(testdataRoot, "app/validators", "admin/access")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.HasSuffix(filepath.ToSlash(path), "app/validators/admin/access_validator.rb") {
			t.Errorf("unexpected path: %s", path)
		}
	})

	t.Run("namespaced CamelCase class name", func(t *testing.T) {
		path, err := validators.Resolve(testdataRoot, "app/validators", "Admin::AccessValidator")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.HasSuffix(filepath.ToSlash(path), "app/validators/admin/access_validator.rb") {
			t.Errorf("unexpected path: %s", path)
		}
	})

	t.Run("by .rb path", func(t *testing.T) {
		path, err := validators.Resolve(testdataRoot, "app/validators", "app/validators/email_format_validator.rb")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.HasSuffix(filepath.ToSlash(path), "app/validators/email_format_validator.rb") {
			t.Errorf("unexpected path: %s", path)
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, err := validators.Resolve(testdataRoot, "app/validators", "nonexistent")
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("ambiguous basename", func(t *testing.T) {
		extra := filepath.Join(testdataRoot, "app/validators", "billing", "access_validator.rb")
		if err := os.MkdirAll(filepath.Dir(extra), 0o755); err != nil {
			t.Fatalf("setup: %v", err)
		}
		if err := os.WriteFile(extra, []byte("class Billing::AccessValidator < ActiveModel::EachValidator\nend\n"), 0o644); err != nil {
			t.Fatalf("setup: %v", err)
		}
		t.Cleanup(func() {
			_ = os.Remove(extra)
			_ = os.Remove(filepath.Dir(extra))
		})

		_, err := validators.Resolve(testdataRoot, "app/validators", "access_validator")
		if err == nil {
			t.Fatal("expected error for ambiguous validator name")
		}
		if !validators.IsAmbiguousError(err) {
			t.Errorf("expected ambiguous error, got: %v", err)
		}
	})

	t.Run("outside validators dir", func(t *testing.T) {
		dir := t.TempDir()
		f := filepath.Join(dir, "elsewhere.rb")
		if err := os.WriteFile(f, []byte("class Elsewhere\nend\n"), 0o644); err != nil {
			t.Fatalf("setup: %v", err)
		}
		_, err := validators.Resolve(testdataRoot, "app/validators", f)
		if err == nil {
			t.Fatal("expected error for file outside validators directory")
		}
	})
}

func TestListNames(t *testing.T) {
	names, err := validators.ListNames(testdataRoot, "app/validators")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"admin/access", "concerns/rule_helpers", "email_format", "record_state"}
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("got %v, want %v", names, want)
	}
}

func TestListNames_MissingValidatorsDir(t *testing.T) {
	dir := t.TempDir()
	names, err := validators.ListNames(dir, "app/validators")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if names != nil {
		t.Fatalf("expected nil names, got %v", names)
	}
}

func TestFormat(t *testing.T) {
	t.Run("plain", func(t *testing.T) {
		path := testdataRoot + "/app/validators/record_state_validator.rb"
		s, err := validators.Parse(path, testdataRoot, "app/validators")
		if err != nil {
			t.Fatal(err)
		}
		out := validators.Format(s, term.Styler{})
		if !strings.Contains(out, "RecordStateValidator < ActiveModel::Validator (") {
			t.Error("missing header in output")
		}
		if !strings.Contains(out, "Constants:") {
			t.Error("missing Constants section")
		}
		if !strings.Contains(out, "ALLOWED_STATES") {
			t.Error("missing ALLOWED_STATES constant in output")
		}
		if !strings.Contains(out, "Methods:") {
			t.Error("missing Methods section")
		}
	})

	t.Run("module has no parent line", func(t *testing.T) {
		t.Setenv("NO_COLOR", "")
		path := testdataRoot + "/app/validators/concerns/rule_helpers.rb"
		s, err := validators.Parse(path, testdataRoot, "app/validators")
		if err != nil {
			t.Fatal(err)
		}
		out := validators.Format(s, term.NewStyler(term.ModeAlways, nil))
		if !strings.Contains(out, "\x1b[1mmodule Concerns::RuleHelpers\x1b[0m") {
			t.Errorf("missing colored header: %s", out)
		}
		if strings.Contains(out, " < ") {
			t.Errorf("module header should have no parent line: %s", out)
		}
	})
}

func parseTempValidator(t *testing.T, relPath, content string) *validators.Summary {
	t.Helper()
	root := t.TempDir()
	path := filepath.Join(root, "app", "validators", filepath.FromSlash(relPath))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := validators.Parse(path, root, "app/validators")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return s
}
