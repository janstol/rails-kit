package schema_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/janstol/rails-kit/internal/schema"
	"github.com/janstol/rails-kit/internal/term"
)

const testdataStructureSQLPath = "../../testdata/db/structure.sql"

var reHighlightANSI = regexp.MustCompile("\x1b\\[[0-9;]*m")

func enabledStyler(t *testing.T) term.Styler {
	t.Helper()
	t.Setenv("NO_COLOR", "")
	return term.NewStyler(term.ModeAlways, nil)
}

func TestHighlight_Disabled(t *testing.T) {
	s, err := schema.Parse(testdataSchema)
	if err != nil {
		t.Fatal(err)
	}
	out, err := s.ExtractTables([]string{"users"})
	if err != nil {
		t.Fatal(err)
	}

	got := schema.Highlight(testdataSchema, out, term.Styler{})
	if got != out {
		t.Errorf("Highlight with a disabled styler must pass through unchanged:\ngot:  %q\nwant: %q", got, out)
	}
}

func TestHighlight_SQLIsNoOp(t *testing.T) {
	out := `CREATE TABLE "users" (` + "\n" + `    id bigint NOT NULL` + "\n" + `);` + "\n"

	got := schema.Highlight(testdataStructureSQLPath, out, enabledStyler(t))
	if got != out {
		t.Errorf(".sql input must pass through unchanged even with an enabled styler:\ngot:  %q\nwant: %q", got, out)
	}
}

func TestHighlight_CreateTable(t *testing.T) {
	st := enabledStyler(t)
	s, err := schema.Parse(testdataSchema)
	if err != nil {
		t.Fatal(err)
	}
	out, err := s.ExtractTables([]string{"users"})
	if err != nil {
		t.Fatal(err)
	}

	got := schema.Highlight(testdataSchema, out, st)

	if !strings.Contains(got, st.Cyan("create_table")) {
		t.Errorf("expected styled create_table keyword:\n%s", got)
	}
	if !strings.Contains(got, st.Bold("users")) {
		t.Errorf("expected styled table name:\n%s", got)
	}
	if stripped := reHighlightANSI.ReplaceAllString(got, ""); stripped != out {
		t.Errorf("stripping ANSI from highlighted output does not reproduce uncolored output:\n--- got ---\n%s\n--- want ---\n%s", stripped, out)
	}
}

func TestHighlight_AddIndexAndForeignKey(t *testing.T) {
	st := enabledStyler(t)
	s, err := schema.Parse(testdataSchema)
	if err != nil {
		t.Fatal(err)
	}
	out, err := s.ExtractTables([]string{"comments"})
	if err != nil {
		t.Fatal(err)
	}

	got := schema.Highlight(testdataSchema, out, st)

	if !strings.Contains(got, st.Cyan("add_index")) {
		t.Errorf("expected styled add_index keyword:\n%s", got)
	}
	if !strings.Contains(got, st.Cyan("add_foreign_key")) {
		t.Errorf("expected styled add_foreign_key keyword:\n%s", got)
	}
	if !strings.Contains(got, st.Bold("comments")) {
		t.Errorf("expected styled table name in add_index/add_foreign_key lines:\n%s", got)
	}
	if stripped := reHighlightANSI.ReplaceAllString(got, ""); stripped != out {
		t.Errorf("stripping ANSI from highlighted output does not reproduce uncolored output:\n--- got ---\n%s\n--- want ---\n%s", stripped, out)
	}
}

func TestHighlight_CreateJoinTable(t *testing.T) {
	st := enabledStyler(t)
	s, err := schema.Parse(testdataSchema)
	if err != nil {
		t.Fatal(err)
	}
	out, err := s.ExtractTables([]string{"posts_tags"})
	if err != nil {
		t.Fatal(err)
	}

	got := schema.Highlight(testdataSchema, out, st)

	if !strings.Contains(got, st.Cyan("create_join_table")) {
		t.Errorf("expected styled create_join_table keyword:\n%s", got)
	}
	if stripped := reHighlightANSI.ReplaceAllString(got, ""); stripped != out {
		t.Errorf("stripping ANSI from highlighted output does not reproduce uncolored output:\n--- got ---\n%s\n--- want ---\n%s", stripped, out)
	}
}

func TestHighlight_LineWithoutKeywordUntouched(t *testing.T) {
	st := enabledStyler(t)
	line := `    t.bigint "user_id", null: false` + "\n"

	got := schema.Highlight(testdataSchema, line, st)
	if got != line {
		t.Errorf("a column-definition line with no DDL keyword must be untouched:\ngot:  %q\nwant: %q", got, line)
	}
}
