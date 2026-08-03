package model_test

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/janstol/rails-kit/internal/model"
	"github.com/janstol/rails-kit/internal/term"
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
		if !strings.HasSuffix(filepath.ToSlash(path), "admin/dashboard.rb") {
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
	out := model.Format(s, term.Styler{})
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

var reANSI = regexp.MustCompile("\x1b\\[[0-9;]*m")

func TestFormat_Colored(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	path := testdataRoot + "/app/models/user.rb"
	s, err := model.Parse(path, testdataRoot, "app/models")
	if err != nil {
		t.Fatal(err)
	}

	uncolored := model.Format(s, term.Styler{})
	colored := model.Format(s, term.NewStyler(term.ModeAlways, nil))

	if !strings.Contains(colored, "\x1b[1mUser\x1b[0m") {
		t.Errorf("expected bold class name in colored output:\n%s", colored)
	}
	if !strings.Contains(colored, "\x1b[1mAssociations:\x1b[0m") {
		t.Errorf("expected bold Associations label in colored output:\n%s", colored)
	}
	if got := reANSI.ReplaceAllString(colored, ""); got != uncolored {
		t.Errorf("stripping ANSI from colored output does not reproduce uncolored output:\n--- got ---\n%s\n--- want ---\n%s", got, uncolored)
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

func TestParse_PrismDSLCompatibility(t *testing.T) {
	content := strings.Join([]string{
		"class Admin::Report < ApplicationRecord",
		"  include Searchable",
		"  include ActiveSupport::Configurable",
		`  self.table_name = "legacy_reports"`,
		"  has_many :entries, source: :items, inverse_of: :report, optional: true, dependent: :destroy, polymorphic: true, class_name: \"Entry\", through: :account",
		"  validates :name, :slug, presence: true, uniqueness: false, length: { minimum: 2 }, format: /x/, numericality: false, inclusion: { in: [] }, exclusion: { in: [] }, confirmation: true, email_format: false, allow_nil: true, allow_blank: true, on: :create",
		"  validates_presence_of :code",
		"  validate :internally_consistent",
		"  scope :active, -> { all }",
		"  scope :by_name, ->(name) { where(name: name) }",
		"  scope :by_pair, ->(left, right) { where(left: left, right: right) }",
		"  scope :by_block, lambda { |value| where(value: value) }",
		"  before_save :normalize",
		"  around_custom :measure",
		"  enum state: { active: 1 }",
		"  delegate :title, :body,",
		"    to: :entry, allow_nil: true",
		"end",
		"",
	}, "\n")
	s := parseTempModel(t, "admin/report.rb", content)

	if s.ParentClass != "ApplicationRecord" || s.TableName != "legacy_reports" {
		t.Fatalf("class metadata = parent %q, table %q", s.ParentClass, s.TableName)
	}
	if want := []string{"  Searchable"}; !reflect.DeepEqual(s.Concerns, want) {
		t.Fatalf("Concerns = %#v, want %#v", s.Concerns, want)
	}
	if want := []string{"  has_many :entries, through: account, class_name: Entry, polymorphic: true, dependent: destroy, optional: true, inverse_of: report, source: items"}; !reflect.DeepEqual(s.Assocs, want) {
		t.Fatalf("Assocs = %#v, want %#v", s.Assocs, want)
	}
	if want := []string{
		"  validates :name, :slug, presence, uniqueness, length, format, numericality, inclusion, exclusion, confirmation, email_format, allow_nil, allow_blank, on: create",
		"  validates :code, presence_of",
		"  validate :internally_consistent (custom)",
	}; !reflect.DeepEqual(s.Valids, want) {
		t.Fatalf("Valids = %#v, want %#v", s.Valids, want)
	}
	if want := []string{"  active", "  by_name(name)", "  by_pair(left, right)", "  by_block(...)"}; !reflect.DeepEqual(s.Scopes, want) {
		t.Fatalf("Scopes = %#v, want %#v", s.Scopes, want)
	}
	if want := []string{"  before_save :normalize", "  around_custom :measure"}; !reflect.DeepEqual(s.Callbacks, want) {
		t.Fatalf("Callbacks = %#v, want %#v", s.Callbacks, want)
	}
	if want := []string{"  state"}; !reflect.DeepEqual(s.Enums, want) {
		t.Fatalf("Enums = %#v, want %#v", s.Enums, want)
	}
	if want := []string{"  delegate :title, :body, to: :entry, allow_nil: true"}; !reflect.DeepEqual(s.Delegates, want) {
		t.Fatalf("Delegates = %#v, want %#v", s.Delegates, want)
	}
}

func TestParse_IgnoresDSLTextThatIsNotRubyCalls(t *testing.T) {
	content := strings.Join([]string{
		"class Report < ApplicationRecord",
		"  # has_many :commented",
		"  EXAMPLE = <<~RUBY",
		"    validates :inside_heredoc, presence: true",
		"  RUBY",
		`  TEXT = "before_save :inside_string"`,
		"  has_many :entries",
		"end",
		"",
	}, "\n")
	s := parseTempModel(t, "report.rb", content)

	if want := []string{"  has_many :entries"}; !reflect.DeepEqual(s.Assocs, want) {
		t.Fatalf("Assocs = %#v, want %#v", s.Assocs, want)
	}
	if len(s.Valids) != 0 || len(s.Callbacks) != 0 {
		t.Fatalf("unexpected false positives: validations=%#v callbacks=%#v", s.Valids, s.Callbacks)
	}
}

func TestParse_ReturnsPartialSummaryWithParseDiagnostics(t *testing.T) {
	content := "class Broken < ApplicationRecord\n  validates :name, presence: true\n  def call(\nend\n"
	s := parseTempModel(t, "broken.rb", content)

	if s.ParentClass != "ApplicationRecord" || !containsSubstr(s.Valids, "validates :name, presence") {
		t.Fatalf("partial summary = %#v", s)
	}
	if len(s.ParseErrors) == 0 {
		t.Fatal("expected a Prism parse diagnostic")
	}
	if s.ParseErrors[0].Line < 1 || s.ParseErrors[0].Message == "" {
		t.Fatalf("invalid parse diagnostic: %#v", s.ParseErrors[0])
	}
}

func TestParse_DelegateTruncatesByRunes(t *testing.T) {
	rest := strings.Repeat("é", 81)
	s := parseTempModel(t, "report.rb", "class Report\n  delegate :"+rest+"\nend\n")
	if len(s.Delegates) != 1 {
		t.Fatalf("Delegates = %#v", s.Delegates)
	}
	want := "  delegate :" + strings.Repeat("é", 79) + "..."
	if s.Delegates[0] != want {
		t.Fatalf("delegate = %q, want %q", s.Delegates[0], want)
	}
}

func TestParse_PreservesLegacySourceShapeQuirks(t *testing.T) {
	content := strings.Join([]string{
		"class Report < ApplicationRecord",
		"  include Registry.lookup(:searchable)",
		"  has_many(",
		"    :ignored_parenthesized,",
		"    dependent: :destroy",
		"  )",
		"  has_many :entries,",
		"    dependent: :destroy",
		"  validates_numericality_of :minimum, :maximum, allow_nil: true",
		"  validate :ready?",
		"  scope :spaced, -> (value) { where(value: value) }",
		"  after_commit :sync?",
		"  class ::External < OtherRecord",
		"  end",
		"end",
		"",
	}, "\n")
	s := parseTempModel(t, "report.rb", content)

	if s.ParentClass != "ApplicationRecord" {
		t.Fatalf("ParentClass = %q, want ApplicationRecord", s.ParentClass)
	}
	if want := []string{"  Registry.lookup(:searchable)"}; !reflect.DeepEqual(s.Concerns, want) {
		t.Fatalf("Concerns = %#v, want %#v", s.Concerns, want)
	}
	if want := []string{"  has_many :entries, dependent: destroy"}; !reflect.DeepEqual(s.Assocs, want) {
		t.Fatalf("Assocs = %#v, want %#v", s.Assocs, want)
	}
	if want := []string{
		"  validates :minimum, :maximum, numericality, allow_nil, numericality_of",
		"  validate :ready (custom)",
	}; !reflect.DeepEqual(s.Valids, want) {
		t.Fatalf("Valids = %#v, want %#v", s.Valids, want)
	}
	if want := []string{"  spaced(value)"}; !reflect.DeepEqual(s.Scopes, want) {
		t.Fatalf("Scopes = %#v, want %#v", s.Scopes, want)
	}
	if want := []string{"  after_commit"}; !reflect.DeepEqual(s.Callbacks, want) {
		t.Fatalf("Callbacks = %#v, want %#v", s.Callbacks, want)
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
	if !strings.Contains(filepath.ToSlash(err.Error()), "lib/missing_models") {
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
	if summary.RelPath != "shared_models/user.rb" {
		t.Fatalf("RelPath = %q, want %q", summary.RelPath, "shared_models/user.rb")
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

func TestListNames(t *testing.T) {
	names, err := model.ListNames(testdataRoot, "app/models")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"admin/dashboard", "comment", "concerns/auditable", "concerns/searchable", "post", "user"}
	if len(names) != len(want) {
		t.Fatalf("got %v, want %v", names, want)
	}
	for i, n := range want {
		if names[i] != n {
			t.Fatalf("got %v, want %v", names, want)
		}
	}
}

func TestListNames_MissingModelsDir(t *testing.T) {
	dir := t.TempDir()
	names, err := model.ListNames(dir, "app/models")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if names != nil {
		t.Fatalf("expected nil names, got %v", names)
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

func parseTempModel(t *testing.T, relPath, content string) *model.Summary {
	t.Helper()
	root := t.TempDir()
	path := filepath.Join(root, "app", "models", filepath.FromSlash(relPath))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := model.Parse(path, root, "app/models")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return s
}
