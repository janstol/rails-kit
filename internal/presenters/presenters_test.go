package presenters_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/janstol/rails-kit/internal/presenters"
	"github.com/janstol/rails-kit/internal/term"
)

const testdataRoot = "../../testdata"

func TestParse_UserPresenter(t *testing.T) {
	path := testdataRoot + "/app/presenters/user_presenter.rb"
	s, err := presenters.Parse(path, testdataRoot, "app/presenters")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if s.ClassName != "UserPresenter" {
		t.Errorf("ClassName = %q, want UserPresenter", s.ClassName)
	}
	if s.Kind != "class" {
		t.Errorf("Kind = %q, want class", s.Kind)
	}
	if s.ParentClass != "BasePresenter" {
		t.Errorf("ParentClass = %q, want BasePresenter", s.ParentClass)
	}

	if want := []string{"  DEFAULT_AVATAR = \"avatar.png\""}; !reflect.DeepEqual(s.Constants, want) {
		t.Errorf("Constants = %#v, want %#v", s.Constants, want)
	}

	if want := []string{"  Rails.application.routes.url_helpers"}; !reflect.DeepEqual(s.Concerns, want) {
		t.Errorf("Concerns = %#v, want %#v", s.Concerns, want)
	}

	if want := []string{"  attr_reader :user, :view_context, :current_admin"}; !reflect.DeepEqual(s.Attributes, want) {
		t.Errorf("Attributes = %#v, want %#v", s.Attributes, want)
	}

	if want := []string{"  delegate :to_model, to: :user"}; !reflect.DeepEqual(s.Macros, want) {
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

func TestParse_NamespacedUsersWorkOverall(t *testing.T) {
	path := testdataRoot + "/app/presenters/users/work/overall_presenter.rb"
	s, err := presenters.Parse(path, testdataRoot, "app/presenters")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.ClassName != "Users::Work::OverallPresenter" {
		t.Errorf("ClassName = %q, want Users::Work::OverallPresenter", s.ClassName)
	}
	if s.Kind != "class" {
		t.Errorf("Kind = %q, want class", s.Kind)
	}
	if want := []string{"  attr_reader :worker"}; !reflect.DeepEqual(s.Attributes, want) {
		t.Errorf("Attributes = %#v, want %#v", s.Attributes, want)
	}
	if want := []string{"  total_hours"}; !reflect.DeepEqual(s.Methods, want) {
		t.Errorf("Methods = %#v, want %#v", s.Methods, want)
	}
}

func TestParse_OnlyOutermostClass(t *testing.T) {
	content := strings.Join([]string{
		"class Broken < BasePresenter",
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
	s := parseTempPresenter(t, "broken.rb", content)

	if want := []string{"  call"}; !reflect.DeepEqual(s.Methods, want) {
		t.Fatalf("Methods leaked nested class methods: %#v", s.Methods)
	}
}

func TestParse_ReturnsPartialSummaryWithParseDiagnostics(t *testing.T) {
	s := parseTempPresenter(t, "broken_presenter.rb", "class Broken < BasePresenter\n  DEFAULT = 1\n  def call(\nend\n")

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
		path, err := presenters.Resolve(testdataRoot, "app/presenters", "user")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.HasSuffix(filepath.ToSlash(path), "app/presenters/user_presenter.rb") {
			t.Errorf("unexpected path: %s", path)
		}
	})

	t.Run("by basename", func(t *testing.T) {
		path, err := presenters.Resolve(testdataRoot, "app/presenters", "user_presenter")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.HasSuffix(filepath.ToSlash(path), "app/presenters/user_presenter.rb") {
			t.Errorf("unexpected path: %s", path)
		}
	})

	t.Run("by CamelCase class name", func(t *testing.T) {
		path, err := presenters.Resolve(testdataRoot, "app/presenters", "UserPresenter")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.HasSuffix(filepath.ToSlash(path), "app/presenters/user_presenter.rb") {
			t.Errorf("unexpected path: %s", path)
		}
	})

	t.Run("namespaced by path", func(t *testing.T) {
		path, err := presenters.Resolve(testdataRoot, "app/presenters", "users/work/overall")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.HasSuffix(filepath.ToSlash(path), "app/presenters/users/work/overall_presenter.rb") {
			t.Errorf("unexpected path: %s", path)
		}
	})

	t.Run("namespaced CamelCase constant path", func(t *testing.T) {
		path, err := presenters.Resolve(testdataRoot, "app/presenters", "Users::Work::OverallPresenter")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.HasSuffix(filepath.ToSlash(path), "app/presenters/users/work/overall_presenter.rb") {
			t.Errorf("unexpected path: %s", path)
		}
	})

	t.Run("by .rb path", func(t *testing.T) {
		path, err := presenters.Resolve(testdataRoot, "app/presenters", "app/presenters/user_presenter.rb")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.HasSuffix(filepath.ToSlash(path), "app/presenters/user_presenter.rb") {
			t.Errorf("unexpected path: %s", path)
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, err := presenters.Resolve(testdataRoot, "app/presenters", "nonexistent")
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("ambiguous basename", func(t *testing.T) {
		extra := filepath.Join(testdataRoot, "app/presenters", "billing", "overall_presenter.rb")
		if err := os.MkdirAll(filepath.Dir(extra), 0o755); err != nil {
			t.Fatalf("setup: %v", err)
		}
		if err := os.WriteFile(extra, []byte("class Billing::OverallPresenter < BasePresenter\nend\n"), 0o644); err != nil {
			t.Fatalf("setup: %v", err)
		}
		t.Cleanup(func() {
			_ = os.Remove(extra)
			_ = os.Remove(filepath.Dir(extra))
		})

		_, err := presenters.Resolve(testdataRoot, "app/presenters", "overall_presenter")
		if err == nil {
			t.Fatal("expected error for ambiguous presenter name")
		}
		if !presenters.IsAmbiguousError(err) {
			t.Errorf("expected ambiguous error, got: %v", err)
		}
		if !strings.Contains(err.Error(), "overall_presenter.rb") {
			t.Errorf("expected ambiguous error to name every match, got: %v", err)
		}
	})

	t.Run("outside presenters dir", func(t *testing.T) {
		dir := t.TempDir()
		f := filepath.Join(dir, "elsewhere.rb")
		if err := os.WriteFile(f, []byte("class Elsewhere\nend\n"), 0o644); err != nil {
			t.Fatalf("setup: %v", err)
		}
		_, err := presenters.Resolve(testdataRoot, "app/presenters", f)
		if err == nil {
			t.Fatal("expected error for file outside presenters directory")
		}
	})
}

func TestListNames(t *testing.T) {
	names, err := presenters.ListNames(testdataRoot, "app/presenters")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"user", "users/work/overall"}
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("got %v, want %v", names, want)
	}
}

func TestListNames_MissingPresentersDir(t *testing.T) {
	dir := t.TempDir()
	names, err := presenters.ListNames(dir, "app/presenters")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if names != nil {
		t.Fatalf("expected nil names, got %v", names)
	}
}

func TestFormat(t *testing.T) {
	t.Run("plain", func(t *testing.T) {
		path := testdataRoot + "/app/presenters/user_presenter.rb"
		s, err := presenters.Parse(path, testdataRoot, "app/presenters")
		if err != nil {
			t.Fatal(err)
		}
		out := presenters.Format(s, term.Styler{})
		if !strings.Contains(out, "UserPresenter < BasePresenter (") {
			t.Error("missing header in output")
		}
		if !strings.Contains(out, "Attributes:") {
			t.Error("missing Attributes section")
		}
		if !strings.Contains(out, "attr_reader :user, :view_context, :current_admin") {
			t.Error("missing attr_reader entry in output")
		}
		if !strings.Contains(out, "Macros:") {
			t.Error("missing Macros section")
		}
		if !strings.Contains(out, "Methods:") {
			t.Error("missing Methods section")
		}
	})

	t.Run("colored", func(t *testing.T) {
		t.Setenv("NO_COLOR", "")
		path := testdataRoot + "/app/presenters/users/work/overall_presenter.rb"
		s, err := presenters.Parse(path, testdataRoot, "app/presenters")
		if err != nil {
			t.Fatal(err)
		}
		out := presenters.Format(s, term.NewStyler(term.ModeAlways, nil))
		if !strings.Contains(out, "\x1b[1mUsers::Work::OverallPresenter\x1b[0m") {
			t.Errorf("missing colored header: %s", out)
		}
		if !strings.Contains(out, "\x1b[36mattr_reader\x1b[0m :worker") {
			t.Errorf("expected attr_reader entry to keep its color accent: %s", out)
		}
	})

	t.Run("module", func(t *testing.T) {
		s := parseTempPresenter(t, "concerns/formattable.rb", "module Concerns::Formattable\n  def format_price\n  end\nend\n")
		out := presenters.Format(s, term.Styler{})
		if !strings.Contains(out, "module Concerns::Formattable (") {
			t.Error("missing module header in output")
		}
		if strings.Contains(out, " < ") {
			t.Errorf("module output should not show a parent class: %s", out)
		}
	})
}

func parseTempPresenter(t *testing.T, relPath, content string) *presenters.Summary {
	t.Helper()
	root := t.TempDir()
	path := filepath.Join(root, "app", "presenters", filepath.FromSlash(relPath))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := presenters.Parse(path, root, "app/presenters")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return s
}
