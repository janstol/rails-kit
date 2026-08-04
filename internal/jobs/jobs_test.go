package jobs_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/janstol/rails-kit/internal/jobs"
	"github.com/janstol/rails-kit/internal/term"
)

const testdataRoot = "../../testdata"

func TestParse_SyncUser(t *testing.T) {
	path := testdataRoot + "/app/jobs/sync_user_job.rb"
	s, err := jobs.Parse(path, testdataRoot, "app/jobs")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if s.ClassName != "SyncUserJob" {
		t.Errorf("ClassName = %q, want SyncUserJob", s.ClassName)
	}
	if s.ParentClass != "ApplicationJob" {
		t.Errorf("ParentClass = %q, want ApplicationJob", s.ParentClass)
	}
	if s.Queue != ":default" {
		t.Errorf("Queue = %q, want :default", s.Queue)
	}

	if want := []string{"  retry_on StandardError, wait: 5.seconds, attempts: 3"}; !reflect.DeepEqual(s.RetryOn, want) {
		t.Errorf("RetryOn = %#v, want %#v", s.RetryOn, want)
	}
	if want := []string{"  discard_on ActiveJob::DeserializationError"}; !reflect.DeepEqual(s.DiscardOn, want) {
		t.Errorf("DiscardOn = %#v, want %#v", s.DiscardOn, want)
	}

	if !containsSubstr(s.Concerns, "Retryable") {
		t.Errorf("expected Retryable concern, got %v", s.Concerns)
	}

	if want := []string{"  perform"}; !reflect.DeepEqual(s.Methods, want) {
		t.Errorf("Methods = %#v, want %#v", s.Methods, want)
	}
	// The private `cleanup` must be excluded from Methods.
	for _, m := range s.Methods {
		if strings.Contains(m, "cleanup") {
			t.Errorf("private method leaked into Methods: %q", m)
		}
	}
}

func TestParse_NamespacedAdminExport(t *testing.T) {
	path := testdataRoot + "/app/jobs/admin/export_job.rb"
	s, err := jobs.Parse(path, testdataRoot, "app/jobs")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.ClassName != "Admin::ExportJob" {
		t.Errorf("ClassName = %q, want Admin::ExportJob", s.ClassName)
	}
	if s.ParentClass != "ApplicationJob" {
		t.Errorf("ParentClass = %q, want ApplicationJob", s.ParentClass)
	}
	if s.Queue != ":low" {
		t.Errorf("Queue = %q, want :low", s.Queue)
	}
	if want := []string{"  perform"}; !reflect.DeepEqual(s.Methods, want) {
		t.Errorf("Methods = %#v, want %#v", s.Methods, want)
	}
}

func TestResolve(t *testing.T) {
	t.Run("by short name", func(t *testing.T) {
		path, err := jobs.Resolve(testdataRoot, "app/jobs", "sync_user")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.HasSuffix(filepath.ToSlash(path), "sync_user_job.rb") {
			t.Errorf("unexpected path: %s", path)
		}
	})

	t.Run("by CamelCase class name", func(t *testing.T) {
		path, err := jobs.Resolve(testdataRoot, "app/jobs", "SyncUserJob")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.HasSuffix(filepath.ToSlash(path), "sync_user_job.rb") {
			t.Errorf("unexpected path: %s", path)
		}
	})

	t.Run("namespaced by path", func(t *testing.T) {
		path, err := jobs.Resolve(testdataRoot, "app/jobs", "admin/export")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.HasSuffix(filepath.ToSlash(path), "admin/export_job.rb") {
			t.Errorf("unexpected path: %s", path)
		}
	})

	t.Run("namespaced CamelCase class name", func(t *testing.T) {
		path, err := jobs.Resolve(testdataRoot, "app/jobs", "Admin::ExportJob")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.HasSuffix(filepath.ToSlash(path), "admin/export_job.rb") {
			t.Errorf("unexpected path: %s", path)
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, err := jobs.Resolve(testdataRoot, "app/jobs", "nonexistent")
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("file without _job suffix", func(t *testing.T) {
		dir := t.TempDir()
		f := filepath.Join(dir, "app", "jobs", "api", "digest.rb")
		if err := os.MkdirAll(filepath.Dir(f), 0o755); err != nil {
			t.Fatalf("setup: %v", err)
		}
		if err := os.WriteFile(f, []byte("module Api::Digest\nend\n"), 0o644); err != nil {
			t.Fatalf("setup: %v", err)
		}

		got, err := jobs.Resolve(dir, "app/jobs", "api/digest")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != f {
			t.Fatalf("got %q, want %q", got, f)
		}
	})

	t.Run("ambiguous basename", func(t *testing.T) {
		extra := filepath.Join(testdataRoot, "app/jobs/reporting/export_job.rb")
		if err := os.MkdirAll(filepath.Dir(extra), 0o755); err != nil {
			t.Fatalf("setup: %v", err)
		}
		if err := os.WriteFile(extra, []byte("class Reporting::ExportJob\nend\n"), 0o644); err != nil {
			t.Fatalf("setup: %v", err)
		}
		t.Cleanup(func() {
			_ = os.Remove(extra)
			_ = os.Remove(filepath.Dir(extra))
		})

		_, err := jobs.Resolve(testdataRoot, "app/jobs", "export")
		if err == nil {
			t.Fatal("expected error for ambiguous job name")
		}
		if !jobs.IsAmbiguousError(err) {
			t.Errorf("expected ambiguous error, got: %v", err)
		}
	})
}

func TestListNames(t *testing.T) {
	names, err := jobs.ListNames(testdataRoot, "app/jobs")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"admin/export", "application", "sync_user"}
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("got %v, want %v", names, want)
	}
}

func TestListNames_MissingJobsDir(t *testing.T) {
	dir := t.TempDir()
	names, err := jobs.ListNames(dir, "app/jobs")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if names != nil {
		t.Fatalf("expected nil names, got %v", names)
	}
}

func TestFormat(t *testing.T) {
	path := testdataRoot + "/app/jobs/sync_user_job.rb"
	s, err := jobs.Parse(path, testdataRoot, "app/jobs")
	if err != nil {
		t.Fatal(err)
	}
	out := jobs.Format(s, term.Styler{})
	if !strings.Contains(out, "SyncUserJob < ApplicationJob (") {
		t.Error("missing class name in output")
	}
	if !strings.Contains(out, "Queue:") {
		t.Error("missing Queue section")
	}
	if !strings.Contains(out, "Retry On:") {
		t.Error("missing Retry On section")
	}
	if !strings.Contains(out, "Discard On:") {
		t.Error("missing Discard On section")
	}
	if !strings.Contains(out, "Job Methods:") {
		t.Error("missing Job Methods section")
	}
}

func TestParse_ReturnsPartialSummaryWithParseDiagnostics(t *testing.T) {
	s := parseTempJob(t, "broken_job.rb", "class Broken < ApplicationJob\n  queue_as :default\n  def perform(\nend\n")

	if s.ParentClass != "ApplicationJob" || s.Queue != ":default" {
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
		"class Broken < ApplicationJob",
		"  def perform",
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
	s := parseTempJob(t, "broken.rb", content)

	if want := []string{"  perform"}; !reflect.DeepEqual(s.Methods, want) {
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

func parseTempJob(t *testing.T, relPath, content string) *jobs.Summary {
	t.Helper()
	root := t.TempDir()
	path := filepath.Join(root, "app", "jobs", filepath.FromSlash(relPath))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := jobs.Parse(path, root, "app/jobs")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return s
}
