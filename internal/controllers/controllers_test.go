package controllers_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/janstol/rails-kit/internal/controllers"
	"github.com/janstol/rails-kit/internal/term"
)

const testdataRoot = "../../testdata"

func TestParse_Users(t *testing.T) {
	path := testdataRoot + "/app/controllers/users_controller.rb"
	s, err := controllers.Parse(path, testdataRoot, "app/controllers")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if s.ClassName != "UsersController" {
		t.Errorf("ClassName = %q, want UsersController", s.ClassName)
	}
	if s.ParentClass != "ApplicationController" {
		t.Errorf("ParentClass = %q, want ApplicationController", s.ParentClass)
	}
	if s.Layout != `"users"` {
		t.Errorf("Layout = %q, want \"users\"", s.Layout)
	}

	if !containsSubstr(s.Filters, "before_action :authenticate_user!") {
		t.Errorf("expected before_action :authenticate_user!, got %v", s.Filters)
	}
	if !containsSubstr(s.Filters, "only: [:show, :edit, :update, :destroy]") {
		t.Errorf("expected only: option, got %v", s.Filters)
	}
	if !containsSubstr(s.Filters, "skip_before_action :authenticate_user!, only: [:index], if: :public_action?") {
		t.Errorf("expected skip_before_action with if:, got %v", s.Filters)
	}
	if !containsSubstr(s.Filters, "around_action :measure_time") {
		t.Errorf("expected around_action, got %v", s.Filters)
	}

	if !containsSubstr(s.RescueFrom, "rescue_from ActiveRecord::RecordNotFound, ActiveRecord::RecordInvalid, with: :handle_not_found") {
		t.Errorf("expected rescue_from with with:, got %v", s.RescueFrom)
	}
	if !containsSubstr(s.RescueFrom, "rescue_from StandardError (block)") {
		t.Errorf("expected block-form rescue_from, got %v", s.RescueFrom)
	}

	if !containsSubstr(s.HelperMethods, "user_display_name") {
		t.Errorf("expected helper_method user_display_name, got %v", s.HelperMethods)
	}

	if want := []string{"  html", "  json"}; !reflect.DeepEqual(s.RespondTo, want) {
		t.Errorf("RespondTo = %#v, want %#v", s.RespondTo, want)
	}

	if want := []string{"  user_params: params.require(:user).permit(:name, :email, tags: [])"}; !reflect.DeepEqual(s.StrongParams, want) {
		t.Errorf("StrongParams = %#v, want %#v", s.StrongParams, want)
	}

	if want := []string{"  index", "  show", "  create", "  update", "  destroy"}; !reflect.DeepEqual(s.Actions, want) {
		t.Errorf("Actions = %#v, want %#v", s.Actions, want)
	}
}

func TestParse_Application(t *testing.T) {
	path := testdataRoot + "/app/controllers/application_controller.rb"
	s, err := controllers.Parse(path, testdataRoot, "app/controllers")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if s.ClassName != "ApplicationController" {
		t.Errorf("ClassName = %q, want ApplicationController", s.ClassName)
	}
	if s.ParentClass != "ActionController::Base" {
		t.Errorf("ParentClass = %q, want ActionController::Base", s.ParentClass)
	}
	if !containsSubstr(s.Concerns, "Authenticatable") {
		t.Errorf("expected Authenticatable concern, got %v", s.Concerns)
	}
	if !containsSubstr(s.HelperMethods, "current_user") || !containsSubstr(s.HelperMethods, "current_account") {
		t.Errorf("expected current_user and current_account helpers, got %v", s.HelperMethods)
	}
	if !containsSubstr(s.RescueFrom, "with: :render_not_found") {
		t.Errorf("expected rescue_from with:, got %v", s.RescueFrom)
	}
	if len(s.Actions) != 0 {
		t.Errorf("expected no public actions (all methods private), got %v", s.Actions)
	}
}

func TestParse_NamespacedAdminReports(t *testing.T) {
	path := testdataRoot + "/app/controllers/admin/reports_controller.rb"
	s, err := controllers.Parse(path, testdataRoot, "app/controllers")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.ClassName != "Admin::ReportsController" {
		t.Errorf("ClassName = %q, want Admin::ReportsController", s.ClassName)
	}
	if s.ParentClass != "Admin::BaseController" {
		t.Errorf("ParentClass = %q, want Admin::BaseController", s.ParentClass)
	}
	if want := []string{"  index", "  show"}; !reflect.DeepEqual(s.Actions, want) {
		t.Errorf("Actions = %#v, want %#v", s.Actions, want)
	}
}

func TestResolve(t *testing.T) {
	t.Run("by short name", func(t *testing.T) {
		path, err := controllers.Resolve(testdataRoot, "app/controllers", "users")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.HasSuffix(filepath.ToSlash(path), "users_controller.rb") {
			t.Errorf("unexpected path: %s", path)
		}
	})

	t.Run("by CamelCase class name", func(t *testing.T) {
		path, err := controllers.Resolve(testdataRoot, "app/controllers", "UsersController")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.HasSuffix(filepath.ToSlash(path), "users_controller.rb") {
			t.Errorf("unexpected path: %s", path)
		}
	})

	t.Run("namespaced by path", func(t *testing.T) {
		path, err := controllers.Resolve(testdataRoot, "app/controllers", "admin/reports")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.HasSuffix(filepath.ToSlash(path), "admin/reports_controller.rb") {
			t.Errorf("unexpected path: %s", path)
		}
	})

	t.Run("namespaced CamelCase class name", func(t *testing.T) {
		path, err := controllers.Resolve(testdataRoot, "app/controllers", "Admin::ReportsController")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.HasSuffix(filepath.ToSlash(path), "admin/reports_controller.rb") {
			t.Errorf("unexpected path: %s", path)
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, err := controllers.Resolve(testdataRoot, "app/controllers", "nonexistent")
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("file without _controller suffix", func(t *testing.T) {
		dir := t.TempDir()
		f := filepath.Join(dir, "app", "controllers", "api", "people.rb")
		if err := os.MkdirAll(filepath.Dir(f), 0o755); err != nil {
			t.Fatalf("setup: %v", err)
		}
		if err := os.WriteFile(f, []byte("module Api::People\nend\n"), 0o644); err != nil {
			t.Fatalf("setup: %v", err)
		}

		got, err := controllers.Resolve(dir, "app/controllers", "api/people")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != f {
			t.Fatalf("got %q, want %q", got, f)
		}
	})

	t.Run("ambiguous basename", func(t *testing.T) {
		extra := filepath.Join(testdataRoot, "app/controllers/reporting/reports_controller.rb")
		if err := os.MkdirAll(filepath.Dir(extra), 0o755); err != nil {
			t.Fatalf("setup: %v", err)
		}
		if err := os.WriteFile(extra, []byte("class Reporting::ReportsController\nend\n"), 0o644); err != nil {
			t.Fatalf("setup: %v", err)
		}
		t.Cleanup(func() {
			_ = os.Remove(extra)
			_ = os.Remove(filepath.Dir(extra))
		})

		_, err := controllers.Resolve(testdataRoot, "app/controllers", "reports")
		if err == nil {
			t.Fatal("expected error for ambiguous controller name")
		}
		if !controllers.IsAmbiguousError(err) {
			t.Errorf("expected ambiguous error, got: %v", err)
		}
	})
}

func TestListNames(t *testing.T) {
	names, err := controllers.ListNames(testdataRoot, "app/controllers", "app/controllers/concerns")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"admin/reports", "application", "users"}
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("got %v, want %v", names, want)
	}
}

func TestListNames_MissingControllersDir(t *testing.T) {
	dir := t.TempDir()
	names, err := controllers.ListNames(dir, "app/controllers", "app/controllers/concerns")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if names != nil {
		t.Fatalf("expected nil names, got %v", names)
	}
}

func TestFormat(t *testing.T) {
	path := testdataRoot + "/app/controllers/users_controller.rb"
	s, err := controllers.Parse(path, testdataRoot, "app/controllers")
	if err != nil {
		t.Fatal(err)
	}
	out := controllers.Format(s, term.Styler{})
	if !strings.Contains(out, "UsersController < ApplicationController (") {
		t.Error("missing class name in output")
	}
	if !strings.Contains(out, "Filters:") {
		t.Error("missing Filters section")
	}
	if !strings.Contains(out, "Strong Params:") {
		t.Error("missing Strong Params section")
	}
}

func TestParse_ReturnsPartialSummaryWithParseDiagnostics(t *testing.T) {
	s := parseTempController(t, "broken_controller.rb", "class Broken < ApplicationController\n  before_action :set_x\n  def index(\nend\n")

	if s.ParentClass != "ApplicationController" || !containsSubstr(s.Filters, "before_action :set_x") {
		t.Fatalf("partial summary = %#v", s)
	}
	if len(s.ParseErrors) == 0 {
		t.Fatal("expected a Prism parse diagnostic")
	}
	if s.ParseErrors[0].Line < 1 || s.ParseErrors[0].Message == "" {
		t.Fatalf("invalid parse diagnostic: %#v", s.ParseErrors[0])
	}
}

func TestParse_OnlyOutermostClass(t *testing.T) {
	content := strings.Join([]string{
		"class Broken < ApplicationController",
		"  def index",
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
	s := parseTempController(t, "broken.rb", content)

	if want := []string{"  index"}; !reflect.DeepEqual(s.Actions, want) {
		t.Fatalf("Actions leaked nested class methods: %#v", s.Actions)
	}
}

func containsSubstr(slice []string, substr string) bool {
	for _, s := range slice {
		if strings.Contains(s, substr) {
			return true
		}
	}
	return false
}

func parseTempController(t *testing.T, relPath, content string) *controllers.Summary {
	t.Helper()
	root := t.TempDir()
	path := filepath.Join(root, "app", "controllers", filepath.FromSlash(relPath))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := controllers.Parse(path, root, "app/controllers")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return s
}
