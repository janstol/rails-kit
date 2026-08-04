package services_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/janstol/rails-kit/internal/services"
	"github.com/janstol/rails-kit/internal/term"
)

const testdataRoot = "../../testdata"

func TestParse_UserExport(t *testing.T) {
	path := testdataRoot + "/app/services/user_export_service.rb"
	s, err := services.Parse(path, testdataRoot, "app/services")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if s.ClassName != "UserExportService" {
		t.Errorf("ClassName = %q, want UserExportService", s.ClassName)
	}
	if s.Kind != "class" {
		t.Errorf("Kind = %q, want class", s.Kind)
	}
	if s.ParentClass != "" {
		t.Errorf("ParentClass = %q, want empty", s.ParentClass)
	}

	if want := []string{"  DEFAULT_LIMIT = 100"}; !reflect.DeepEqual(s.Constants, want) {
		t.Errorf("Constants = %#v, want %#v", s.Constants, want)
	}
	if want := []string{"  Searchable"}; !reflect.DeepEqual(s.Concerns, want) {
		t.Errorf("Concerns = %#v, want %#v", s.Concerns, want)
	}
	if want := []string{"  initialize", "  call"}; !reflect.DeepEqual(s.Methods, want) {
		t.Errorf("Methods = %#v, want %#v", s.Methods, want)
	}
	// The private `export` must be excluded from Methods.
	for _, m := range s.Methods {
		if strings.Contains(m, "export") {
			t.Errorf("private method leaked into Methods: %q", m)
		}
	}
}

func TestParse_ModuleNotification(t *testing.T) {
	path := testdataRoot + "/app/services/notification_service.rb"
	s, err := services.Parse(path, testdataRoot, "app/services")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if s.ClassName != "NotificationService" {
		t.Errorf("ClassName = %q, want NotificationService", s.ClassName)
	}
	if s.Kind != "module" {
		t.Errorf("Kind = %q, want module", s.Kind)
	}
	if s.ParentClass != "" {
		t.Errorf("ParentClass = %q, want empty for a module", s.ParentClass)
	}

	if want := []string{"  DEFAULT_CHANNEL = :email"}; !reflect.DeepEqual(s.Constants, want) {
		t.Errorf("Constants = %#v, want %#v", s.Constants, want)
	}
	if want := []string{"  Loggable"}; !reflect.DeepEqual(s.Concerns, want) {
		t.Errorf("Concerns = %#v, want %#v", s.Concerns, want)
	}
	// Singleton `def self.call` and `def self.format` are collected regardless
	// of visibility.
	if want := []string{"  call", "  format"}; !reflect.DeepEqual(s.Methods, want) {
		t.Errorf("Methods = %#v, want %#v", s.Methods, want)
	}
}

func TestParse_NamespacedAdminBilling(t *testing.T) {
	path := testdataRoot + "/app/services/admin/billing_service.rb"
	s, err := services.Parse(path, testdataRoot, "app/services")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.ClassName != "Admin::BillingService" {
		t.Errorf("ClassName = %q, want Admin::BillingService", s.ClassName)
	}
	if s.Kind != "class" {
		t.Errorf("Kind = %q, want class", s.Kind)
	}
	if s.ParentClass != "BaseService" {
		t.Errorf("ParentClass = %q, want BaseService", s.ParentClass)
	}
	if want := []string{"  DEFAULT_CURRENCY = \"USD\""}; !reflect.DeepEqual(s.Constants, want) {
		t.Errorf("Constants = %#v, want %#v", s.Constants, want)
	}
	if want := []string{"  Billable"}; !reflect.DeepEqual(s.Concerns, want) {
		t.Errorf("Concerns = %#v, want %#v", s.Concerns, want)
	}
	if want := []string{"  call"}; !reflect.DeepEqual(s.Methods, want) {
		t.Errorf("Methods = %#v, want %#v", s.Methods, want)
	}
	// The private `charge` must be excluded.
	for _, m := range s.Methods {
		if strings.Contains(m, "charge") {
			t.Errorf("private method leaked into Methods: %q", m)
		}
	}
}

func TestResolve(t *testing.T) {
	t.Run("by resource name", func(t *testing.T) {
		path, err := services.Resolve(testdataRoot, "app/services", "user_export_service")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.HasSuffix(filepath.ToSlash(path), "app/services/user_export_service.rb") {
			t.Errorf("unexpected path: %s", path)
		}
	})

	t.Run("by basename", func(t *testing.T) {
		path, err := services.Resolve(testdataRoot, "app/services", "billing_service")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.HasSuffix(filepath.ToSlash(path), "app/services/admin/billing_service.rb") {
			t.Errorf("unexpected path: %s", path)
		}
	})

	t.Run("by CamelCase class name", func(t *testing.T) {
		path, err := services.Resolve(testdataRoot, "app/services", "UserExportService")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.HasSuffix(filepath.ToSlash(path), "app/services/user_export_service.rb") {
			t.Errorf("unexpected path: %s", path)
		}
	})

	t.Run("namespaced by path", func(t *testing.T) {
		path, err := services.Resolve(testdataRoot, "app/services", "admin/billing_service")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.HasSuffix(filepath.ToSlash(path), "app/services/admin/billing_service.rb") {
			t.Errorf("unexpected path: %s", path)
		}
	})

	t.Run("namespaced CamelCase class name", func(t *testing.T) {
		path, err := services.Resolve(testdataRoot, "app/services", "Admin::BillingService")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.HasSuffix(filepath.ToSlash(path), "app/services/admin/billing_service.rb") {
			t.Errorf("unexpected path: %s", path)
		}
	})

	t.Run("by .rb path", func(t *testing.T) {
		path, err := services.Resolve(testdataRoot, "app/services", "app/services/user_export_service.rb")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.HasSuffix(filepath.ToSlash(path), "app/services/user_export_service.rb") {
			t.Errorf("unexpected path: %s", path)
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, err := services.Resolve(testdataRoot, "app/services", "nonexistent")
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("ambiguous basename", func(t *testing.T) {
		extra := filepath.Join(testdataRoot, "app/services", "billing", "billing_service.rb")
		if err := os.MkdirAll(filepath.Dir(extra), 0o755); err != nil {
			t.Fatalf("setup: %v", err)
		}
		if err := os.WriteFile(extra, []byte("class Billing::BillingService\nend\n"), 0o644); err != nil {
			t.Fatalf("setup: %v", err)
		}
		t.Cleanup(func() {
			_ = os.Remove(extra)
			_ = os.Remove(filepath.Dir(extra))
		})

		_, err := services.Resolve(testdataRoot, "app/services", "billing_service")
		if err == nil {
			t.Fatal("expected error for ambiguous service name")
		}
		if !services.IsAmbiguousError(err) {
			t.Errorf("expected ambiguous error, got: %v", err)
		}
	})

	t.Run("outside services dir", func(t *testing.T) {
		dir := t.TempDir()
		f := filepath.Join(dir, "elsewhere.rb")
		if err := os.WriteFile(f, []byte("class Elsewhere\nend\n"), 0o644); err != nil {
			t.Fatalf("setup: %v", err)
		}
		_, err := services.Resolve(testdataRoot, "app/services", f)
		if err == nil {
			t.Fatal("expected error for file outside services directory")
		}
	})
}

func TestListNames(t *testing.T) {
	names, err := services.ListNames(testdataRoot, "app/services")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"admin/billing_service", "notification_service", "user_export_service"}
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("got %v, want %v", names, want)
	}
}

func TestListNames_MissingServicesDir(t *testing.T) {
	dir := t.TempDir()
	names, err := services.ListNames(dir, "app/services")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if names != nil {
		t.Fatalf("expected nil names, got %v", names)
	}
}

func TestFormat(t *testing.T) {
	t.Run("class", func(t *testing.T) {
		path := testdataRoot + "/app/services/admin/billing_service.rb"
		s, err := services.Parse(path, testdataRoot, "app/services")
		if err != nil {
			t.Fatal(err)
		}
		out := services.Format(s, term.Styler{})
		if !strings.Contains(out, "Admin::BillingService < BaseService (") {
			t.Error("missing class header in output")
		}
		if !strings.Contains(out, "Constants:") {
			t.Error("missing Constants section")
		}
		if !strings.Contains(out, "Concerns:") {
			t.Error("missing Concerns section")
		}
		if !strings.Contains(out, "Methods:") {
			t.Error("missing Methods section")
		}
	})

	t.Run("module", func(t *testing.T) {
		path := testdataRoot + "/app/services/notification_service.rb"
		s, err := services.Parse(path, testdataRoot, "app/services")
		if err != nil {
			t.Fatal(err)
		}
		out := services.Format(s, term.Styler{})
		if !strings.Contains(out, "module NotificationService (") {
			t.Error("missing module header in output")
		}
		if strings.Contains(out, " < ") {
			t.Errorf("module output should not show a parent class: %s", out)
		}
	})
}

func TestParse_ReturnsPartialSummaryWithParseDiagnostics(t *testing.T) {
	s := parseTempService(t, "broken_service.rb", "class Broken < BaseService\n  DEFAULT = 1\n  def call(\nend\n")

	if s.ParentClass != "BaseService" {
		t.Fatalf("ParentClass = %q, want BaseService", s.ParentClass)
	}
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

func TestParse_OnlyOutermostClass(t *testing.T) {
	content := strings.Join([]string{
		"class Broken < BaseService",
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
	s := parseTempService(t, "broken.rb", content)

	if want := []string{"  call"}; !reflect.DeepEqual(s.Methods, want) {
		t.Fatalf("Methods leaked nested class methods: %#v", s.Methods)
	}
}

func TestParse_ModuleWithoutNestedClassIsRecognized(t *testing.T) {
	content := strings.Join([]string{
		"module Plain",
		"  VALUE = 1",
		"  def self.call",
		"  end",
		"end",
		"",
	}, "\n")
	s := parseTempService(t, "plain.rb", content)

	if s.Kind != "module" {
		t.Fatalf("Kind = %q, want module", s.Kind)
	}
	if s.ClassName != "Plain" {
		t.Fatalf("ClassName = %q, want Plain", s.ClassName)
	}
	if want := []string{"  call"}; !reflect.DeepEqual(s.Methods, want) {
		t.Fatalf("Methods = %#v, want %#v", s.Methods, want)
	}
}

func parseTempService(t *testing.T, relPath, content string) *services.Summary {
	t.Helper()
	root := t.TempDir()
	path := filepath.Join(root, "app", "services", filepath.FromSlash(relPath))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := services.Parse(path, root, "app/services")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return s
}
