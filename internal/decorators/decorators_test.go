package decorators_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/janstol/rails-kit/internal/decorators"
	"github.com/janstol/rails-kit/internal/term"
)

const testdataRoot = "../../testdata"

func TestParse_UserDecorator(t *testing.T) {
	path := testdataRoot + "/app/decorators/user_decorator.rb"
	s, err := decorators.Parse(path, testdataRoot, "app/decorators")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if s.ClassName != "UserDecorator" {
		t.Errorf("ClassName = %q, want UserDecorator", s.ClassName)
	}
	if s.Kind != "class" {
		t.Errorf("Kind = %q, want class", s.Kind)
	}
	if s.ParentClass != "ApplicationDecorator" {
		t.Errorf("ParentClass = %q, want ApplicationDecorator", s.ParentClass)
	}

	if want := []string{"  DEFAULT_AVATAR = \"avatar.png\""}; !reflect.DeepEqual(s.Constants, want) {
		t.Errorf("Constants = %#v, want %#v", s.Constants, want)
	}

	// Rails.application.routes.url_helpers is a dotted CallNode chain, not a
	// ConstantReadNode/ConstantPathNode -- this pins the IncludedConcern
	// fallback that keeps it from being silently dropped. The constant-path
	// include (ActionView::Helpers::NumberHelper) must still work too.
	want := []string{"  Rails.application.routes.url_helpers", "  ActionView::Helpers::NumberHelper"}
	if !reflect.DeepEqual(s.Concerns, want) {
		t.Errorf("Concerns = %#v, want %#v", s.Concerns, want)
	}

	if want := []string{"  delegate_all"}; !reflect.DeepEqual(s.Macros, want) {
		t.Errorf("Macros = %#v, want %#v", s.Macros, want)
	}

	wantMethods := []string{
		"  full_name",
		"  formatted_created_at(format: :short)",
		"  build_default",
	}
	if !reflect.DeepEqual(s.Methods, wantMethods) {
		t.Errorf("Methods = %#v, want %#v", s.Methods, wantMethods)
	}
	// The private methods -- both the `private` block form and the
	// `private def foo; end` single-method form -- must be excluded.
	for _, m := range s.Methods {
		if strings.Contains(m, "internal_token") || strings.Contains(m, "secret_hash") {
			t.Errorf("private method leaked into Methods: %q", m)
		}
	}
}

func TestParse_NamespacedAdminReport(t *testing.T) {
	path := testdataRoot + "/app/decorators/admin/report_decorator.rb"
	s, err := decorators.Parse(path, testdataRoot, "app/decorators")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.ClassName != "Admin::ReportDecorator" {
		t.Errorf("ClassName = %q, want Admin::ReportDecorator", s.ClassName)
	}
	if s.Kind != "class" {
		t.Errorf("Kind = %q, want class", s.Kind)
	}
	if s.ParentClass != "ApplicationDecorator" {
		t.Errorf("ParentClass = %q, want ApplicationDecorator", s.ParentClass)
	}
	if want := []string{"  delegate_all"}; !reflect.DeepEqual(s.Macros, want) {
		t.Errorf("Macros = %#v, want %#v", s.Macros, want)
	}
	if want := []string{"  summary_line"}; !reflect.DeepEqual(s.Methods, want) {
		t.Errorf("Methods = %#v, want %#v", s.Methods, want)
	}
}

func TestParse_ConcernsModule(t *testing.T) {
	// Nothing owns app/decorators/concerns the way app/controllers/concerns
	// is owned by the concerns command, so it must resolve and parse cleanly
	// as a module like any other decorator file.
	path := testdataRoot + "/app/decorators/concerns/formatting.rb"
	s, err := decorators.Parse(path, testdataRoot, "app/decorators")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.ClassName != "Concerns::Formatting" {
		t.Errorf("ClassName = %q, want Concerns::Formatting", s.ClassName)
	}
	if s.Kind != "module" {
		t.Errorf("Kind = %q, want module", s.Kind)
	}
	want := []string{"  formatted_amount(cents)", "  formatted_date(date)"}
	if !reflect.DeepEqual(s.Methods, want) {
		t.Errorf("Methods = %#v, want %#v", s.Methods, want)
	}
}

func TestParse_DottedIncludeNotDropped(t *testing.T) {
	content := strings.Join([]string{
		"class Custom < ApplicationDecorator",
		"  include Rails.application.routes.url_helpers",
		"end",
		"",
	}, "\n")
	s := parseTempDecorator(t, "custom_decorator.rb", content)

	want := []string{"  Rails.application.routes.url_helpers"}
	if !reflect.DeepEqual(s.Concerns, want) {
		t.Fatalf("Concerns = %#v, want %#v (dotted include must not be silently dropped)", s.Concerns, want)
	}
}

func TestParse_OnlyOutermostClass(t *testing.T) {
	content := strings.Join([]string{
		"class Broken < ApplicationDecorator",
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
	s := parseTempDecorator(t, "broken.rb", content)

	if want := []string{"  call"}; !reflect.DeepEqual(s.Methods, want) {
		t.Fatalf("Methods leaked nested class methods: %#v", s.Methods)
	}
}

func TestParse_ReturnsPartialSummaryWithParseDiagnostics(t *testing.T) {
	s := parseTempDecorator(t, "broken_decorator.rb", "class Broken < ApplicationDecorator\n  DEFAULT = 1\n  def call(\nend\n")

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
		path, err := decorators.Resolve(testdataRoot, "app/decorators", "user")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.HasSuffix(filepath.ToSlash(path), "app/decorators/user_decorator.rb") {
			t.Errorf("unexpected path: %s", path)
		}
	})

	t.Run("by basename", func(t *testing.T) {
		path, err := decorators.Resolve(testdataRoot, "app/decorators", "user_decorator")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.HasSuffix(filepath.ToSlash(path), "app/decorators/user_decorator.rb") {
			t.Errorf("unexpected path: %s", path)
		}
	})

	t.Run("by CamelCase class name", func(t *testing.T) {
		path, err := decorators.Resolve(testdataRoot, "app/decorators", "UserDecorator")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.HasSuffix(filepath.ToSlash(path), "app/decorators/user_decorator.rb") {
			t.Errorf("unexpected path: %s", path)
		}
	})

	t.Run("namespaced by path", func(t *testing.T) {
		path, err := decorators.Resolve(testdataRoot, "app/decorators", "admin/report")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.HasSuffix(filepath.ToSlash(path), "app/decorators/admin/report_decorator.rb") {
			t.Errorf("unexpected path: %s", path)
		}
	})

	t.Run("namespaced CamelCase class name", func(t *testing.T) {
		path, err := decorators.Resolve(testdataRoot, "app/decorators", "Admin::ReportDecorator")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.HasSuffix(filepath.ToSlash(path), "app/decorators/admin/report_decorator.rb") {
			t.Errorf("unexpected path: %s", path)
		}
	})

	t.Run("by .rb path", func(t *testing.T) {
		path, err := decorators.Resolve(testdataRoot, "app/decorators", "app/decorators/user_decorator.rb")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.HasSuffix(filepath.ToSlash(path), "app/decorators/user_decorator.rb") {
			t.Errorf("unexpected path: %s", path)
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, err := decorators.Resolve(testdataRoot, "app/decorators", "nonexistent")
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("ambiguous basename", func(t *testing.T) {
		extra := filepath.Join(testdataRoot, "app/decorators", "billing", "report_decorator.rb")
		if err := os.MkdirAll(filepath.Dir(extra), 0o755); err != nil {
			t.Fatalf("setup: %v", err)
		}
		if err := os.WriteFile(extra, []byte("class Billing::ReportDecorator < ApplicationDecorator\nend\n"), 0o644); err != nil {
			t.Fatalf("setup: %v", err)
		}
		t.Cleanup(func() {
			_ = os.Remove(extra)
			_ = os.Remove(filepath.Dir(extra))
		})

		_, err := decorators.Resolve(testdataRoot, "app/decorators", "report_decorator")
		if err == nil {
			t.Fatal("expected error for ambiguous decorator name")
		}
		if !decorators.IsAmbiguousError(err) {
			t.Errorf("expected ambiguous error, got: %v", err)
		}
	})

	t.Run("outside decorators dir", func(t *testing.T) {
		dir := t.TempDir()
		f := filepath.Join(dir, "elsewhere.rb")
		if err := os.WriteFile(f, []byte("class Elsewhere\nend\n"), 0o644); err != nil {
			t.Fatalf("setup: %v", err)
		}
		_, err := decorators.Resolve(testdataRoot, "app/decorators", f)
		if err == nil {
			t.Fatal("expected error for file outside decorators directory")
		}
	})
}

func TestListNames(t *testing.T) {
	names, err := decorators.ListNames(testdataRoot, "app/decorators")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"admin/report", "concerns/formatting", "user"}
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("got %v, want %v", names, want)
	}
}

func TestListNames_MissingDecoratorsDir(t *testing.T) {
	dir := t.TempDir()
	names, err := decorators.ListNames(dir, "app/decorators")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if names != nil {
		t.Fatalf("expected nil names, got %v", names)
	}
}

func TestFormat(t *testing.T) {
	t.Run("plain", func(t *testing.T) {
		path := testdataRoot + "/app/decorators/admin/report_decorator.rb"
		s, err := decorators.Parse(path, testdataRoot, "app/decorators")
		if err != nil {
			t.Fatal(err)
		}
		out := decorators.Format(s, term.Styler{})
		if !strings.Contains(out, "Admin::ReportDecorator < ApplicationDecorator (") {
			t.Error("missing header in output")
		}
		if !strings.Contains(out, "Macros:") {
			t.Error("missing Macros section")
		}
		if !strings.Contains(out, "delegate_all") {
			t.Error("missing delegate_all macro in output")
		}
		if !strings.Contains(out, "Methods:") {
			t.Error("missing Methods section")
		}
	})

	t.Run("colored", func(t *testing.T) {
		t.Setenv("NO_COLOR", "")
		path := testdataRoot + "/app/decorators/concerns/formatting.rb"
		s, err := decorators.Parse(path, testdataRoot, "app/decorators")
		if err != nil {
			t.Fatal(err)
		}
		out := decorators.Format(s, term.NewStyler(term.ModeAlways, nil))
		if !strings.Contains(out, "\x1b[1mmodule Concerns::Formatting\x1b[0m") {
			t.Errorf("missing colored header: %s", out)
		}
		if !strings.Contains(out, "Methods:") {
			t.Error("missing Methods section")
		}
	})
}

func parseTempDecorator(t *testing.T, relPath, content string) *decorators.Summary {
	t.Helper()
	root := t.TempDir()
	path := filepath.Join(root, "app", "decorators", filepath.FromSlash(relPath))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := decorators.Parse(path, root, "app/decorators")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return s
}
