package mailers_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/janstol/rails-kit/internal/mailers"
	"github.com/janstol/rails-kit/internal/term"
)

const testdataRoot = "../../testdata"

func TestParse_User(t *testing.T) {
	path := testdataRoot + "/app/mailers/user_mailer.rb"
	s, err := mailers.Parse(path, testdataRoot, "app/mailers")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if s.ClassName != "UserMailer" {
		t.Errorf("ClassName = %q, want UserMailer", s.ClassName)
	}
	if s.ParentClass != "ApplicationMailer" {
		t.Errorf("ParentClass = %q, want ApplicationMailer", s.ParentClass)
	}
	if s.Layout != `"mailer"` {
		t.Errorf("Layout = %q, want \"mailer\"", s.Layout)
	}

	if !containsSubstr(s.Default, `from: "noreply@example.com"`) {
		t.Errorf("expected default from:, got %v", s.Default)
	}
	if !containsSubstr(s.Default, `reply_to: "support@example.com"`) {
		t.Errorf("expected default reply_to:, got %v", s.Default)
	}

	if !containsSubstr(s.Concerns, "HeaderFooter") {
		t.Errorf("expected HeaderFooter concern, got %v", s.Concerns)
	}

	if want := []string{"  welcome_email", "  shipment_notification"}; !reflect.DeepEqual(s.Methods, want) {
		t.Errorf("Methods = %#v, want %#v", s.Methods, want)
	}
	// The private `internal_helper` must be excluded from Methods, but its
	// attachment is still collected.
	for _, m := range s.Methods {
		if strings.Contains(m, "internal_helper") {
			t.Errorf("private method leaked into Methods: %q", m)
		}
	}

	if !containsSubstr(s.Attachments, `attachments["invoice.pdf"]`) {
		t.Errorf("expected regular attachment, got %v", s.Attachments)
	}
	if !containsSubstr(s.Attachments, `attachments.inline["logo.png"]`) {
		t.Errorf("expected inline attachment, got %v", s.Attachments)
	}
	if !containsSubstr(s.Attachments, `attachments["secret.txt"]`) {
		t.Errorf("expected attachment from private method, got %v", s.Attachments)
	}
}

func TestParse_NamespacedAdminNotification(t *testing.T) {
	path := testdataRoot + "/app/mailers/admin/notification_mailer.rb"
	s, err := mailers.Parse(path, testdataRoot, "app/mailers")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.ClassName != "Admin::NotificationMailer" {
		t.Errorf("ClassName = %q, want Admin::NotificationMailer", s.ClassName)
	}
	if s.ParentClass != "ApplicationMailer" {
		t.Errorf("ParentClass = %q, want ApplicationMailer", s.ParentClass)
	}
	if want := []string{"  shipment_notification"}; !reflect.DeepEqual(s.Methods, want) {
		t.Errorf("Methods = %#v, want %#v", s.Methods, want)
	}
	if !containsSubstr(s.Default, `to: "admin@example.com"`) {
		t.Errorf("expected default to:, got %v", s.Default)
	}
}

func TestResolve(t *testing.T) {
	t.Run("by short name", func(t *testing.T) {
		path, err := mailers.Resolve(testdataRoot, "app/mailers", "user")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.HasSuffix(filepath.ToSlash(path), "user_mailer.rb") {
			t.Errorf("unexpected path: %s", path)
		}
	})

	t.Run("by CamelCase class name", func(t *testing.T) {
		path, err := mailers.Resolve(testdataRoot, "app/mailers", "UserMailer")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.HasSuffix(filepath.ToSlash(path), "user_mailer.rb") {
			t.Errorf("unexpected path: %s", path)
		}
	})

	t.Run("namespaced by path", func(t *testing.T) {
		path, err := mailers.Resolve(testdataRoot, "app/mailers", "admin/notification")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.HasSuffix(filepath.ToSlash(path), "admin/notification_mailer.rb") {
			t.Errorf("unexpected path: %s", path)
		}
	})

	t.Run("namespaced CamelCase class name", func(t *testing.T) {
		path, err := mailers.Resolve(testdataRoot, "app/mailers", "Admin::NotificationMailer")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.HasSuffix(filepath.ToSlash(path), "admin/notification_mailer.rb") {
			t.Errorf("unexpected path: %s", path)
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, err := mailers.Resolve(testdataRoot, "app/mailers", "nonexistent")
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("file without _mailer suffix", func(t *testing.T) {
		dir := t.TempDir()
		f := filepath.Join(dir, "app", "mailers", "api", "digest.rb")
		if err := os.MkdirAll(filepath.Dir(f), 0o755); err != nil {
			t.Fatalf("setup: %v", err)
		}
		if err := os.WriteFile(f, []byte("module Api::Digest\nend\n"), 0o644); err != nil {
			t.Fatalf("setup: %v", err)
		}

		got, err := mailers.Resolve(dir, "app/mailers", "api/digest")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != f {
			t.Fatalf("got %q, want %q", got, f)
		}
	})

	t.Run("ambiguous basename", func(t *testing.T) {
		extra := filepath.Join(testdataRoot, "app/mailers/reporting/notification_mailer.rb")
		if err := os.MkdirAll(filepath.Dir(extra), 0o755); err != nil {
			t.Fatalf("setup: %v", err)
		}
		if err := os.WriteFile(extra, []byte("class Reporting::NotificationMailer\nend\n"), 0o644); err != nil {
			t.Fatalf("setup: %v", err)
		}
		t.Cleanup(func() {
			_ = os.Remove(extra)
			_ = os.Remove(filepath.Dir(extra))
		})

		_, err := mailers.Resolve(testdataRoot, "app/mailers", "notification")
		if err == nil {
			t.Fatal("expected error for ambiguous mailer name")
		}
		if !mailers.IsAmbiguousError(err) {
			t.Errorf("expected ambiguous error, got: %v", err)
		}
	})
}

func TestListNames(t *testing.T) {
	names, err := mailers.ListNames(testdataRoot, "app/mailers")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"admin/notification", "application", "user"}
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("got %v, want %v", names, want)
	}
}

func TestListNames_MissingMailersDir(t *testing.T) {
	dir := t.TempDir()
	names, err := mailers.ListNames(dir, "app/mailers")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if names != nil {
		t.Fatalf("expected nil names, got %v", names)
	}
}

func TestFormat(t *testing.T) {
	path := testdataRoot + "/app/mailers/user_mailer.rb"
	s, err := mailers.Parse(path, testdataRoot, "app/mailers")
	if err != nil {
		t.Fatal(err)
	}
	out := mailers.Format(s, term.Styler{})
	if !strings.Contains(out, "UserMailer < ApplicationMailer (") {
		t.Error("missing class name in output")
	}
	if !strings.Contains(out, "Default:") {
		t.Error("missing Default section")
	}
	if !strings.Contains(out, "Mailer Methods:") {
		t.Error("missing Mailer Methods section")
	}
	if !strings.Contains(out, "Attachments:") {
		t.Error("missing Attachments section")
	}
}

func TestParse_ReturnsPartialSummaryWithParseDiagnostics(t *testing.T) {
	s := parseTempMailer(t, "broken_mailer.rb", "class Broken < ApplicationMailer\n  default from: \"x\"\n  def index(\nend\n")

	if s.ParentClass != "ApplicationMailer" || !containsSubstr(s.Default, `from: "x"`) {
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
		"class Broken < ApplicationMailer",
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
	s := parseTempMailer(t, "broken.rb", content)

	if want := []string{"  index"}; !reflect.DeepEqual(s.Methods, want) {
		t.Fatalf("Methods leaked nested class methods: %#v", s.Methods)
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

func parseTempMailer(t *testing.T, relPath, content string) *mailers.Summary {
	t.Helper()
	root := t.TempDir()
	path := filepath.Join(root, "app", "mailers", filepath.FromSlash(relPath))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := mailers.Parse(path, root, "app/mailers")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return s
}
