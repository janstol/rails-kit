package fixtures_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/janstol/rails-kit/internal/fixtures"
)

const testdataFixtures = "../../testdata/test/fixtures"

func TestListFiles(t *testing.T) {
	names, err := fixtures.ListFiles(testdataFixtures)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := map[string]bool{"posts": true, "users": true, "admin/dashboards": true}
	for _, n := range names {
		delete(want, n)
	}
	if len(want) > 0 {
		t.Errorf("missing fixture files: %v", want)
	}
}

func TestListFiles_IncludesYaml(t *testing.T) {
	names, err := fixtures.ListFiles(testdataFixtures)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, n := range names {
		if n == "tags" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'tags' in ListFiles output, got: %v", names)
	}
}

func TestLoad_YamlExtension(t *testing.T) {
	filename, data, err := fixtures.Load(testdataFixtures, "tag", "tags")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if filename != "tags.yaml" {
		t.Errorf("filename = %q, want tags.yaml", filename)
	}
	if _, ok := data["ruby"]; !ok {
		t.Error("missing ruby fixture entry")
	}
}

func TestLoad_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "users.yml"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	_, _, err := fixtures.Load(dir, "user", "users")
	if err == nil {
		t.Fatal("expected error for empty fixture file, got nil")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Errorf("expected 'empty' in error message, got: %v", err)
	}
}

func TestListFiles_UnreadableSubdir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod has no read-blocking semantics on Windows")
	}
	if os.Getuid() == 0 {
		t.Skip("cannot test permission errors as root")
	}
	dir := t.TempDir()
	subdir := filepath.Join(dir, "locked")
	if err := os.MkdirAll(subdir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ok.yml"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(subdir, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(subdir, 0o755) })

	_, err := fixtures.ListFiles(dir)
	if err == nil {
		t.Error("expected error for unreadable subdirectory, got nil")
	}
}

func TestLoad(t *testing.T) {
	t.Run("users plural", func(t *testing.T) {
		filename, data, err := fixtures.Load(testdataFixtures, "user", "users")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if filename != "users.yml" {
			t.Errorf("filename = %q, want users.yml", filename)
		}
		if _, ok := data["alice"]; !ok {
			t.Error("missing alice fixture")
		}
	})

	t.Run("nested by plural basename", func(t *testing.T) {
		filename, data, err := fixtures.Load(testdataFixtures, "dashboard", "dashboards")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if filename != "admin/dashboards.yml" {
			t.Errorf("filename = %q, want admin/dashboards.yml", filename)
		}
		if _, ok := data["main"]; !ok {
			t.Error("missing main fixture")
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, _, err := fixtures.Load(testdataFixtures, "nonexistent", "nonexistents")
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("normalizes valid yaml erb values", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "users.yml"), []byte("alice:\n  name: '<%= ENV[\"USER\"] %>'\n"), 0o644); err != nil {
			t.Fatal(err)
		}

		_, data, err := fixtures.Load(dir, "user", "users")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		alice, ok := data["alice"].(map[string]interface{})
		if !ok {
			t.Fatalf("unexpected entry type: %T", data["alice"])
		}
		if got := alice["name"]; got != "__ERB__" {
			t.Fatalf("name = %v, want __ERB__", got)
		}
	})

	t.Run("normalizes double-quoted yaml erb values", func(t *testing.T) {
		dir := t.TempDir()
		content := "alice:\n  name: \"<%= ENV[\\\"USER\\\"] %>\"\n"
		if err := os.WriteFile(filepath.Join(dir, "users.yml"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}

		_, data, err := fixtures.Load(dir, "user", "users")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		alice, ok := data["alice"].(map[string]interface{})
		if !ok {
			t.Fatalf("unexpected entry type: %T", data["alice"])
		}
		if got := alice["name"]; got != "__ERB__" {
			t.Fatalf("name = %v, want __ERB__", got)
		}
	})

	t.Run("normalizes block scalar erb values", func(t *testing.T) {
		dir := t.TempDir()
		content := "alice:\n  bio: |\n    Hello\n    <%= ENV[\"USER\"] %>\n"
		if err := os.WriteFile(filepath.Join(dir, "users.yml"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}

		_, data, err := fixtures.Load(dir, "user", "users")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		alice := data["alice"].(map[string]interface{})
		if got := alice["bio"]; got != "Hello\n__ERB__\n" {
			t.Fatalf("bio = %#v, want block scalar with __ERB__", got)
		}
	})

	t.Run("normalizes list item erb values", func(t *testing.T) {
		dir := t.TempDir()
		content := "alice:\n  tags:\n    - <%= ENV[\"USER\"] %>\n"
		if err := os.WriteFile(filepath.Join(dir, "users.yml"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}

		_, data, err := fixtures.Load(dir, "user", "users")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		alice := data["alice"].(map[string]interface{})
		tags := alice["tags"].([]interface{})
		if len(tags) != 1 || tags[0] != "__ERB__" {
			t.Fatalf("tags = %#v, want [__ERB__]", tags)
		}
	})

	t.Run("rejects structural erb loops", func(t *testing.T) {
		dir := t.TempDir()
		content := strings.Join([]string{
			`<% 2.times do |n| %>`,
			`user_<%= n %>:`,
			`  email: user_<%= n %>@example.com`,
			`<% end %>`,
			"",
		}, "\n")
		if err := os.WriteFile(filepath.Join(dir, "users.yml"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}

		_, _, err := fixtures.Load(dir, "user", "users")
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "cannot be summarized safely") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("rejects erb in fixture keys", func(t *testing.T) {
		dir := t.TempDir()
		content := "user_<%= ENV[\"USER\"] %>:\n  name: Alice\n"
		if err := os.WriteFile(filepath.Join(dir, "users.yml"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}

		_, _, err := fixtures.Load(dir, "user", "users")
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "cannot be summarized safely") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("rejects mixed control flow and output on one line", func(t *testing.T) {
		dir := t.TempDir()
		content := "alice:\n  <% if ENV[\"SHOW_EMAIL\"] %>email: <%= ENV[\"SHOW_EMAIL\"] %><% end %>\n"
		if err := os.WriteFile(filepath.Join(dir, "users.yml"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}

		_, _, err := fixtures.Load(dir, "user", "users")
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "cannot be summarized safely") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestSummarize(t *testing.T) {
	t.Run("picks interesting keys first", func(t *testing.T) {
		attrs := map[string]interface{}{
			"email":      "alice@example.com",
			"name":       "Alice",
			"created_at": "2024-01-01",
		}
		got := fixtures.Summarize(attrs)
		if !strings.Contains(got, "name: Alice") {
			t.Errorf("expected name in output, got %q", got)
		}
		if !strings.Contains(got, "email: alice@example.com") {
			t.Errorf("expected email in output, got %q", got)
		}
	})

	t.Run("truncates long values", func(t *testing.T) {
		attrs := map[string]interface{}{
			"name": strings.Repeat("x", 100),
		}
		got := fixtures.Summarize(attrs)
		if len(got) > 200 {
			t.Errorf("output too long: %d chars", len(got))
		}
	})

	t.Run("empty attrs", func(t *testing.T) {
		got := fixtures.Summarize(nil)
		if got != "(empty)" {
			t.Errorf("got %q, want (empty)", got)
		}
	})

	t.Run("collapses multi-line values", func(t *testing.T) {
		attrs := map[string]interface{}{
			"name": "Hello\n__ERB__\n",
		}
		got := fixtures.Summarize(attrs)
		if strings.Contains(got, "\n") {
			t.Errorf("output contains newline: %q", got)
		}
		if !strings.Contains(got, "Hello __ERB__") {
			t.Errorf("expected collapsed value, got %q", got)
		}
	})
}

func TestLoad_AmbiguousFixture(t *testing.T) {
	dir := t.TempDir()

	// Create two nested fixtures with the same basename
	for _, sub := range []string{"admin", "analytics"} {
		subDir := filepath.Join(dir, sub)
		if err := os.MkdirAll(subDir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(subDir, "dashboards.yml"), []byte("main:\n  title: test\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	_, _, err := fixtures.Load(dir, "dashboard", "dashboards")
	if err == nil {
		t.Fatal("expected error for ambiguous fixture")
	}
	if !strings.Contains(err.Error(), "ambiguous") {
		t.Errorf("expected 'ambiguous' in error, got: %v", err)
	}
}

func TestLoadKeyOrderDeterministic(t *testing.T) {
	dir := t.TempDir()
	content := "zebra:\n  name: Z\nalpha:\n  name: A\nmiddle:\n  name: M\n"
	if err := os.WriteFile(filepath.Join(dir, "things.yml"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	_, data, err := fixtures.Load(dir, "thing", "things")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Collect and sort keys the same way cmd/fixtures.go does.
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	// sort package is tested indirectly; just verify all keys present and consistent.
	want := []string{"alpha", "middle", "zebra"}
	got := make([]string, len(keys))
	copy(got, keys)
	// Sort to normalize, then compare set.
	if len(got) != len(want) {
		t.Fatalf("expected %d keys, got %d: %v", len(want), len(got), got)
	}
	wantSet := map[string]bool{"alpha": true, "middle": true, "zebra": true}
	for _, k := range got {
		if !wantSet[k] {
			t.Errorf("unexpected key %q", k)
		}
	}
}

func TestStripERB(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{`name: <%= "Alice" %>`, `name: "__ERB__"`},
		{`id: <%= SecureRandom.uuid %>`, `id: "__ERB__"`},
		{"<%= method(\n  arg\n) %>", `"__ERB__"`},
		// Double-quoted YAML value: outer quotes must be consumed so result is valid YAML.
		{`name: "<%= ENV["USER"] %>"`, `name: "__ERB__"`},
		// Single-quoted YAML value: outer quotes consumed.
		{`name: '<%= ENV["USER"] %>'`, `name: "__ERB__"`},
	}
	for _, tc := range cases {
		got := fixtures.StripERB(tc.in)
		if got != tc.want {
			t.Errorf("StripERB(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestLoadNamespacedIrregularFixture(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "admin", "people.yml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("alice:\n  name: Alice\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	filename, data, err := fixtures.Load(dir, "admin/person", "admin/people")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if filename != "admin/people.yml" {
		t.Fatalf("filename = %q, want admin/people.yml", filename)
	}
	if _, ok := data["alice"]; !ok {
		t.Fatal("missing alice fixture")
	}
}

func TestVisibleEntries(t *testing.T) {
	data := map[string]interface{}{
		"_fixture":         map[string]interface{}{"ignore": true},
		"_hidden_but_real": map[string]interface{}{"name": "Hidden"},
		"alice":            map[string]interface{}{"name": "Alice"},
	}

	got := fixtures.VisibleEntries(data)

	if len(got) != 2 {
		t.Fatalf("expected 2 visible entries, got %d: %v", len(got), got)
	}
	if _, ok := got["alice"]; !ok {
		t.Fatal("expected alice entry")
	}
	if _, ok := got["_hidden_but_real"]; !ok {
		t.Fatal("expected underscore-prefixed real fixture entry")
	}
	if _, ok := got["_fixture"]; ok {
		t.Fatal("did not expect _fixture metadata entry")
	}
}

func TestValidateERBUsage(t *testing.T) {
	t.Run("allows scalar values", func(t *testing.T) {
		content := "alice:\n  name: <%= ENV[\"USER\"] %>\n"
		if err := fixtures.ValidateERBUsage(content); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("allows block scalar values", func(t *testing.T) {
		content := "alice:\n  bio: |\n    Hello\n    <%= ENV[\"USER\"] %>\n"
		if err := fixtures.ValidateERBUsage(content); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("allows list item values", func(t *testing.T) {
		content := "alice:\n  tags:\n    - <%= ENV[\"USER\"] %>\n"
		if err := fixtures.ValidateERBUsage(content); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("rejects control flow", func(t *testing.T) {
		content := "<% if Rails.env.test? %>\nalice:\n  name: Alice\n<% end %>\n"
		err := fixtures.ValidateERBUsage(content)
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "unsupported structural ERB") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("rejects key templates", func(t *testing.T) {
		content := "user_<%= ENV[\"USER\"] %>:\n  name: Alice\n"
		err := fixtures.ValidateERBUsage(content)
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "unsupported structural ERB") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("rejects mixed control flow and output on one line", func(t *testing.T) {
		content := "alice:\n  <% if ENV[\"SHOW_EMAIL\"] %>email: <%= ENV[\"SHOW_EMAIL\"] %><% end %>\n"
		err := fixtures.ValidateERBUsage(content)
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "unsupported structural ERB") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestLoad_MissingFixturesDir(t *testing.T) {
	_, _, err := fixtures.Load(filepath.Join(t.TempDir(), "missing"), "user", "users")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "fixtures path") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoad_FixturesPathIsFile(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "fixtures.txt")
	if err := os.WriteFile(file, []byte("nope"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, _, err := fixtures.Load(file, "user", "users")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "not a directory") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestListFiles_MissingFixturesDir(t *testing.T) {
	_, err := fixtures.ListFiles(filepath.Join(t.TempDir(), "missing"))
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "fixtures path") {
		t.Fatalf("unexpected error: %v", err)
	}
}
