package reader_test

import (
	"strings"
	"testing"

	"github.com/janstol/rails-kit/internal/reader"
	"github.com/janstol/rails-kit/internal/term"
)

func TestFormat_HeaderWithParent(t *testing.T) {
	k := reader.Kind{}
	out := k.Format(reader.Header{Title: "User", Parent: "ApplicationRecord", RelPath: "app/models/user.rb"}, nil, term.Styler{})
	if !strings.HasPrefix(out, "User < ApplicationRecord (app/models/user.rb)\n"+strings.Repeat("=", 40)+"\n") {
		t.Errorf("unexpected header:\n%s", out)
	}
}

func TestFormat_HeaderWithoutParent(t *testing.T) {
	k := reader.Kind{}
	out := k.Format(reader.Header{Title: "User", RelPath: "app/models/user.rb"}, nil, term.Styler{})
	if !strings.HasPrefix(out, "User (app/models/user.rb)\n"+strings.Repeat("=", 40)+"\n") {
		t.Errorf("unexpected header, want no ' < ' when Parent is empty:\n%s", out)
	}
}

func TestFormat_ModuleStyleHeader(t *testing.T) {
	// services renders "module Foo" as the whole Title with no Parent, even
	// when ParentClass would otherwise be set -- the caller decides this by
	// what it puts in Header, not Format. See TestClassOrModuleHeader for the
	// helper that makes that decision for the readers that need it.
	k := reader.Kind{}
	out := k.Format(reader.Header{Title: "module Foo", RelPath: "app/services/foo.rb"}, nil, term.Styler{})
	if !strings.HasPrefix(out, "module Foo (app/services/foo.rb)\n") {
		t.Errorf("unexpected module-style header:\n%s", out)
	}
}

func TestClassOrModuleHeader(t *testing.T) {
	tests := []struct {
		name   string
		kind   string
		parent string
		want   reader.Header
	}{
		{
			name:   "class with parent",
			kind:   "class",
			parent: "ApplicationService",
			want:   reader.Header{Title: "Foo", Parent: "ApplicationService", RelPath: "app/services/foo.rb"},
		},
		{
			name:   "class without parent",
			kind:   "class",
			parent: "",
			want:   reader.Header{Title: "Foo", Parent: "", RelPath: "app/services/foo.rb"},
		},
		{
			// The load-bearing case: a module has no superclass, so even
			// when the caller passes a non-empty parentClass, it must not
			// end up in the Header -- otherwise Format would render the
			// invalid `module Foo < Bar`.
			name:   "module with parentClass supplied is dropped",
			kind:   "module",
			parent: "ApplicationService",
			want:   reader.Header{Title: "module Foo", Parent: "", RelPath: "app/services/foo.rb"},
		},
		{
			name:   "module without parent",
			kind:   "module",
			parent: "",
			want:   reader.Header{Title: "module Foo", Parent: "", RelPath: "app/services/foo.rb"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := reader.ClassOrModuleHeader(tt.kind, "Foo", tt.parent, "app/services/foo.rb")
			if got != tt.want {
				t.Errorf("ClassOrModuleHeader(%q, %q, %q, %q) = %+v, want %+v",
					tt.kind, "Foo", tt.parent, "app/services/foo.rb", got, tt.want)
			}
		})
	}
}

func TestFormat_ValueSection(t *testing.T) {
	k := reader.Kind{}
	out := k.Format(reader.Header{Title: "X"}, []reader.Section{{Label: "Queue", Value: "default"}}, term.Styler{})
	if !strings.Contains(out, "\nQueue:\n  default\n") {
		t.Errorf("expected rendered Value section:\n%s", out)
	}
}

func TestFormat_EntriesSectionRoutedThroughStyleEntry(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	k := reader.Kind{IsMacro: func(tok string) bool { return tok == "belongs_to" }}
	out := k.Format(reader.Header{Title: "X"}, []reader.Section{
		{Label: "Associations", Entries: []string{"  belongs_to :account"}},
	}, term.NewStyler(term.ModeAlways, nil))
	if !strings.Contains(out, "  \x1b[36mbelongs_to\x1b[0m :account\n") {
		t.Errorf("expected entry routed through StyleEntry with color accent:\n%s", out)
	}
}

func TestFormat_EntriesSectionNilIsMacroPassesThrough(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	k := reader.Kind{IsMacro: nil}
	out := k.Format(reader.Header{Title: "X"}, []reader.Section{
		{Label: "Associations", Entries: []string{"  belongs_to :account"}},
	}, term.NewStyler(term.ModeAlways, nil))
	if !strings.Contains(out, "  belongs_to :account\n") {
		t.Errorf("expected entry unstyled when IsMacro is nil:\n%s", out)
	}
	if strings.Contains(out, "\x1b[36m") {
		t.Errorf("expected no color accent when IsMacro is nil:\n%s", out)
	}
}

func TestFormat_EmptySectionSkipped(t *testing.T) {
	k := reader.Kind{}
	out := k.Format(reader.Header{Title: "X"}, []reader.Section{
		{Label: "Empty Value", Value: ""},
		{Label: "Empty Entries", Entries: nil},
		{Label: "Present", Value: "here"},
	}, term.Styler{})
	if strings.Contains(out, "Empty Value:") || strings.Contains(out, "Empty Entries:") {
		t.Errorf("expected empty sections to render nothing:\n%s", out)
	}
	if !strings.Contains(out, "Present:") {
		t.Errorf("expected non-empty section to render:\n%s", out)
	}
}

func TestFormat_SectionOrderPreserved(t *testing.T) {
	k := reader.Kind{}
	out := k.Format(reader.Header{Title: "X"}, []reader.Section{
		{Label: "Second", Value: "b"},
		{Label: "First", Value: "a"},
	}, term.Styler{})
	if strings.Index(out, "Second:") > strings.Index(out, "First:") {
		t.Errorf("expected sections rendered in the given order:\n%s", out)
	}
}

func TestFormat_TrailingNewline(t *testing.T) {
	k := reader.Kind{}
	out := k.Format(reader.Header{Title: "X", RelPath: "x.rb"}, nil, term.Styler{})
	if !strings.HasSuffix(out, "\n\n") {
		t.Errorf("expected output to end with a blank trailing line, got:\n%q", out)
	}
}
