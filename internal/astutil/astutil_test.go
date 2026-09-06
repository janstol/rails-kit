package astutil_test

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/danielgatis/go-ruby-prism/parser"

	"github.com/janstol/rails-kit/internal/astutil"
	"github.com/janstol/rails-kit/internal/prism"
)

// writeRuby writes src to a temp .rb file and returns its path.
func writeRuby(t *testing.T, src string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "sample.rb")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatalf("writing sample file: %v", err)
	}
	return path
}

// classBody parses src -- expected to declare a single top-level class -- and
// returns its source bytes and body statements.
func classBody(t *testing.T, src string) ([]byte, []parser.Node) {
	t.Helper()
	p, err := astutil.ParseFile(writeRuby(t, src))
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if p.Program == nil {
		t.Fatalf("no program recovered from source: %s", src)
	}
	class := astutil.TopLevelClass(p.Program)
	if class == nil {
		t.Fatalf("no top-level class found in source: %s", src)
	}
	return p.Src, prism.BlockStatements(class.Body)
}

func TestParseFile(t *testing.T) {
	p, err := astutil.ParseFile(writeRuby(t, "class Foo\nend\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(p.Diagnostics) != 0 {
		t.Errorf("Diagnostics = %#v, want none", p.Diagnostics)
	}
	if p.Program == nil {
		t.Fatal("Program = nil, want a recovered program")
	}
	if class := astutil.TopLevelClass(p.Program); class == nil {
		t.Error("expected TopLevelClass to find Foo")
	}
}

func TestParseFile_SyntaxError(t *testing.T) {
	p, err := astutil.ParseFile(writeRuby(t, "class Foo\n  def bar(\nend\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(p.Diagnostics) == 0 {
		t.Fatal("expected at least one parse diagnostic")
	}
	if p.Diagnostics[0].Line == 0 {
		t.Errorf("Diagnostics[0].Line = 0, want a positive line number")
	}
}

func TestSummaryPath_Nested(t *testing.T) {
	railsRoot := filepath.FromSlash("/rails")
	dirPath := "app/controllers"
	filePath := filepath.Join(railsRoot, "app", "controllers", "admin", "users_controller.rb")

	relPath, className := astutil.SummaryPath(filePath, railsRoot, dirPath)

	if want := "app/controllers/admin/users_controller.rb"; relPath != want {
		t.Errorf("relPath = %q, want %q", relPath, want)
	}
	if want := "Admin::UsersController"; className != want {
		t.Errorf("className = %q, want %q", className, want)
	}
}

func TestSummaryPath_OutsideRailsRoot(t *testing.T) {
	railsRoot := filepath.FromSlash("/rails")
	filePath := filepath.FromSlash("/elsewhere/users_controller.rb")

	relPath, _ := astutil.SummaryPath(filePath, railsRoot, "app/controllers")

	if relPath != filepath.ToSlash(filePath) {
		t.Errorf("relPath = %q, want the absolute filePath %q", relPath, filePath)
	}
}

func TestSummaryPath_OutsideDirPath(t *testing.T) {
	railsRoot := filepath.FromSlash("/rails")
	filePath := filepath.Join(railsRoot, "lib", "users_controller.rb")

	_, className := astutil.SummaryPath(filePath, railsRoot, "app/controllers")

	if want := "UsersController"; className != want {
		t.Errorf("className = %q, want %q (falls back to the base name)", className, want)
	}
}

func TestWalkClassBody_Visibility(t *testing.T) {
	_, nodes := classBody(t, `class Foo
  def a; end

  private

  def b; end

  public

  def c; end
end
`)

	var defs []string
	astutil.WalkClassBody(nodes, astutil.ClassBody{
		Def: func(def *parser.DefNode, visibility string) {
			defs = append(defs, def.Name+":"+visibility)
		},
	})

	want := []string{"a:public", "b:private", "c:public"}
	if !reflect.DeepEqual(defs, want) {
		t.Errorf("defs = %#v, want %#v", defs, want)
	}
}

func TestWalkClassBody_PrivateDefSingleMethodForm(t *testing.T) {
	_, nodes := classBody(t, `class Foo
  private def a; end
  def b; end
end
`)

	var defs []string
	astutil.WalkClassBody(nodes, astutil.ClassBody{
		Def: func(def *parser.DefNode, visibility string) {
			defs = append(defs, def.Name+":"+visibility)
		},
	})

	want := []string{"a:private", "b:public"}
	if !reflect.DeepEqual(defs, want) {
		t.Errorf("defs = %#v, want %#v", defs, want)
	}
}

func TestWalkClassBody_SymbolListForm(t *testing.T) {
	_, nodes := classBody(t, `class Foo
  def a; end
  private :a, :b
  other_macro :x
end
`)

	t.Run("forwarded by default", func(t *testing.T) {
		var calls []string
		astutil.WalkClassBody(nodes, astutil.ClassBody{
			Call: func(call *parser.CallNode) {
				calls = append(calls, call.Name)
			},
		})
		want := []string{"private", "other_macro"}
		if !reflect.DeepEqual(calls, want) {
			t.Errorf("calls = %#v, want %#v", calls, want)
		}
	})

	t.Run("dropped with SkipVisibilityCalls", func(t *testing.T) {
		var calls []string
		astutil.WalkClassBody(nodes, astutil.ClassBody{
			Call: func(call *parser.CallNode) {
				calls = append(calls, call.Name)
			},
			SkipVisibilityCalls: true,
		})
		want := []string{"other_macro"}
		if !reflect.DeepEqual(calls, want) {
			t.Errorf("calls = %#v, want %#v", calls, want)
		}
	})
}

func TestWalkClassBody_NilCallbacks(t *testing.T) {
	_, nodes := classBody(t, `class Foo
  include Bar
  BAZ = 1

  def a; end
end
`)

	// Nothing should panic when every callback is nil.
	astutil.WalkClassBody(nodes, astutil.ClassBody{})
}

func TestWalkClassBody_Const(t *testing.T) {
	_, nodes := classBody(t, `class Foo
  LIMIT = 100
  def a; end
end
`)

	var consts []string
	astutil.WalkClassBody(nodes, astutil.ClassBody{
		Const: func(n *parser.ConstantWriteNode) {
			consts = append(consts, n.Name)
		},
	})

	want := []string{"LIMIT"}
	if !reflect.DeepEqual(consts, want) {
		t.Errorf("consts = %#v, want %#v", consts, want)
	}
}

func TestIncludedConcern(t *testing.T) {
	src, nodes := classBody(t, `class Foo
  include Trackable
  include ActionController::Cookies
end
`)

	var calls []*parser.CallNode
	astutil.WalkClassBody(nodes, astutil.ClassBody{
		Call: func(call *parser.CallNode) {
			calls = append(calls, call)
		},
	})
	if len(calls) != 2 {
		t.Fatalf("got %d include calls, want 2", len(calls))
	}

	if got := astutil.IncludedConcern(src, prism.ArgNodes(calls[0]), []string{"ActionController"}); got != "Trackable" {
		t.Errorf("IncludedConcern = %q, want Trackable", got)
	}
	if got := astutil.IncludedConcern(src, prism.ArgNodes(calls[1]), []string{"ActionController"}); got != "" {
		t.Errorf("IncludedConcern = %q, want \"\" (skipped prefix)", got)
	}
	if got := astutil.IncludedConcern(src, prism.ArgNodes(calls[1]), nil); got != "ActionController::Cookies" {
		t.Errorf("IncludedConcern with no skipped prefixes = %q, want ActionController::Cookies", got)
	}
	if got := astutil.IncludedConcern(src, nil, nil); got != "" {
		t.Errorf("IncludedConcern with no args = %q, want \"\"", got)
	}
}

func TestLayoutValue(t *testing.T) {
	src, nodes := classBody(t, `class Foo
  layout false
  layout "admin"
  layout :application
  layout resolve_layout
end
`)

	var calls []*parser.CallNode
	astutil.WalkClassBody(nodes, astutil.ClassBody{
		Call: func(call *parser.CallNode) {
			calls = append(calls, call)
		},
	})
	if len(calls) != 4 {
		t.Fatalf("got %d layout calls, want 4", len(calls))
	}

	tests := []struct {
		name string
		want string
	}{
		{"false literal", "false"},
		{"string", `"admin"`},
		{"symbol", ":application"},
		{"other expression", "resolve_layout"},
	}
	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := astutil.LayoutValue(src, prism.ArgNodes(calls[i]))
			if got != tt.want {
				t.Errorf("LayoutValue = %q, want %q", got, tt.want)
			}
		})
	}

	if got := astutil.LayoutValue(src, nil); got != "" {
		t.Errorf("LayoutValue with no args = %q, want \"\"", got)
	}
}

func TestAssocsBySymbolKey(t *testing.T) {
	_, nodes := classBody(t, `class Foo
  before_action :check, only: [:show], if: :admin?
end
`)

	var call *parser.CallNode
	astutil.WalkClassBody(nodes, astutil.ClassBody{
		Call: func(c *parser.CallNode) { call = c },
	})
	if call == nil {
		t.Fatal("expected to find before_action call")
	}

	var assocs []*parser.AssocNode
	for _, arg := range prism.ArgNodes(call) {
		assocs = append(assocs, prism.KeywordAssocs(arg)...)
	}

	byKey := astutil.AssocsBySymbolKey(assocs)
	if _, ok := byKey["only"]; !ok {
		t.Error(`expected "only" key`)
	}
	if _, ok := byKey["if"]; !ok {
		t.Error(`expected "if" key`)
	}
	if len(byKey) != 2 {
		t.Errorf("byKey has %d entries, want 2", len(byKey))
	}
}

func TestCollectCalls_SourceOrder(t *testing.T) {
	_, nodes := classBody(t, `class Foo
  def bar
    third_call
    if true
      first_call
    end
    second_call
  end
end
`)

	var def *parser.DefNode
	astutil.WalkClassBody(nodes, astutil.ClassBody{
		Def: func(d *parser.DefNode, _ string) { def = d },
	})
	if def == nil {
		t.Fatal("expected to find def bar")
	}

	calls := astutil.CollectCalls(def.Body, func(call *parser.CallNode) bool {
		return call.Name == "first_call" || call.Name == "second_call" || call.Name == "third_call"
	})

	var names []string
	for _, call := range calls {
		names = append(names, call.Name)
	}
	want := []string{"third_call", "first_call", "second_call"}
	if !reflect.DeepEqual(names, want) {
		t.Errorf("names = %#v, want %#v", names, want)
	}
}

func TestCollectCalls_NoMatch(t *testing.T) {
	_, nodes := classBody(t, `class Foo
  def bar
    baz
  end
end
`)

	var def *parser.DefNode
	astutil.WalkClassBody(nodes, astutil.ClassBody{
		Def: func(d *parser.DefNode, _ string) { def = d },
	})
	if def == nil {
		t.Fatal("expected to find def bar")
	}

	calls := astutil.CollectCalls(def.Body, func(call *parser.CallNode) bool {
		return call.Name == "nonexistent"
	})
	if calls != nil {
		t.Errorf("calls = %#v, want nil", calls)
	}
}
