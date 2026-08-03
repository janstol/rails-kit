package concerns_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/janstol/rails-kit/internal/concerns"
)

const testdataRoot = "../../testdata"

func TestListFiles_ModelConcerns(t *testing.T) {
	dir := filepath.Join(testdataRoot, "app/models/concerns")
	names, err := concerns.ListFiles(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"auditable", "searchable"}
	if len(names) != len(want) {
		t.Fatalf("got %v, want %v", names, want)
	}
	for i, n := range names {
		if n != want[i] {
			t.Errorf("names[%d] = %q, want %q", i, n, want[i])
		}
	}
}

func TestListFiles_ControllerConcerns(t *testing.T) {
	dir := filepath.Join(testdataRoot, "app/controllers/concerns")
	names, err := concerns.ListFiles(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(names) != 1 || names[0] != "authenticatable" {
		t.Fatalf("got %v, want [authenticatable]", names)
	}
}

func TestListFiles_MissingDir(t *testing.T) {
	names, err := concerns.ListFiles("/nonexistent/path/to/concerns")
	if err != nil {
		t.Fatalf("missing dir should not error, got: %v", err)
	}
	if names != nil {
		t.Fatalf("missing dir should return nil, got: %v", names)
	}
}

func TestListFiles_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	names, err := concerns.ListFiles(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(names) != 0 {
		t.Fatalf("expected empty, got %v", names)
	}
}

func TestParse_Searchable(t *testing.T) {
	path := filepath.Join(testdataRoot, "app/models/concerns/searchable.rb")
	d, err := concerns.Parse(path, "app/models/concerns/searchable.rb", "model")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.Name != "Searchable" {
		t.Errorf("Name = %q, want Searchable", d.Name)
	}
	if d.Type != "model" {
		t.Errorf("Type = %q, want model", d.Type)
	}
	if !d.HasIncludedBlock {
		t.Error("HasIncludedBlock = false, want true")
	}
	if !d.HasClassMethodsBlock {
		t.Error("HasClassMethodsBlock = false, want true")
	}
	if !containsMethod(d.Methods, "search_highlight") {
		t.Errorf("Methods should contain search_highlight, got %v", d.Methods)
	}
	if !containsMethod(d.Methods, "search_excerpt") {
		t.Errorf("Methods should contain search_excerpt, got %v", d.Methods)
	}
	if !containsMethod(d.ClassMethods, "search_all") {
		t.Errorf("ClassMethods should contain search_all, got %v", d.ClassMethods)
	}
}

func TestParse_Auditable(t *testing.T) {
	path := filepath.Join(testdataRoot, "app/models/concerns/auditable.rb")
	d, err := concerns.Parse(path, "app/models/concerns/auditable.rb", "model")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.Name != "Auditable" {
		t.Errorf("Name = %q, want Auditable", d.Name)
	}
	if !d.HasIncludedBlock {
		t.Error("HasIncludedBlock = false, want true")
	}
	if d.HasClassMethodsBlock {
		t.Error("HasClassMethodsBlock = true, want false")
	}
	if !containsMethod(d.Methods, "audit_trail") {
		t.Errorf("Methods should contain audit_trail, got %v", d.Methods)
	}
	if !containsMethod(d.Methods, "last_audited_at") {
		t.Errorf("Methods should contain last_audited_at, got %v", d.Methods)
	}
	if len(d.ClassMethods) != 0 {
		t.Errorf("ClassMethods should be empty, got %v", d.ClassMethods)
	}
}

func TestParse_Authenticatable(t *testing.T) {
	path := filepath.Join(testdataRoot, "app/controllers/concerns/authenticatable.rb")
	d, err := concerns.Parse(path, "app/controllers/concerns/authenticatable.rb", "controller")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.Name != "Authenticatable" {
		t.Errorf("Name = %q, want Authenticatable", d.Name)
	}
	if d.Type != "controller" {
		t.Errorf("Type = %q, want controller", d.Type)
	}
	if !d.HasIncludedBlock {
		t.Error("HasIncludedBlock = false, want true")
	}
	if !containsMethod(d.Methods, "authenticate_user!") {
		t.Errorf("Methods should contain authenticate_user!, got %v", d.Methods)
	}
	if !containsMethod(d.Methods, "current_user") {
		t.Errorf("Methods should contain current_user, got %v", d.Methods)
	}
}

func TestFindConcern_ModelConcern(t *testing.T) {
	modelDir := filepath.Join(testdataRoot, "app/models/concerns")
	ctrlDir := filepath.Join(testdataRoot, "app/controllers/concerns")
	root := testdataRoot

	fullPath, relPath, cType, err := concerns.FindConcern(modelDir, ctrlDir, root, "searchable")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cType != "model" {
		t.Errorf("type = %q, want model", cType)
	}
	if _, statErr := os.Stat(fullPath); statErr != nil {
		t.Errorf("fullPath %q does not exist: %v", fullPath, statErr)
	}
	if relPath == "" {
		t.Error("relPath should not be empty")
	}
}

func TestFindConcern_ControllerConcern(t *testing.T) {
	modelDir := filepath.Join(testdataRoot, "app/models/concerns")
	ctrlDir := filepath.Join(testdataRoot, "app/controllers/concerns")
	root := testdataRoot

	_, _, cType, err := concerns.FindConcern(modelDir, ctrlDir, root, "authenticatable")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cType != "controller" {
		t.Errorf("type = %q, want controller", cType)
	}
}

func TestFindConcern_QualifiedModel(t *testing.T) {
	modelDir := filepath.Join(testdataRoot, "app/models/concerns")
	ctrlDir := filepath.Join(testdataRoot, "app/controllers/concerns")
	root := testdataRoot

	_, _, cType, err := concerns.FindConcern(modelDir, ctrlDir, root, "model/searchable")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cType != "model" {
		t.Errorf("type = %q, want model", cType)
	}
}

func TestFindConcern_NotFound(t *testing.T) {
	modelDir := filepath.Join(testdataRoot, "app/models/concerns")
	ctrlDir := filepath.Join(testdataRoot, "app/controllers/concerns")
	root := testdataRoot

	_, _, _, err := concerns.FindConcern(modelDir, ctrlDir, root, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent concern")
	}
}

func TestFindConcern_Ambiguous(t *testing.T) {
	dir := t.TempDir()
	modelDir := filepath.Join(dir, "models/concerns")
	ctrlDir := filepath.Join(dir, "controllers/concerns")
	if err := os.MkdirAll(modelDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(ctrlDir, 0755); err != nil {
		t.Fatal(err)
	}
	// Create same concern in both directories
	for _, d := range []string{modelDir, ctrlDir} {
		if err := os.WriteFile(filepath.Join(d, "shared.rb"), []byte("module Shared\nend\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	_, _, _, err := concerns.FindConcern(modelDir, ctrlDir, dir, "shared")
	if err == nil {
		t.Fatal("expected ambiguity error")
	}
}

func containsMethod(methods []string, name string) bool {
	for _, m := range methods {
		if m == name {
			return true
		}
	}
	return false
}

func parseTempConcern(t *testing.T, content string) *concerns.ConcernDetail {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "concern.rb")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	d, err := concerns.Parse(path, "app/models/concerns/concern.rb", "model")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return d
}

// Regression guard for regex-scanner bug #1: an `end` that isn't alone on its
// line (closing a modifier, a method chain, or a parenthesized argument) used
// to drift the old line-based depth counter, permanently misfiling every
// instance method that followed as a class method. The AST has no such notion
// of depth, so these can't recur, but the fixtures are kept as a regression
// guard since they're exactly the shapes that broke the old scanner.
func TestParse_EndVariantsDoNotLeakClassMethodsScope(t *testing.T) {
	cases := []struct {
		name        string
		content     string
		classMethod string
	}{
		{
			name: "end followed by a comment",
			content: `module Bug1bEndComment
  extend ActiveSupport::Concern

  class_methods do
    def guarded_class_method
      if some_flag
        1
      end # inner if
    end

    def class_method_one
      2
    end
  end

  def instance_after_class_methods
    "should stay an instance method"
  end
end
`,
			classMethod: "guarded_class_method",
		},
		{
			name: "end followed by a method chain",
			content: `module Bug1cEndTap
  extend ActiveSupport::Concern

  class_methods do
    def wrapped_class_method
      begin
        compute_default
      end.tap { |v| v }
    end

    def class_method_one
      2
    end
  end

  def instance_after_class_methods
    "should stay an instance method"
  end
end
`,
			classMethod: "wrapped_class_method",
		},
		{
			name: "end closing a parenthesized argument",
			content: `module Bug1dEndParen
  extend ActiveSupport::Concern

  class_methods do
    def wrapped_class_method
      foo(1,
        begin
          compute_default
        end)
    end

    def class_method_one
      2
    end
  end

  def instance_after_class_methods
    "should stay an instance method"
  end
end
`,
			classMethod: "wrapped_class_method",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := parseTempConcern(t, tc.content)
			if !containsMethod(d.ClassMethods, tc.classMethod) {
				t.Errorf("ClassMethods should contain %s, got %v", tc.classMethod, d.ClassMethods)
			}
			if !containsMethod(d.ClassMethods, "class_method_one") {
				t.Errorf("ClassMethods should contain class_method_one, got %v", d.ClassMethods)
			}
			if !containsMethod(d.Methods, "instance_after_class_methods") {
				t.Errorf("Methods should contain instance_after_class_methods, got %v", d.Methods)
			}
			if containsMethod(d.ClassMethods, "instance_after_class_methods") {
				t.Errorf("instance_after_class_methods leaked into ClassMethods: %v", d.ClassMethods)
			}
		})
	}
}

// Regression guard for regex-scanner bug #2: an endless method (`def foo =
// expr`, Ruby 3.0+) never matched the scanner's opener/closer heuristic and
// drifted depth the same way a stray `end` did.
func TestParse_EndlessMethodInsideClassMethodsBlock(t *testing.T) {
	d := parseTempConcern(t, `module Bug2EndlessMethod
  extend ActiveSupport::Concern

  class_methods do
    def short_name = name.split(" ").first

    def class_method_one
      1
    end
  end

  def instance_after_class_methods
    "should stay an instance method"
  end
end
`)
	if !containsMethod(d.ClassMethods, "short_name") {
		t.Errorf("ClassMethods should contain short_name, got %v", d.ClassMethods)
	}
	if !containsMethod(d.ClassMethods, "class_method_one") {
		t.Errorf("ClassMethods should contain class_method_one, got %v", d.ClassMethods)
	}
	if !containsMethod(d.Methods, "instance_after_class_methods") {
		t.Errorf("Methods should contain instance_after_class_methods, got %v", d.Methods)
	}
}

// Regression guard for regex-scanner bug #3: `class << self` bodies used to
// be recorded as instance Methods. The AST driver correctly maps them to
// ClassMethods -- a deliberate behavior change from the old scanner.
func TestParse_SingletonClassMapsToClassMethods(t *testing.T) {
	d := parseTempConcern(t, `module Bug3SingletonClass
  extend ActiveSupport::Concern

  class << self
    def class_level_helper
      "should be a class method"
    end
  end

  def instance_method
    "an actual instance method"
  end
end
`)
	if !containsMethod(d.ClassMethods, "class_level_helper") {
		t.Errorf("ClassMethods should contain class_level_helper, got %v", d.ClassMethods)
	}
	if containsMethod(d.Methods, "class_level_helper") {
		t.Errorf("class_level_helper should not appear in Methods, got %v", d.Methods)
	}
	if !containsMethod(d.Methods, "instance_method") {
		t.Errorf("Methods should contain instance_method, got %v", d.Methods)
	}
}

// Regression guard for regex-scanner bug #4: a heredoc body containing `def`
// and `end` used to be scanned as code, fabricating a phantom method and
// corrupting the depth counter.
func TestParse_HeredocBodyIsNotParsedAsCode(t *testing.T) {
	d := parseTempConcern(t, `module Bug4Heredoc
  extend ActiveSupport::Concern

  def render_snippet
    <<~RUBY
      def fake_method
        "this is text inside a heredoc, not a real def"
      end
    RUBY
  end

  class_methods do
    def class_method_one
      1
    end
  end

  def instance_after_class_methods
    "should stay an instance method"
  end
end
`)
	if containsMethod(d.Methods, "fake_method") {
		t.Errorf("fake_method from inside the heredoc should not appear, got %v", d.Methods)
	}
	if !containsMethod(d.Methods, "render_snippet") {
		t.Errorf("Methods should contain render_snippet, got %v", d.Methods)
	}
	if !containsMethod(d.ClassMethods, "class_method_one") {
		t.Errorf("ClassMethods should contain class_method_one, got %v", d.ClassMethods)
	}
	if !containsMethod(d.Methods, "instance_after_class_methods") {
		t.Errorf("Methods should contain instance_after_class_methods, got %v", d.Methods)
	}
}

// Regression guard for regex-scanner bug #5: nested keyword modules
// ("module A; module B; ...; end; end") report only the outermost module's
// Name, not "A::B". This is a deliberate, documented divergence rather than
// a bug to fix -- locked in here so it can't silently change. The nested
// module's content is still walked, so methods inside it aren't lost.
func TestParse_NestedModuleReportsOutermostNameOnly(t *testing.T) {
	d := parseTempConcern(t, `module Bug5NestedModule
  module Inner
    extend ActiveSupport::Concern

    def nested_method
      "inside the nested module"
    end
  end
end
`)
	if d.Name != "Bug5NestedModule" {
		t.Errorf("Name = %q, want Bug5NestedModule (outermost only)", d.Name)
	}
	if !containsMethod(d.Methods, "nested_method") {
		t.Errorf("Methods should still contain nested_method, got %v", d.Methods)
	}
}

// Real-app dogfooding against Application A/B turned up a sixth construct:
// the pre-ActiveSupport::Concern idiom `def self.included(base); base
// .class_eval do ... end; end`, used to inject instance methods before the
// `included do...end` macro existed. The regex scanner handled it by
// accident (while also emitting a bogus "self" method from misparsing
// `def self.included`); Parse recognizes it explicitly and treats the
// class_eval block like an `included do` block.
func TestParse_LegacyIncludedHookClassEval(t *testing.T) {
	d := parseTempConcern(t, `module Verifiable
  def self.included(base)
    base.class_eval do
      attr_accessor :email

      def verify_person
        "verified"
      end
    end
  end
end
`)
	if !d.HasIncludedBlock {
		t.Error("HasIncludedBlock = false, want true")
	}
	if !containsMethod(d.Methods, "verify_person") {
		t.Errorf("Methods should contain verify_person, got %v", d.Methods)
	}
	if containsMethod(d.Methods, "self") {
		t.Errorf("Methods should not contain a bogus \"self\" entry, got %v", d.Methods)
	}
	if containsMethod(d.Methods, "included") {
		t.Errorf("Methods should not contain the hook itself, got %v", d.Methods)
	}
}

func TestParse_ReturnsPartialDetailWithParseDiagnostics(t *testing.T) {
	d := parseTempConcern(t, "module Broken\n  def call(\nend\n")
	if len(d.ParseErrors) == 0 {
		t.Fatal("expected a Prism parse diagnostic")
	}
	if d.ParseErrors[0].Line < 1 || d.ParseErrors[0].Message == "" {
		t.Fatalf("invalid parse diagnostic: %#v", d.ParseErrors[0])
	}
}
