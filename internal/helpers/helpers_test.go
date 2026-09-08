package helpers_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/janstol/rails-kit/internal/helpers"
	"github.com/janstol/rails-kit/internal/term"
)

const testdataRoot = "../../testdata"

func TestParse_UsersHelper(t *testing.T) {
	path := testdataRoot + "/app/helpers/users_helper.rb"
	s, err := helpers.Parse(path, testdataRoot, "app/helpers")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if s.ClassName != "UsersHelper" {
		t.Errorf("ClassName = %q, want UsersHelper", s.ClassName)
	}
	if s.Kind != "module" {
		t.Errorf("Kind = %q, want module", s.Kind)
	}
	if s.ParentClass != "" {
		t.Errorf("ParentClass = %q, want empty", s.ParentClass)
	}

	if want := []string{"  DEFAULT_AVATAR_SIZE = 40"}; !reflect.DeepEqual(s.Constants, want) {
		t.Errorf("Constants = %#v, want %#v", s.Constants, want)
	}
	if want := []string{"  IconHelper"}; !reflect.DeepEqual(s.Concerns, want) {
		t.Errorf("Concerns = %#v, want %#v", s.Concerns, want)
	}

	want := []string{
		"  user_avatar(user)",
		"  user_badge(user, size = DEFAULT_AVATAR_SIZE)",
		"  user_status_tag(user:, css_class: \"status\")",
		"  user_tags(*tags)",
		"  with_user_context(&block)",
		"  current_user_name",
		"  user_summary(user, show_email: false, show_role: false)",
		"  default_avatar_url",
		"  default_role_label",
		"  internal_role_key",
	}
	if !reflect.DeepEqual(s.Methods, want) {
		t.Errorf("Methods = %#v, want %#v", s.Methods, want)
	}

	// The private methods -- both the `private` block form and the
	// `private def foo; end` single-method form -- must be excluded.
	for _, m := range s.Methods {
		if strings.Contains(m, "user_secret_token") || strings.Contains(m, "formatted_user_id") {
			t.Errorf("private method leaked into Methods: %q", m)
		}
	}
}

func TestParse_NamespacedAdminReports(t *testing.T) {
	path := testdataRoot + "/app/helpers/admin/reports_helper.rb"
	s, err := helpers.Parse(path, testdataRoot, "app/helpers")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.ClassName != "Admin::ReportsHelper" {
		t.Errorf("ClassName = %q, want Admin::ReportsHelper", s.ClassName)
	}
	if s.Kind != "module" {
		t.Errorf("Kind = %q, want module", s.Kind)
	}
	want := []string{"  report_title(report)", "  report_period(report)"}
	if !reflect.DeepEqual(s.Methods, want) {
		t.Errorf("Methods = %#v, want %#v", s.Methods, want)
	}
}

func TestParse_ApplicationHelper(t *testing.T) {
	path := testdataRoot + "/app/helpers/application_helper.rb"
	s, err := helpers.Parse(path, testdataRoot, "app/helpers")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.ClassName != "ApplicationHelper" {
		t.Errorf("ClassName = %q, want ApplicationHelper", s.ClassName)
	}
	want := []string{"  page_title(title)"}
	if !reflect.DeepEqual(s.Methods, want) {
		t.Errorf("Methods = %#v, want %#v", s.Methods, want)
	}
}

func TestParse_ClassForm(t *testing.T) {
	content := strings.Join([]string{
		"class LegacyHelper < BaseHelper",
		"  def call",
		"  end",
		"end",
		"",
	}, "\n")
	s := parseTempHelper(t, "legacy_helper.rb", content)

	if s.Kind != "class" {
		t.Fatalf("Kind = %q, want class", s.Kind)
	}
	if s.ParentClass != "BaseHelper" {
		t.Fatalf("ParentClass = %q, want BaseHelper", s.ParentClass)
	}
	if want := []string{"  call"}; !reflect.DeepEqual(s.Methods, want) {
		t.Fatalf("Methods = %#v, want %#v", s.Methods, want)
	}
}

// TestParse_OnlyOutermostModule pins astutil's namespacing rule as applied to
// helpers: a method defined directly on the wrapping namespace module must
// not leak into the recognized inner module's summary, mirroring the
// `module Admin; module ReportsHelper; ...; end; end` idiom real helper files
// use for namespacing.
// TestParse_PureNamespaceModuleDescendsToNested pins the legitimate
// namespacing idiom: a wrapping module with no content of its own besides
// the nested module is pure namespacing, so Parse descends into the nested
// module -- the real `module Admin; module ReportsHelper; ...; end; end`
// shape.
func TestParse_PureNamespaceModuleDescendsToNested(t *testing.T) {
	content := strings.Join([]string{
		"module Namespace",
		"  module Nested",
		"    def real_method",
		"    end",
		"  end",
		"end",
		"",
	}, "\n")
	s := parseTempHelper(t, "nested_helper.rb", content)

	if want := []string{"  real_method"}; !reflect.DeepEqual(s.Methods, want) {
		t.Fatalf("Methods = %#v, want %#v", s.Methods, want)
	}
}

// TestParse_ModuleWithOwnMethodKeepsOwnMethodOverNestedClass pins the fix for
// a real bug found dogfooding: a module that defines its own methods
// alongside a nested class (a link-renderer/decorator-style implementation
// detail, e.g. real apps' `WillPaginateHelper`/`CarouselHelper`) must keep
// its own methods as the summary -- the nested class is not the file's real
// target, and must not silently replace the module's actually-callable
// methods (nor leak its own method into the module's summary, mirroring the
// anti-leak rule for nested classes elsewhere).
func TestParse_ModuleWithOwnMethodKeepsOwnMethodOverNestedClass(t *testing.T) {
	content := strings.Join([]string{
		"module Paginator",
		"  def paginate_remote(collection)",
		"  end",
		"",
		"  class RemoteLinkRenderer",
		"    def link",
		"    end",
		"  end",
		"end",
		"",
	}, "\n")
	s := parseTempHelper(t, "paginator_helper.rb", content)

	if s.Kind != "module" {
		t.Errorf("Kind = %q, want module", s.Kind)
	}
	want := []string{"  paginate_remote(collection)"}
	if !reflect.DeepEqual(s.Methods, want) {
		t.Fatalf("Methods = %#v, want %#v (RemoteLinkRenderer#link must not leak, "+
			"and must not replace paginate_remote)", s.Methods, want)
	}
}

func TestParse_ReturnsPartialSummaryWithParseDiagnostics(t *testing.T) {
	s := parseTempHelper(t, "broken_helper.rb", "module Broken\n  DEFAULT = 1\n  def call(\nend\n")

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
		path, err := helpers.Resolve(testdataRoot, "app/helpers", "users")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.HasSuffix(filepath.ToSlash(path), "app/helpers/users_helper.rb") {
			t.Errorf("unexpected path: %s", path)
		}
	})

	t.Run("by basename", func(t *testing.T) {
		path, err := helpers.Resolve(testdataRoot, "app/helpers", "users_helper")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.HasSuffix(filepath.ToSlash(path), "app/helpers/users_helper.rb") {
			t.Errorf("unexpected path: %s", path)
		}
	})

	t.Run("by CamelCase module name", func(t *testing.T) {
		path, err := helpers.Resolve(testdataRoot, "app/helpers", "UsersHelper")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.HasSuffix(filepath.ToSlash(path), "app/helpers/users_helper.rb") {
			t.Errorf("unexpected path: %s", path)
		}
	})

	t.Run("namespaced by path", func(t *testing.T) {
		path, err := helpers.Resolve(testdataRoot, "app/helpers", "admin/reports")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.HasSuffix(filepath.ToSlash(path), "app/helpers/admin/reports_helper.rb") {
			t.Errorf("unexpected path: %s", path)
		}
	})

	t.Run("namespaced CamelCase module name", func(t *testing.T) {
		path, err := helpers.Resolve(testdataRoot, "app/helpers", "Admin::ReportsHelper")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.HasSuffix(filepath.ToSlash(path), "app/helpers/admin/reports_helper.rb") {
			t.Errorf("unexpected path: %s", path)
		}
	})

	t.Run("by .rb path", func(t *testing.T) {
		path, err := helpers.Resolve(testdataRoot, "app/helpers", "app/helpers/users_helper.rb")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.HasSuffix(filepath.ToSlash(path), "app/helpers/users_helper.rb") {
			t.Errorf("unexpected path: %s", path)
		}
	})

	t.Run("suffix preferred over raw filename", func(t *testing.T) {
		dir := t.TempDir()
		helpersDir := filepath.Join(dir, "app", "helpers")
		if err := os.MkdirAll(helpersDir, 0o755); err != nil {
			t.Fatalf("setup: %v", err)
		}
		if err := os.WriteFile(filepath.Join(helpersDir, "widgets_helper.rb"), []byte("module WidgetsHelper\nend\n"), 0o644); err != nil {
			t.Fatalf("setup: %v", err)
		}
		if err := os.WriteFile(filepath.Join(helpersDir, "widgets.rb"), []byte("module Widgets\nend\n"), 0o644); err != nil {
			t.Fatalf("setup: %v", err)
		}
		path, err := helpers.Resolve(dir, "app/helpers", "widgets")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.HasSuffix(filepath.ToSlash(path), "widgets_helper.rb") {
			t.Errorf("expected suffixed file to win, got: %s", path)
		}
	})

	t.Run("falls back to raw filename when no suffix match", func(t *testing.T) {
		dir := t.TempDir()
		helpersDir := filepath.Join(dir, "app", "helpers")
		if err := os.MkdirAll(helpersDir, 0o755); err != nil {
			t.Fatalf("setup: %v", err)
		}
		if err := os.WriteFile(filepath.Join(helpersDir, "legacy.rb"), []byte("module Legacy\nend\n"), 0o644); err != nil {
			t.Fatalf("setup: %v", err)
		}
		path, err := helpers.Resolve(dir, "app/helpers", "legacy")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.HasSuffix(filepath.ToSlash(path), "legacy.rb") {
			t.Errorf("unexpected path: %s", path)
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, err := helpers.Resolve(testdataRoot, "app/helpers", "nonexistent")
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("ambiguous basename", func(t *testing.T) {
		// The real fixture lives at admin/reports_helper.rb (nested), so
		// resolving by bare basename never hits the direct-path short
		// circuit; adding a second nested match makes it ambiguous.
		extra := filepath.Join(testdataRoot, "app/helpers", "billing", "reports_helper.rb")
		if err := os.MkdirAll(filepath.Dir(extra), 0o755); err != nil {
			t.Fatalf("setup: %v", err)
		}
		if err := os.WriteFile(extra, []byte("module Billing::ReportsHelper\nend\n"), 0o644); err != nil {
			t.Fatalf("setup: %v", err)
		}
		t.Cleanup(func() {
			_ = os.Remove(extra)
			_ = os.Remove(filepath.Dir(extra))
		})

		_, err := helpers.Resolve(testdataRoot, "app/helpers", "reports_helper")
		if err == nil {
			t.Fatal("expected error for ambiguous helper name")
		}
		if !helpers.IsAmbiguousError(err) {
			t.Errorf("expected ambiguous error, got: %v", err)
		}
	})

	t.Run("outside helpers dir", func(t *testing.T) {
		dir := t.TempDir()
		f := filepath.Join(dir, "elsewhere.rb")
		if err := os.WriteFile(f, []byte("module Elsewhere\nend\n"), 0o644); err != nil {
			t.Fatalf("setup: %v", err)
		}
		_, err := helpers.Resolve(testdataRoot, "app/helpers", f)
		if err == nil {
			t.Fatal("expected error for file outside helpers directory")
		}
	})
}

func TestListNames(t *testing.T) {
	names, err := helpers.ListNames(testdataRoot, "app/helpers")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"admin/reports", "application", "users"}
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("got %v, want %v", names, want)
	}
}

func TestListNames_MissingHelpersDir(t *testing.T) {
	dir := t.TempDir()
	names, err := helpers.ListNames(dir, "app/helpers")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if names != nil {
		t.Fatalf("expected nil names, got %v", names)
	}
}

func TestFormat(t *testing.T) {
	t.Run("plain", func(t *testing.T) {
		path := testdataRoot + "/app/helpers/admin/reports_helper.rb"
		s, err := helpers.Parse(path, testdataRoot, "app/helpers")
		if err != nil {
			t.Fatal(err)
		}
		out := helpers.Format(s, term.Styler{})
		if !strings.Contains(out, "module Admin::ReportsHelper (") {
			t.Error("missing module header in output")
		}
		if strings.Contains(out, " < ") {
			t.Errorf("module output should not show a parent class: %s", out)
		}
		if !strings.Contains(out, "Methods:") {
			t.Error("missing Methods section")
		}
		if !strings.Contains(out, "report_title(report)") {
			t.Error("missing method signature in output")
		}
	})

	t.Run("colored", func(t *testing.T) {
		path := testdataRoot + "/app/helpers/users_helper.rb"
		s, err := helpers.Parse(path, testdataRoot, "app/helpers")
		if err != nil {
			t.Fatal(err)
		}
		out := helpers.Format(s, term.NewStyler(term.ModeAlways, nil))
		if !strings.Contains(out, "\x1b[1mmodule UsersHelper\x1b[0m") {
			t.Errorf("missing colored header: %s", out)
		}
		if !strings.Contains(out, "Constants:") {
			t.Error("missing Constants section")
		}
		if !strings.Contains(out, "Concerns:") {
			t.Error("missing Concerns section")
		}
	})
}

func parseTempHelper(t *testing.T, relPath, content string) *helpers.Summary {
	t.Helper()
	root := t.TempDir()
	path := filepath.Join(root, "app", "helpers", filepath.FromSlash(relPath))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := helpers.Parse(path, root, "app/helpers")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return s
}
