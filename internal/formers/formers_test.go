package formers_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/janstol/rails-kit/internal/formers"
	"github.com/janstol/rails-kit/internal/term"
)

const testdataRoot = "../../testdata"

func TestParse_UserFormer(t *testing.T) {
	path := testdataRoot + "/app/formers/user_former.rb"
	s, err := formers.Parse(path, testdataRoot, "app/formers")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if s.ClassName != "UserFormer" {
		t.Errorf("ClassName = %q, want UserFormer", s.ClassName)
	}
	if s.Kind != "class" {
		t.Errorf("Kind = %q, want class", s.Kind)
	}
	if s.ParentClass != "" {
		t.Errorf("ParentClass = %q, want empty (ActiveModel::Model is an include, not a superclass)", s.ParentClass)
	}

	if want := []string{"  DEFAULT_ROLE = \"member\""}; !reflect.DeepEqual(s.Constants, want) {
		t.Errorf("Constants = %#v, want %#v", s.Constants, want)
	}

	if want := []string{"  ActiveModel::Model"}; !reflect.DeepEqual(s.Concerns, want) {
		t.Errorf("Concerns = %#v, want %#v", s.Concerns, want)
	}

	if want := []string{"  attr_accessor :name, :email"}; !reflect.DeepEqual(s.Attributes, want) {
		t.Errorf("Attributes = %#v, want %#v", s.Attributes, want)
	}

	wantValidations := []string{
		"  validates :name, presence: true",
		"  validates :email, presence: true, format: { with: /@/ }",
		"  validate :email_not_blacklisted",
		"  with_options if: -> { role == \"admin\" }",
		"    validates :department, presence: true",
		"    validates :approval_code, presence: true",
	}
	if !reflect.DeepEqual(s.Validations, wantValidations) {
		t.Errorf("Validations = %#v, want %#v", s.Validations, wantValidations)
	}

	if want := []string{"  delegate :to_model, to: :user"}; !reflect.DeepEqual(s.Macros, want) {
		t.Errorf("Macros = %#v, want %#v", s.Macros, want)
	}

	wantMethods := []string{
		"  save",
		"  apply(current_user)",
		"  build_default",
	}
	if !reflect.DeepEqual(s.Methods, wantMethods) {
		t.Errorf("Methods = %#v, want %#v", s.Methods, wantMethods)
	}
	// The private methods -- both the `private` block form and the
	// `private def foo; end` single-method form -- must be excluded.
	for _, m := range s.Methods {
		if strings.Contains(m, "email_not_blacklisted") || strings.Contains(m, "  user") {
			t.Errorf("private method leaked into Methods: %q", m)
		}
	}
}

func TestParse_AltSuffixSessionForm(t *testing.T) {
	path := testdataRoot + "/app/formers/session_form.rb"
	s, err := formers.Parse(path, testdataRoot, "app/formers")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.ClassName != "SessionForm" {
		t.Errorf("ClassName = %q, want SessionForm", s.ClassName)
	}
	wantValidations := []string{
		"  validates :email, presence: true",
		"  validates :password, presence: true",
	}
	if !reflect.DeepEqual(s.Validations, wantValidations) {
		t.Errorf("Validations = %#v, want %#v", s.Validations, wantValidations)
	}
}

func TestParse_NamespacedAdminReport(t *testing.T) {
	path := testdataRoot + "/app/formers/admin/report_former.rb"
	s, err := formers.Parse(path, testdataRoot, "app/formers")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.ClassName != "Admin::ReportFormer" {
		t.Errorf("ClassName = %q, want Admin::ReportFormer", s.ClassName)
	}
	if s.Kind != "class" {
		t.Errorf("Kind = %q, want class", s.Kind)
	}
	if want := []string{"  validates :start_date, presence: true"}; !reflect.DeepEqual(s.Validations, want) {
		t.Errorf("Validations = %#v, want %#v", s.Validations, want)
	}
	if want := []string{"  generate"}; !reflect.DeepEqual(s.Methods, want) {
		t.Errorf("Methods = %#v, want %#v", s.Methods, want)
	}
}

func TestParse_ConcernsModule(t *testing.T) {
	// Nothing owns app/formers/concerns the way app/controllers/concerns is
	// owned by the concerns command, so it must resolve and parse cleanly as
	// a module like any other former.
	path := testdataRoot + "/app/formers/concerns/validatable.rb"
	s, err := formers.Parse(path, testdataRoot, "app/formers")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.ClassName != "Concerns::Validatable" {
		t.Errorf("ClassName = %q, want Concerns::Validatable", s.ClassName)
	}
	if s.Kind != "module" {
		t.Errorf("Kind = %q, want module", s.Kind)
	}
	want := []string{"  validate_presence_of_all(*attrs)", "  validation_summary"}
	if !reflect.DeepEqual(s.Methods, want) {
		t.Errorf("Methods = %#v, want %#v", s.Methods, want)
	}
}

func TestParse_WithOptionsWithoutValidationsStaysInMacros(t *testing.T) {
	content := strings.Join([]string{
		"class Custom",
		"  with_options if: :admin? do",
		"    attr_accessor :note",
		"  end",
		"end",
		"",
	}, "\n")
	s := parseTempFormer(t, "custom_former.rb", content)

	want := []string{"  with_options if: :admin? (block)"}
	if !reflect.DeepEqual(s.Macros, want) {
		t.Fatalf("Macros = %#v, want %#v (with_options holding no validations must render like any other macro)", s.Macros, want)
	}
	if len(s.Validations) != 0 {
		t.Fatalf("Validations = %#v, want empty", s.Validations)
	}
}

func TestParse_OnlyOutermostClass(t *testing.T) {
	content := strings.Join([]string{
		"class Broken",
		"  def call",
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
	s := parseTempFormer(t, "broken.rb", content)

	if want := []string{"  call"}; !reflect.DeepEqual(s.Methods, want) {
		t.Fatalf("Methods leaked nested class methods: %#v", s.Methods)
	}
}

func TestParse_ReturnsPartialSummaryWithParseDiagnostics(t *testing.T) {
	s := parseTempFormer(t, "broken_former.rb", "class Broken\n  DEFAULT = 1\n  def call(\nend\n")

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
		path, err := formers.Resolve(testdataRoot, "app/formers", "user")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.HasSuffix(filepath.ToSlash(path), "app/formers/user_former.rb") {
			t.Errorf("unexpected path: %s", path)
		}
	})

	t.Run("by basename", func(t *testing.T) {
		path, err := formers.Resolve(testdataRoot, "app/formers", "user_former")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.HasSuffix(filepath.ToSlash(path), "app/formers/user_former.rb") {
			t.Errorf("unexpected path: %s", path)
		}
	})

	t.Run("by CamelCase class name", func(t *testing.T) {
		path, err := formers.Resolve(testdataRoot, "app/formers", "UserFormer")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.HasSuffix(filepath.ToSlash(path), "app/formers/user_former.rb") {
			t.Errorf("unexpected path: %s", path)
		}
	})

	t.Run("alt suffix by resource name", func(t *testing.T) {
		path, err := formers.Resolve(testdataRoot, "app/formers", "session")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.HasSuffix(filepath.ToSlash(path), "app/formers/session_form.rb") {
			t.Errorf("unexpected path: %s, want app/formers/session_form.rb suffix", path)
		}
	})

	t.Run("alt suffix by basename", func(t *testing.T) {
		path, err := formers.Resolve(testdataRoot, "app/formers", "session_form")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.HasSuffix(filepath.ToSlash(path), "app/formers/session_form.rb") {
			t.Errorf("unexpected path: %s", path)
		}
	})

	t.Run("namespaced by path", func(t *testing.T) {
		path, err := formers.Resolve(testdataRoot, "app/formers", "admin/report")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.HasSuffix(filepath.ToSlash(path), "app/formers/admin/report_former.rb") {
			t.Errorf("unexpected path: %s", path)
		}
	})

	t.Run("namespaced CamelCase class name", func(t *testing.T) {
		path, err := formers.Resolve(testdataRoot, "app/formers", "Admin::ReportFormer")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.HasSuffix(filepath.ToSlash(path), "app/formers/admin/report_former.rb") {
			t.Errorf("unexpected path: %s", path)
		}
	})

	t.Run("by .rb path", func(t *testing.T) {
		path, err := formers.Resolve(testdataRoot, "app/formers", "app/formers/user_former.rb")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.HasSuffix(filepath.ToSlash(path), "app/formers/user_former.rb") {
			t.Errorf("unexpected path: %s", path)
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, err := formers.Resolve(testdataRoot, "app/formers", "nonexistent")
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("ambiguous basename", func(t *testing.T) {
		extra := filepath.Join(testdataRoot, "app/formers", "billing", "report_former.rb")
		if err := os.MkdirAll(filepath.Dir(extra), 0o755); err != nil {
			t.Fatalf("setup: %v", err)
		}
		if err := os.WriteFile(extra, []byte("class Billing::ReportFormer\nend\n"), 0o644); err != nil {
			t.Fatalf("setup: %v", err)
		}
		t.Cleanup(func() {
			_ = os.Remove(extra)
			_ = os.Remove(filepath.Dir(extra))
		})

		_, err := formers.Resolve(testdataRoot, "app/formers", "report_former")
		if err == nil {
			t.Fatal("expected error for ambiguous former name")
		}
		if !formers.IsAmbiguousError(err) {
			t.Errorf("expected ambiguous error, got: %v", err)
		}
	})

	t.Run("outside formers dir", func(t *testing.T) {
		dir := t.TempDir()
		f := filepath.Join(dir, "elsewhere.rb")
		if err := os.WriteFile(f, []byte("class Elsewhere\nend\n"), 0o644); err != nil {
			t.Fatalf("setup: %v", err)
		}
		_, err := formers.Resolve(testdataRoot, "app/formers", f)
		if err == nil {
			t.Fatal("expected error for file outside formers directory")
		}
	})
}

func TestListNames(t *testing.T) {
	names, err := formers.ListNames(testdataRoot, "app/formers")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"admin/report", "concerns/validatable", "session", "user"}
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("got %v, want %v", names, want)
	}
}

func TestListNames_DedupesAcrossSuffixConventions(t *testing.T) {
	dir := t.TempDir()
	for _, rel := range []string{"app/formers/sync_user_former.rb", "app/formers/sync_user_form.rb"} {
		full := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("setup: %v", err)
		}
		if err := os.WriteFile(full, []byte("class SyncUserFormer\nend\n"), 0o644); err != nil {
			t.Fatalf("setup: %v", err)
		}
	}

	names, err := formers.ListNames(dir, "app/formers")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"sync_user"}
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("got %v, want %v (both suffix conventions reduce to the same name once)", names, want)
	}
}

func TestListNames_MissingFormersDir(t *testing.T) {
	dir := t.TempDir()
	names, err := formers.ListNames(dir, "app/formers")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if names != nil {
		t.Fatalf("expected nil names, got %v", names)
	}
}

func TestFormat(t *testing.T) {
	t.Run("plain", func(t *testing.T) {
		path := testdataRoot + "/app/formers/user_former.rb"
		s, err := formers.Parse(path, testdataRoot, "app/formers")
		if err != nil {
			t.Fatal(err)
		}
		out := formers.Format(s, term.Styler{})
		if !strings.Contains(out, "UserFormer (") {
			t.Error("missing header in output")
		}
		if !strings.Contains(out, "Attributes:") {
			t.Error("missing Attributes section")
		}
		if !strings.Contains(out, "Validations:") {
			t.Error("missing Validations section")
		}
		if !strings.Contains(out, "    validates :department, presence: true") {
			t.Error("missing nested with_options validation in output")
		}
		if !strings.Contains(out, "Macros:") {
			t.Error("missing Macros section")
		}
		if !strings.Contains(out, "Methods:") {
			t.Error("missing Methods section")
		}
	})

	t.Run("colored", func(t *testing.T) {
		path := testdataRoot + "/app/formers/concerns/validatable.rb"
		s, err := formers.Parse(path, testdataRoot, "app/formers")
		if err != nil {
			t.Fatal(err)
		}
		out := formers.Format(s, term.NewStyler(term.ModeAlways, nil))
		if !strings.Contains(out, "\x1b[1mmodule Concerns::Validatable\x1b[0m") {
			t.Errorf("missing colored header: %s", out)
		}
		if !strings.Contains(out, "Methods:") {
			t.Error("missing Methods section")
		}
	})

	t.Run("nested with_options entry keeps macro accent", func(t *testing.T) {
		path := testdataRoot + "/app/formers/user_former.rb"
		s, err := formers.Parse(path, testdataRoot, "app/formers")
		if err != nil {
			t.Fatal(err)
		}
		out := formers.Format(s, term.NewStyler(term.ModeAlways, nil))
		if !strings.Contains(out, "    \x1b[36mvalidates\x1b[0m :department, presence: true") {
			t.Errorf("expected nested validates entry to keep its color accent: %s", out)
		}
	})
}

func parseTempFormer(t *testing.T, relPath, content string) *formers.Summary {
	t.Helper()
	root := t.TempDir()
	path := filepath.Join(root, "app", "formers", filepath.FromSlash(relPath))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := formers.Parse(path, root, "app/formers")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return s
}
