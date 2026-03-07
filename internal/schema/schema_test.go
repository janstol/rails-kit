package schema_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/janstol/rails-kit/internal/schema"
)

const testdataSchema = "../../testdata/db/schema.rb"

func TestListTables(t *testing.T) {
	tables, err := schema.ListTables(testdataSchema)
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

func TestExtractJoinTable(t *testing.T) {
	out, err := schema.ExtractTables(testdataSchema, []string{"posts_tags"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "create_join_table") {
		t.Error("output does not contain create_join_table")
	}
	if !strings.Contains(out, "posts") || !strings.Contains(out, "tags") {
		t.Error("output does not reference posts and tags")
	}
}

func TestJoinTableNameOrdering(t *testing.T) {
	got := schema.JoinTableName("tags", "posts")
	if got != "posts_tags" {
		t.Errorf("JoinTableName(tags, posts) = %q, want posts_tags", got)
	}
	got2 := schema.JoinTableName("posts", "tags")
	if got2 != "posts_tags" {
		t.Errorf("JoinTableName(posts, tags) = %q, want posts_tags", got2)
	}
}

func TestExtractTables(t *testing.T) {
	t.Run("single table", func(t *testing.T) {
		out, err := schema.ExtractTables(testdataSchema, []string{"users"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(out, `create_table "users"`) {
			t.Error("output does not contain create_table users")
		}
		if !strings.Contains(out, `index_users_on_email`) {
			t.Error("output does not contain email index")
		}
	})

	t.Run("table with foreign keys", func(t *testing.T) {
		out, err := schema.ExtractTables(testdataSchema, []string{"comments"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(out, `add_foreign_key "comments"`) {
			t.Error("output does not contain foreign keys for comments")
		}
	})

	t.Run("unknown table", func(t *testing.T) {
		_, err := schema.ExtractTables(testdataSchema, []string{"nonexistent"})
		if err == nil {
			t.Error("expected error for unknown table")
		}
	})

	t.Run("multiple tables", func(t *testing.T) {
		out, err := schema.ExtractTables(testdataSchema, []string{"users", "posts"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(out, `create_table "users"`) {
			t.Error("missing users table")
		}
		if !strings.Contains(out, `create_table "posts"`) {
			t.Error("missing posts table")
		}
	})
}

func TestExtractTables_IgnoresDoEndInStringsAndComments(t *testing.T) {
	dir := t.TempDir()
	schemaPath := filepath.Join(dir, "schema.rb")
	content := strings.Join([]string{
		`ActiveRecord::Schema[7.2].define(version: 2024_01_01_000001) do`,
		`  create_table "events", force: :cascade do |t|`,
		`    t.string "starts_at_label", default: "end date"`,
		`    t.string "workflow_state", default: "do not publish"`,
		`    t.string "notes", comment: "do not show before end"`,
		`    # do not change this column before the end`,
		`  end`,
		`end`,
		"",
	}, "\n")
	if err := os.WriteFile(schemaPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := schema.ExtractTables(schemaPath, []string{"events"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, `t.string "notes", comment: "do not show before end"`) {
		t.Fatalf("expected full table block, got:\n%s", out)
	}
	if !strings.Contains(out, "\n  end\n") {
		t.Fatalf("expected closing end in output, got:\n%s", out)
	}
}

func TestExtractTables_TracksRealNestedBlocks(t *testing.T) {
	dir := t.TempDir()
	schemaPath := filepath.Join(dir, "schema.rb")
	content := strings.Join([]string{
		`ActiveRecord::Schema[7.2].define(version: 2024_01_01_000001) do`,
		`  create_table "widgets", force: :cascade do |t|`,
		`    t.check_constraint "price > 0", name: "positive_price" do`,
		`      "price_positive"`,
		`    end`,
		`    t.string "name"`,
		`  end`,
		`end`,
		"",
	}, "\n")
	if err := os.WriteFile(schemaPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := schema.ExtractTables(schemaPath, []string{"widgets"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, `t.check_constraint "price > 0", name: "positive_price" do`) {
		t.Fatalf("missing nested block line:\n%s", out)
	}
	if !strings.Contains(out, `t.string "name"`) {
		t.Fatalf("expected lines after nested block, got:\n%s", out)
	}
}

func TestListTables_LongLine(t *testing.T) {
	dir := t.TempDir()
	schemaPath := filepath.Join(dir, "schema.rb")
	longDefault := strings.Repeat("x", 70*1024)
	content := strings.Join([]string{
		`ActiveRecord::Schema[7.2].define(version: 2024_01_01_000001) do`,
		`  create_table "logs", force: :cascade do |t|`,
		`    t.text "payload", default: "` + longDefault + `"`,
		`  end`,
		`end`,
		"",
	}, "\n")
	if err := os.WriteFile(schemaPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	tables, err := schema.ListTables(schemaPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tables) != 1 || tables[0] != "logs" {
		t.Fatalf("unexpected tables: %v", tables)
	}
}

func TestExtractTableMap(t *testing.T) {
	t.Run("single table", func(t *testing.T) {
		result, err := schema.ExtractTableMap(testdataSchema, []string{"users"})
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
		if !strings.Contains(v, `create_table "users"`) {
			t.Error("output does not contain create_table users")
		}
		if !strings.Contains(v, `index_users_on_email`) {
			t.Error("output does not contain email index")
		}
	})

	t.Run("multiple tables", func(t *testing.T) {
		result, err := schema.ExtractTableMap(testdataSchema, []string{"users", "posts"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(result) != 2 {
			t.Fatalf("expected 2 entries, got %d", len(result))
		}
		if !strings.Contains(result["users"], `create_table "users"`) {
			t.Error("missing users table block")
		}
		if !strings.Contains(result["posts"], `create_table "posts"`) {
			t.Error("missing posts table block")
		}
	})

	t.Run("table with foreign keys", func(t *testing.T) {
		result, err := schema.ExtractTableMap(testdataSchema, []string{"comments"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(result["comments"], `add_foreign_key "comments"`) {
			t.Error("output does not contain foreign keys for comments")
		}
	})

	t.Run("unknown table", func(t *testing.T) {
		_, err := schema.ExtractTableMap(testdataSchema, []string{"nonexistent"})
		if err == nil {
			t.Error("expected error for unknown table")
		}
	})
}

func TestJoinTableWithExplicitTableName(t *testing.T) {
	dir := t.TempDir()
	schemaPath := filepath.Join(dir, "schema.rb")
	content := strings.Join([]string{
		`ActiveRecord::Schema[7.2].define(version: 2024_01_01_000001) do`,
		`  create_join_table :users, :roles, table_name: :memberships do |t|`,
		`    t.index ["user_id", "role_id"]`,
		`  end`,
		`  add_index "memberships", ["user_id"], name: "index_memberships_on_user_id"`,
		`  add_foreign_key "memberships", "users"`,
		`end`,
		"",
	}, "\n")
	if err := os.WriteFile(schemaPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	tables, err := schema.ListTables(schemaPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tables) != 1 || tables[0] != "memberships" {
		t.Fatalf("unexpected tables: %v", tables)
	}

	out, err := schema.ExtractTables(schemaPath, []string{"memberships"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, `create_join_table :users, :roles, table_name: :memberships`) {
		t.Fatalf("missing join table block:\n%s", out)
	}
	if !strings.Contains(out, `add_index "memberships"`) {
		t.Fatalf("missing memberships index:\n%s", out)
	}
	if !strings.Contains(out, `add_foreign_key "memberships"`) {
		t.Fatalf("missing memberships foreign key:\n%s", out)
	}
}
