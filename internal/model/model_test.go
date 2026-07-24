package model_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/janstol/rails-kit/internal/model"
)

const testdataRoot = "../../testdata"

func TestParse_User(t *testing.T) {
	path := testdataRoot + "/app/models/user.rb"
	s, err := model.Parse(path, testdataRoot, "app/models")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if s.ClassName != "User" {
		t.Errorf("ClassName = %q, want User", s.ClassName)
	}
	if s.ParentClass != "ApplicationRecord" {
		t.Errorf("ParentClass = %q, want ApplicationRecord", s.ParentClass)
	}

	// Concerns
	if !containsSubstr(s.Concerns, "Searchable") {
		t.Errorf("expected Searchable concern, got %v", s.Concerns)
	}

	// Associations
	if !containsSubstr(s.Assocs, "has_many :posts") {
		t.Errorf("expected has_many :posts, got %v", s.Assocs)
	}
	if !containsSubstr(s.Assocs, "dependent: destroy") {
		t.Errorf("expected dependent: destroy, got %v", s.Assocs)
	}
	if !containsSubstr(s.Assocs, "through: posts") {
		t.Errorf("expected through: posts, got %v", s.Assocs)
	}
	if !containsSubstr(s.Assocs, "class_name: Post") {
		t.Errorf("expected class_name: Post, got %v", s.Assocs)
	}

	// Validations
	if !containsSubstr(s.Valids, "validates :email") {
		t.Errorf("expected validates :email, got %v", s.Valids)
	}
	if !containsSubstr(s.Valids, "presence") {
		t.Errorf("expected presence validation, got %v", s.Valids)
	}
	if !containsSubstr(s.Valids, "format") {
		t.Errorf("expected format validation, got %v", s.Valids)
	}

	// Scopes
	if !containsSubstr(s.Scopes, "active") {
		t.Errorf("expected active scope, got %v", s.Scopes)
	}
	if !containsSubstr(s.Scopes, "by_name(") {
		t.Errorf("expected by_name scope with args, got %v", s.Scopes)
	}

	// Callbacks
	if !containsSubstr(s.Callbacks, "before_validation") {
		t.Errorf("expected before_validation callback, got %v", s.Callbacks)
	}
	if !containsSubstr(s.Callbacks, "after_commit") {
		t.Errorf("expected after_commit callback, got %v", s.Callbacks)
	}
	if !containsSubstr(s.Callbacks, "after_touch") {
		t.Errorf("expected after_touch callback, got %v", s.Callbacks)
	}
	if !containsSubstr(s.Callbacks, "after_create_commit") {
		t.Errorf("expected after_create_commit callback, got %v", s.Callbacks)
	}

	// Enums
	if !containsSubstr(s.Enums, "role") {
		t.Errorf("expected role enum, got %v", s.Enums)
	}

	// Delegates
	if !containsSubstr(s.Delegates, "delegate") {
		t.Errorf("expected delegate, got %v", s.Delegates)
	}
}

func TestParse_Post(t *testing.T) {
	path := testdataRoot + "/app/models/post.rb"
	s, err := model.Parse(path, testdataRoot, "app/models")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.ClassName != "Post" {
		t.Errorf("ClassName = %q, want Post", s.ClassName)
	}
	if !containsSubstr(s.Assocs, "belongs_to :user") {
		t.Errorf("expected belongs_to :user, got %v", s.Assocs)
	}
}

func TestParse_Dashboard(t *testing.T) {
	path := testdataRoot + "/app/models/admin/dashboard.rb"
	s, err := model.Parse(path, testdataRoot, "app/models")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.ClassName != "Admin::Dashboard" {
		t.Errorf("ClassName = %q, want Admin::Dashboard", s.ClassName)
	}
}

func TestResolve(t *testing.T) {
	t.Run("by name", func(t *testing.T) {
		path, err := model.Resolve(testdataRoot, "app/models", "user")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.HasSuffix(path, "user.rb") {
			t.Errorf("unexpected path: %s", path)
		}
	})

	t.Run("by CamelCase class name", func(t *testing.T) {
		path, err := model.Resolve(testdataRoot, "app/models", "Post")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.HasSuffix(path, "post.rb") {
			t.Errorf("unexpected path: %s", path)
		}
	})

	t.Run("namespaced CamelCase class name", func(t *testing.T) {
		path, err := model.Resolve(testdataRoot, "app/models", "Admin::Dashboard")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.HasSuffix(filepath.ToSlash(path), "admin/dashboard.rb") {
			t.Errorf("unexpected path: %s", path)
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, err := model.Resolve(testdataRoot, "app/models", "nonexistent")
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("nested model by namespace", func(t *testing.T) {
		path, err := model.Resolve(testdataRoot, "app/models", "admin/dashboard")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.HasSuffix(path, "admin/dashboard.rb") {
			t.Errorf("unexpected path: %s", path)
		}
	})

	t.Run("nested model with backslashes", func(t *testing.T) {
		path, err := model.Resolve(testdataRoot, "app/models", `admin\dashboard`)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.HasSuffix(filepath.ToSlash(path), "admin/dashboard.rb") {
			t.Errorf("unexpected path: %s", path)
		}
	})

	t.Run("ambiguous basename", func(t *testing.T) {
		// Create a second dashboard temporarily to force ambiguity.
		extra := filepath.Join(testdataRoot, "app/models/analytics/dashboard.rb")
		if err := os.MkdirAll(filepath.Dir(extra), 0o755); err != nil {
			t.Fatalf("setup: %v", err)
		}
		if err := os.WriteFile(extra, []byte("class Analytics::Dashboard\nend\n"), 0o644); err != nil {
			t.Fatalf("setup: %v", err)
		}
		t.Cleanup(func() {
			_ = os.Remove(extra)
			_ = os.Remove(filepath.Dir(extra))
		})

		_, err := model.Resolve(testdataRoot, "app/models", "dashboard")
		if err == nil {
			t.Error("expected error for ambiguous model name")
		}
		if !strings.Contains(err.Error(), "ambiguous") {
			t.Errorf("expected 'ambiguous' in error, got: %v", err)
		}
	})
}

func TestResolve_PrefersExactNamespaceOverBasenameFallback(t *testing.T) {
	dir := t.TempDir()
	admin := filepath.Join(dir, "app", "models", "admin", "dashboard.rb")
	analytics := filepath.Join(dir, "app", "models", "analytics", "dashboard.rb")
	for _, path := range []string{admin, analytics} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("setup: %v", err)
		}
	}
	if err := os.WriteFile(admin, []byte("class Admin::Dashboard\nend\n"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.WriteFile(analytics, []byte("class Analytics::Dashboard\nend\n"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	got, err := model.Resolve(dir, "app/models", `admin\dashboard`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != admin {
		t.Fatalf("got %q, want %q", got, admin)
	}
}

func TestFormat(t *testing.T) {
	path := testdataRoot + "/app/models/user.rb"
	s, err := model.Parse(path, testdataRoot, "app/models")
	if err != nil {
		t.Fatal(err)
	}
	out := model.Format(s)
	if !strings.Contains(out, "User < ApplicationRecord (") {
		t.Error("missing class name in output")
	}
	if !strings.Contains(out, "Associations:") {
		t.Error("missing Associations section")
	}
	if !strings.Contains(out, "Validations:") {
		t.Error("missing Validations section")
	}
}

func TestParse_CustomTableNameAndValidationOptions(t *testing.T) {
	dir := t.TempDir()
	modelFile := filepath.Join(dir, "app", "models", "admin", "report.rb")
	if err := os.MkdirAll(filepath.Dir(modelFile), 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	content := strings.Join([]string{
		"class Admin::Report < ApplicationRecord",
		`  self.table_name = "legacy_reports"`,
		"  belongs_to :account, optional: true, inverse_of: :reports",
		"  validates :published_at, presence: true, allow_nil: true, on: :create",
		"end",
		"",
	}, "\n")
	if err := os.WriteFile(modelFile, []byte(content), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	s, err := model.Parse(modelFile, dir, "app/models")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.ClassName != "Admin::Report" {
		t.Fatalf("ClassName = %q, want Admin::Report", s.ClassName)
	}
	if s.ParentClass != "ApplicationRecord" {
		t.Fatalf("ParentClass = %q, want ApplicationRecord", s.ParentClass)
	}
	if s.TableName != "legacy_reports" {
		t.Fatalf("TableName = %q, want legacy_reports", s.TableName)
	}
	if !containsSubstr(s.Assocs, "optional: true") {
		t.Fatalf("expected optional association, got %v", s.Assocs)
	}
	if !containsSubstr(s.Assocs, "inverse_of: reports") {
		t.Fatalf("expected inverse_of association option, got %v", s.Assocs)
	}
	if !containsSubstr(s.Valids, "allow_nil") || !containsSubstr(s.Valids, "on: create") {
		t.Fatalf("expected validation modifiers, got %v", s.Valids)
	}
}

func TestParse_CustomModelsPath(t *testing.T) {
	dir := t.TempDir()
	modelFile := filepath.Join(dir, "lib", "models", "admin", "widget.rb")
	if err := os.MkdirAll(filepath.Dir(modelFile), 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.WriteFile(modelFile, []byte("class Admin::Widget\nend\n"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	s, err := model.Parse(modelFile, dir, "lib/models")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.ClassName != "Admin::Widget" {
		t.Errorf("ClassName = %q, want Admin::Widget", s.ClassName)
	}
}

func TestResolve_RbOutsideModelsDir(t *testing.T) {
	dir := t.TempDir()
	outsideFile := filepath.Join(t.TempDir(), "sneaky.rb")
	if err := os.WriteFile(outsideFile, []byte("class Sneaky\nend\n"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	_, err := model.Resolve(dir, "app/models", outsideFile)
	if err == nil {
		t.Fatal("expected error for path outside models directory")
	}
	if !strings.Contains(err.Error(), "outside models directory") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestResolve_RbInsideModelsDir(t *testing.T) {
	dir := t.TempDir()
	modelFile := filepath.Join(dir, "app", "models", "thing.rb")
	if err := os.MkdirAll(filepath.Dir(modelFile), 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.WriteFile(modelFile, []byte("class Thing\nend\n"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	got, err := model.Resolve(dir, "app/models", modelFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != modelFile {
		t.Errorf("got %q, want %q", got, modelFile)
	}
}

func TestResolve_MissingModelsPath(t *testing.T) {
	dir := t.TempDir()

	_, err := model.Resolve(dir, "lib/missing_models", "user")
	if err == nil {
		t.Fatal("expected error for missing models path")
	}
	if !strings.Contains(err.Error(), "models path") {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(err.Error(), "lib/missing_models") {
		t.Fatalf("missing configured path in error: %v", err)
	}
}

func TestResolve_AbsoluteModelsPath(t *testing.T) {
	dir := t.TempDir()
	modelsDir := filepath.Join(dir, "shared_models")
	modelFile := filepath.Join(modelsDir, "user.rb")
	if err := os.MkdirAll(modelsDir, 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.WriteFile(modelFile, []byte("class User\nend\n"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	got, err := model.Resolve(dir, modelsDir, "user")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != modelFile {
		t.Fatalf("got %q, want %q", got, modelFile)
	}

	summary, err := model.Parse(modelFile, dir, modelsDir)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if summary.ClassName != "User" {
		t.Fatalf("ClassName = %q, want User", summary.ClassName)
	}
	if summary.RelPath != filepath.Join("shared_models", "user.rb") {
		t.Fatalf("RelPath = %q, want %q", summary.RelPath, filepath.Join("shared_models", "user.rb"))
	}
}

func TestResolve_RelativeRbWithCustomModelsPath(t *testing.T) {
	dir := t.TempDir()
	modelFile := filepath.Join(dir, "lib", "models", "user.rb")
	if err := os.MkdirAll(filepath.Dir(modelFile), 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.WriteFile(modelFile, []byte("class User\nend\n"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	got, err := model.Resolve(dir, "lib/models", "user.rb")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != modelFile {
		t.Errorf("got %q, want %q", got, modelFile)
	}

	// Nested case
	nestedFile := filepath.Join(dir, "lib", "models", "admin", "dashboard.rb")
	if err := os.MkdirAll(filepath.Dir(nestedFile), 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.WriteFile(nestedFile, []byte("class Admin::Dashboard\nend\n"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	got, err = model.Resolve(dir, "lib/models", "admin/dashboard.rb")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nestedFile {
		t.Errorf("got %q, want %q", got, nestedFile)
	}
}

func TestResolve_AbsoluteModelsPathWithPrefix(t *testing.T) {
	dir := t.TempDir()
	modelsDir := filepath.Join(dir, "external", "models")
	modelFile := filepath.Join(modelsDir, "user.rb")
	if err := os.MkdirAll(modelsDir, 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.WriteFile(modelFile, []byte("class User\nend\n"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	// Passing "app/models/user.rb" with an absolute modelsPath should not double the prefix.
	got, err := model.Resolve(dir, modelsDir, "app/models/user.rb")
	if err == nil {
		// This path doesn't exist under the absolute modelsDir, so it should fail.
		t.Fatalf("expected error, got path: %s", got)
	}

	// Passing just "user.rb" should work fine.
	got, err = model.Resolve(dir, modelsDir, "user.rb")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != modelFile {
		t.Fatalf("got %q, want %q", got, modelFile)
	}
}

func TestResolve_RelativeModelsPathWithPrefix(t *testing.T) {
	dir := t.TempDir()
	modelFile := filepath.Join(dir, "app", "models", "user.rb")
	if err := os.MkdirAll(filepath.Dir(modelFile), 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.WriteFile(modelFile, []byte("class User\nend\n"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	// Passing "app/models/user.rb" with a relative modelsPath should not double the prefix.
	got, err := model.Resolve(dir, "app/models", "app/models/user.rb")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != modelFile {
		t.Fatalf("got %q, want %q", got, modelFile)
	}
}

func TestResolve_CamelCaseMultiWordName(t *testing.T) {
	dir := t.TempDir()
	modelFile := filepath.Join(dir, "app", "models", "s3_bucket_archive_policy.rb")
	if err := os.MkdirAll(filepath.Dir(modelFile), 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.WriteFile(modelFile, []byte("class S3BucketArchivePolicy\nend\n"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	got, err := model.Resolve(dir, "app/models", "S3BucketArchivePolicy")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != modelFile {
		t.Fatalf("got %q, want %q", got, modelFile)
	}
}

func TestResolve_CamelCaseOrderItem(t *testing.T) {
	dir := t.TempDir()
	modelFile := filepath.Join(dir, "app", "models", "order_item.rb")
	if err := os.MkdirAll(filepath.Dir(modelFile), 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.WriteFile(modelFile, []byte("class OrderItem\nend\n"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	got, err := model.Resolve(dir, "app/models", "OrderItem")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != modelFile {
		t.Fatalf("got %q, want %q", got, modelFile)
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
