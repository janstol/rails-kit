package datagrids_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/janstol/rails-kit/internal/datagrids"
	"github.com/janstol/rails-kit/internal/term"
)

const testdataRoot = "../../testdata"

func TestParse_Example(t *testing.T) {
	path := testdataRoot + "/app/datagrids/example_datagrid.rb"
	s, err := datagrids.Parse(path, testdataRoot, "app/datagrids")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if s.ClassName != "ExampleDatagrid" {
		t.Errorf("ClassName = %q, want ExampleDatagrid", s.ClassName)
	}
	if s.ParentClass != "BaseDatagrid" {
		t.Errorf("ParentClass = %q, want BaseDatagrid", s.ParentClass)
	}
	if s.Decorate != "ExampleDecorator" {
		t.Errorf("Decorate = %q, want ExampleDecorator", s.Decorate)
	}
	if s.Scope != "(block)" {
		t.Errorf("Scope = %q, want (block)", s.Scope)
	}

	if want := []string{"  Filterable", "  Sortable"}; !reflect.DeepEqual(s.Concerns, want) {
		t.Errorf("Concerns = %#v, want %#v", s.Concerns, want)
	}

	if want := []string{
		"  filter :name",
		"  filter :latch_model, :enum, select: -> { Example.latch_options }, input_options: { class: 'select2' }",
		"  filter :active, :boolean, default: false (block)",
	}; !reflect.DeepEqual(s.Filters, want) {
		t.Errorf("Filters = %#v, want %#v", s.Filters, want)
	}

	if want := []string{
		"  column :name, &:show_link",
		"  column :state, order: proc { |direction| direction }, &:state_name",
		`  column :score, order: "score" (block)`,
	}; !reflect.DeepEqual(s.Columns, want) {
		t.Errorf("Columns = %#v, want %#v", s.Columns, want)
	}

	if want := []string{"  filter_per_page", "  column_actions"}; !reflect.DeepEqual(s.Macros, want) {
		t.Errorf("Macros = %#v, want %#v", s.Macros, want)
	}

	// Public instance method `assets` and singleton `def self.build_default`
	// are collected; the `private def secret_scope` single-def form and the
	// bare-private `base_relation` are excluded.
	if want := []string{"  assets", "  build_default"}; !reflect.DeepEqual(s.Methods, want) {
		t.Errorf("Methods = %#v, want %#v", s.Methods, want)
	}
	for _, m := range s.Methods {
		if strings.Contains(m, "secret_scope") || strings.Contains(m, "base_relation") {
			t.Errorf("private method leaked into Methods: %q", m)
		}
	}
}

func TestParse_CustomGrid(t *testing.T) {
	path := testdataRoot + "/app/datagrids/custom_grid.rb"
	s, err := datagrids.Parse(path, testdataRoot, "app/datagrids")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Graceful degradation: a non-`datagrid`-gem file keeps a useful
	// services-shaped summary but has no datagrid-gem-specific structure.
	if s.ClassName != "CustomGrid" {
		t.Errorf("ClassName = %q, want CustomGrid", s.ClassName)
	}
	if s.ParentClass != "" {
		t.Errorf("ParentClass = %q, want empty (no BaseDatagrid)", s.ParentClass)
	}
	if s.Decorate != "" {
		t.Errorf("Decorate = %q, want empty", s.Decorate)
	}
	if s.Scope != "" {
		t.Errorf("Scope = %q, want empty", s.Scope)
	}
	if s.Filters != nil {
		t.Errorf("Filters = %#v, want nil", s.Filters)
	}
	if s.Columns != nil {
		t.Errorf("Columns = %#v, want nil", s.Columns)
	}
	if want := []string{"  Exportable"}; !reflect.DeepEqual(s.Concerns, want) {
		t.Errorf("Concerns = %#v, want %#v", s.Concerns, want)
	}
	if want := []string{"  my_filter :name", "  my_column :total", "  batch_size 50"}; !reflect.DeepEqual(s.Macros, want) {
		t.Errorf("Macros = %#v, want %#v", s.Macros, want)
	}
	if want := []string{"  rows", "  default_grid"}; !reflect.DeepEqual(s.Methods, want) {
		t.Errorf("Methods = %#v, want %#v", s.Methods, want)
	}
	for _, m := range s.Methods {
		if strings.Contains(m, "normalized_rows") {
			t.Errorf("private method leaked into Methods: %q", m)
		}
	}
}

func TestListNames(t *testing.T) {
	names, err := datagrids.ListNames(testdataRoot, "app/datagrids")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// `example_datagrid` has the suffix stripped; `custom_grid` does not, so it
	// is preserved verbatim.
	want := []string{"custom_grid", "example"}
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("got %v, want %v", names, want)
	}
}

func TestListNames_MissingDatagridsDir(t *testing.T) {
	dir := t.TempDir()
	names, err := datagrids.ListNames(dir, "app/datagrids")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if names != nil {
		t.Fatalf("expected nil names, got %v", names)
	}
}

func TestResolve(t *testing.T) {
	t.Run("by short name (suffixed)", func(t *testing.T) {
		path, err := datagrids.Resolve(testdataRoot, "app/datagrids", "example")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.HasSuffix(filepath.ToSlash(path), "app/datagrids/example_datagrid.rb") {
			t.Errorf("unexpected path: %s", path)
		}
	})

	t.Run("by CamelCase class name (suffixed)", func(t *testing.T) {
		path, err := datagrids.Resolve(testdataRoot, "app/datagrids", "ExampleDatagrid")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.HasSuffix(filepath.ToSlash(path), "app/datagrids/example_datagrid.rb") {
			t.Errorf("unexpected path: %s", path)
		}
	})

	t.Run("by full basename with suffix", func(t *testing.T) {
		path, err := datagrids.Resolve(testdataRoot, "app/datagrids", "example_datagrid")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.HasSuffix(filepath.ToSlash(path), "app/datagrids/example_datagrid.rb") {
			t.Errorf("unexpected path: %s", path)
		}
	})

	t.Run("by short name (non-suffixed custom grid)", func(t *testing.T) {
		path, err := datagrids.Resolve(testdataRoot, "app/datagrids", "custom_grid")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.HasSuffix(filepath.ToSlash(path), "app/datagrids/custom_grid.rb") {
			t.Errorf("unexpected path: %s", path)
		}
	})

	t.Run("by CamelCase class name (non-suffixed custom grid)", func(t *testing.T) {
		path, err := datagrids.Resolve(testdataRoot, "app/datagrids", "CustomGrid")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.HasSuffix(filepath.ToSlash(path), "app/datagrids/custom_grid.rb") {
			t.Errorf("unexpected path: %s", path)
		}
	})

	t.Run("by .rb path", func(t *testing.T) {
		path, err := datagrids.Resolve(testdataRoot, "app/datagrids", "app/datagrids/example_datagrid.rb")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.HasSuffix(filepath.ToSlash(path), "app/datagrids/example_datagrid.rb") {
			t.Errorf("unexpected path: %s", path)
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, err := datagrids.Resolve(testdataRoot, "app/datagrids", "nonexistent")
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("ambiguous basename", func(t *testing.T) {
		// Two nested files sharing a basename and no top-level match: the
		// suffixed candidate resolves by basename in two places at once.
		extra1 := filepath.Join(testdataRoot, "app/datagrids", "reporting", "report_datagrid.rb")
		extra2 := filepath.Join(testdataRoot, "app/datagrids", "billing", "report_datagrid.rb")
		for _, extra := range []string{extra1, extra2} {
			if err := os.MkdirAll(filepath.Dir(extra), 0o755); err != nil {
				t.Fatalf("setup: %v", err)
			}
			if err := os.WriteFile(extra, []byte("class ReportDatagrid < BaseDatagrid\nend\n"), 0o644); err != nil {
				t.Fatalf("setup: %v", err)
			}
		}
		t.Cleanup(func() {
			_ = os.Remove(extra1)
			_ = os.Remove(filepath.Dir(extra1))
			_ = os.Remove(extra2)
			_ = os.Remove(filepath.Dir(extra2))
		})

		_, err := datagrids.Resolve(testdataRoot, "app/datagrids", "report")
		if err == nil {
			t.Fatal("expected error for ambiguous datagrid name")
		}
		if !datagrids.IsAmbiguousError(err) {
			t.Errorf("expected ambiguous error, got: %v", err)
		}
	})

	t.Run("outside datagrids dir", func(t *testing.T) {
		dir := t.TempDir()
		f := filepath.Join(dir, "elsewhere.rb")
		if err := os.WriteFile(f, []byte("class Elsewhere\nend\n"), 0o644); err != nil {
			t.Fatalf("setup: %v", err)
		}
		_, err := datagrids.Resolve(testdataRoot, "app/datagrids", f)
		if err == nil {
			t.Fatal("expected error for file outside datagrids directory")
		}
	})
}

func TestFormat(t *testing.T) {
	path := testdataRoot + "/app/datagrids/example_datagrid.rb"
	s, err := datagrids.Parse(path, testdataRoot, "app/datagrids")
	if err != nil {
		t.Fatal(err)
	}
	out := datagrids.Format(s, term.Styler{})
	if !strings.Contains(out, "ExampleDatagrid < BaseDatagrid (") {
		t.Error("missing class header in output")
	}
	if !strings.Contains(out, "Decorate:") {
		t.Error("missing Decorate section")
	}
	if !strings.Contains(out, "Scope:") {
		t.Error("missing Scope section")
	}
	if !strings.Contains(out, "Filters:") {
		t.Error("missing Filters section")
	}
	if !strings.Contains(out, "Columns:") {
		t.Error("missing Columns section")
	}
	if !strings.Contains(out, "Macros:") {
		t.Error("missing Macros section")
	}
	if !strings.Contains(out, "Methods:") {
		t.Error("missing Methods section")
	}
}

func TestParse_ReturnsPartialSummaryWithParseDiagnostics(t *testing.T) {
	s := parseTempDatagrid(t, "broken_datagrid.rb", "class Broken < BaseDatagrid\n  filter :name\n  def assets(\nend\n")

	if s.ParentClass != "BaseDatagrid" || !containsSubstr(s.Filters, "filter :name") {
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
		"class Broken < BaseDatagrid",
		"  filter :name",
		"",
		"  class InlineError < StandardError",
		"    def message",
		"      \"nope\"",
		"    end",
		"  end",
		"end",
		"",
	}, "\n")
	s := parseTempDatagrid(t, "broken.rb", content)

	if want := []string{"  filter :name"}; !reflect.DeepEqual(s.Filters, want) {
		t.Fatalf("Filters leaked nested class calls: %#v", s.Filters)
	}
	if len(s.Methods) != 0 {
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

func parseTempDatagrid(t *testing.T, relPath, content string) *datagrids.Summary {
	t.Helper()
	root := t.TempDir()
	path := filepath.Join(root, "app", "datagrids", filepath.FromSlash(relPath))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := datagrids.Parse(path, root, "app/datagrids")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return s
}
