package schema_test

import (
	"strings"
	"testing"

	"github.com/janstol/rails-kit/internal/schema"
)

const testdataStructureSQL = "../../testdata/db/structure.sql"

func TestSQLListTables(t *testing.T) {
	tables, err := schema.ListTables(testdataStructureSQL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"comments", "posts", "posts_tags", "tags", "users"}
	if len(tables) != len(want) {
		t.Fatalf("got %v, want %v", tables, want)
	}
	for i, w := range want {
		if tables[i] != w {
			t.Errorf("tables[%d] = %q, want %q", i, tables[i], w)
		}
	}
}

func TestSQLExcludesInternalTables(t *testing.T) {
	tables, err := schema.ListTables(testdataStructureSQL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, name := range tables {
		if name == "schema_migrations" || name == "ar_internal_metadata" {
			t.Errorf("internal table %q should not appear in ListTables output", name)
		}
	}
}

func TestSQLExtractTables(t *testing.T) {
	t.Run("single table", func(t *testing.T) {
		out, err := schema.ExtractTables(testdataStructureSQL, []string{"users"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(out, "CREATE TABLE users") {
			t.Error("output does not contain CREATE TABLE users")
		}
		if !strings.Contains(out, "index_users_on_email") {
			t.Error("output does not contain email index")
		}
	})

	t.Run("table with foreign keys", func(t *testing.T) {
		out, err := schema.ExtractTables(testdataStructureSQL, []string{"comments"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(out, "FOREIGN KEY") {
			t.Error("output does not contain FOREIGN KEY for comments")
		}
		if !strings.Contains(out, "posts") {
			t.Error("output does not reference posts in foreign key")
		}
	})

	t.Run("unknown table", func(t *testing.T) {
		_, err := schema.ExtractTables(testdataStructureSQL, []string{"nonexistent"})
		if err == nil {
			t.Error("expected error for unknown table")
		}
	})

	t.Run("multiple tables", func(t *testing.T) {
		out, err := schema.ExtractTables(testdataStructureSQL, []string{"users", "posts"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(out, "CREATE TABLE users") {
			t.Error("missing users table")
		}
		if !strings.Contains(out, "CREATE TABLE posts") {
			t.Error("missing posts table")
		}
	})

	t.Run("join table", func(t *testing.T) {
		out, err := schema.ExtractTables(testdataStructureSQL, []string{"posts_tags"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(out, "CREATE TABLE posts_tags") {
			t.Error("output does not contain CREATE TABLE posts_tags")
		}
		if !strings.Contains(out, "index_posts_tags_on_post_id_and_tag_id") {
			t.Error("output does not contain unique index for posts_tags")
		}
	})
}

func TestSQLExtractTableMap(t *testing.T) {
	t.Run("single table", func(t *testing.T) {
		result, err := schema.ExtractTableMap(testdataStructureSQL, []string{"users"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(result) != 1 {
			t.Fatalf("expected 1 entry, got %d", len(result))
		}
		v, ok := result["users"]
		if !ok {
			t.Fatal("missing 'users' key")
		}
		if !strings.Contains(v, "CREATE TABLE users") {
			t.Error("output does not contain CREATE TABLE users")
		}
	})

	t.Run("unknown table", func(t *testing.T) {
		_, err := schema.ExtractTableMap(testdataStructureSQL, []string{"nonexistent"})
		if err == nil {
			t.Error("expected error for unknown table")
		}
	})
}

func TestSQLForeignKeyMultiLine(t *testing.T) {
	out, err := schema.ExtractTables(testdataStructureSQL, []string{"comments"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// The FK statement should be multi-line (ALTER TABLE ... \n    ADD CONSTRAINT ...)
	if !strings.Contains(out, "ALTER TABLE ONLY comments") {
		t.Error("output does not contain ALTER TABLE ONLY comments")
	}
	if !strings.Contains(out, "ADD CONSTRAINT") {
		t.Error("output does not contain ADD CONSTRAINT")
	}
	// Both FKs for comments should be present
	if !strings.Contains(out, "REFERENCES posts") {
		t.Error("missing FK to posts")
	}
	if !strings.Contains(out, "REFERENCES users") {
		t.Error("missing FK to users")
	}
}
