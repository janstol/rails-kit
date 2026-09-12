package locales_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/janstol/rails-kit/internal/locales"
)

const testdataLocales = "../../testdata/config/locales"

func TestLoad(t *testing.T) {
	merged, err := locales.Load(testdataLocales)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if merged == nil {
		t.Fatal("got nil merged map")
	}
	if _, ok := merged["en"]; !ok {
		t.Error("missing 'en' top-level key")
	}
}

func TestListScopes(t *testing.T) {
	merged, err := locales.Load(testdataLocales)
	if err != nil {
		t.Fatal(err)
	}
	scopes := locales.ListScopes(merged)
	found := map[string]bool{}
	for _, s := range scopes {
		found[s] = true
	}
	if !found["en.activerecord.attributes"] {
		t.Error("expected en.activerecord.attributes scope")
	}
	if !found["en.activerecord.attributes.user"] {
		t.Error("expected en.activerecord.attributes.user scope")
	}
	if !found["en.views.users.show"] {
		t.Error("expected en.views.users.show scope")
	}
	if !found["en.views.users"] {
		t.Error("expected en.views.users scope")
	}
	if !found["en.views"] {
		t.Error("expected en.views scope")
	}
	if !found["en.activerecord"] {
		t.Error("expected en.activerecord scope")
	}
	if found["en"] {
		t.Error("did not expect en scope")
	}
	if !found["en.activerecord.models"] {
		t.Error("expected en.activerecord.models scope")
	}
}

func TestNavigate(t *testing.T) {
	merged, err := locales.Load(testdataLocales)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("valid scope", func(t *testing.T) {
		node, err := locales.Navigate(merged, "en.activerecord.models")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		m, ok := node.(map[string]interface{})
		if !ok {
			t.Fatalf("expected map, got %T", node)
		}
		if m["user"] != "User" {
			t.Errorf("expected 'User', got %v", m["user"])
		}
	})

	t.Run("missing scope", func(t *testing.T) {
		_, err := locales.Navigate(merged, "en.nonexistent.key")
		if err == nil {
			t.Error("expected error for missing scope")
		}
	})
}

func TestLoadNestedDir(t *testing.T) {
	merged, err := locales.Load(testdataLocales)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	en, ok := merged["en"].(map[string]interface{})
	if !ok {
		t.Fatal("missing 'en' top-level key")
	}
	if _, ok := en["nested_section"]; !ok {
		t.Error("expected nested_section from nested subdirectory to be loaded")
	}
}

func TestLoad_YamlExtension(t *testing.T) {
	merged, err := locales.Load(testdataLocales)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	en, ok := merged["en"].(map[string]interface{})
	if !ok {
		t.Fatal("missing 'en' top-level key")
	}
	if _, ok := en["yaml_section"]; !ok {
		t.Error("expected yaml_section from extra.yaml to be loaded")
	}
}

func TestLoadNonexistentDir(t *testing.T) {
	_, err := locales.Load("/nonexistent/path/to/locales")
	if err == nil {
		t.Fatal("expected error for nonexistent dir")
	}
}

func TestListScopesNonEnLocale(t *testing.T) {
	merged := map[string]interface{}{
		"en": map[string]interface{}{
			"views": map[string]interface{}{
				"users": map[string]interface{}{
					"title": "Users",
				},
			},
		},
		"cs": map[string]interface{}{
			"views": map[string]interface{}{
				"users": map[string]interface{}{
					"title": "Uživatelé",
				},
			},
		},
	}
	scopes := locales.ListScopes(merged)
	found := map[string]bool{}
	for _, s := range scopes {
		found[s] = true
	}
	if !found["en.views.users"] {
		t.Error("expected en.views.users scope")
	}
	if !found["en.views"] {
		t.Error("expected en.views scope")
	}
	if !found["cs.views.users"] {
		t.Error("expected cs.views.users scope")
	}
	if !found["cs.views"] {
		t.Error("expected cs.views scope")
	}
	if found["en"] {
		t.Error("did not expect en scope")
	}
	if found["cs"] {
		t.Error("did not expect cs scope")
	}
	if len(scopes) == 0 {
		t.Error("expected at least one scope")
	}
}

func TestListScopesIncludesShallowScopes(t *testing.T) {
	merged := map[string]interface{}{
		"en": map[string]interface{}{
			"errors": map[string]interface{}{
				"messages": map[string]interface{}{
					"blank": "can't be blank",
				},
			},
			"homepage": map[string]interface{}{
				"title": "Home",
			},
		},
		"cs": map[string]interface{}{
			"errors": map[string]interface{}{
				"messages": map[string]interface{}{
					"blank": "nesmi byt prazdne",
				},
			},
		},
	}

	scopes := locales.ListScopes(merged)
	found := map[string]bool{}
	for _, scope := range scopes {
		found[scope] = true
	}

	for _, want := range []string{
		"en.errors",
		"en.errors.messages",
		"en.homepage",
		"cs.errors",
		"cs.errors.messages",
	} {
		if !found[want] {
			t.Errorf("expected %s scope", want)
		}
	}
	if found["en"] {
		t.Error("did not expect en scope")
	}
	if found["cs"] {
		t.Error("did not expect cs scope")
	}
}

func TestPrintTree(t *testing.T) {
	merged, err := locales.Load(testdataLocales)
	if err != nil {
		t.Fatal(err)
	}
	node, err := locales.Navigate(merged, "en.activerecord.models")
	if err != nil {
		t.Fatal(err)
	}
	var sb strings.Builder
	locales.PrintTree(&sb, node, 0)
	out := sb.String()
	if !strings.Contains(out, "user: User") {
		t.Errorf("expected 'user: User' in output, got:\n%s", out)
	}
}

func TestPrintTree_ArrayOutput(t *testing.T) {
	value := map[string]interface{}{
		"labels": []interface{}{"first", "second"},
	}

	var sb strings.Builder
	locales.PrintTree(&sb, value, 0)
	out := sb.String()

	want := "labels:\n  - first\n  - second\n"
	if out != want {
		t.Fatalf("unexpected output:\n%s\nwant:\n%s", out, want)
	}
}

func TestPrintTree_NestedCompositeOutput(t *testing.T) {
	value := map[string]interface{}{
		"filters": []interface{}{
			map[string]interface{}{
				"name":   "status",
				"values": []interface{}{"active", "archived"},
			},
		},
	}

	var sb strings.Builder
	locales.PrintTree(&sb, value, 0)
	out := sb.String()

	for _, fragment := range []string{
		"filters:\n",
		"  -\n",
		"    name: status\n",
		"    values:\n",
		"      - active\n",
		"      - archived\n",
	} {
		if !strings.Contains(out, fragment) {
			t.Fatalf("expected fragment %q in output:\n%s", fragment, out)
		}
	}
}

func TestLoadEmptyDir(t *testing.T) {
	dir := t.TempDir()

	merged, err := locales.Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(merged) != 0 {
		t.Fatalf("expected empty map, got %d keys", len(merged))
	}
}

func TestLoadMalformedYAML(t *testing.T) {
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "en.yml"), []byte("en:\n  key: value\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "broken.yml"), []byte(":\t invalid yaml: [\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := locales.Load(dir)
	if err == nil {
		t.Fatal("expected error for malformed YAML")
	}
	if !strings.Contains(err.Error(), "parsing") {
		t.Errorf("expected 'parsing' in error, got: %v", err)
	}
}

func TestLoad_NumericKeyDeepMerge(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.yml"), []byte(strings.Join([]string{
		"en:",
		"  status:",
		"    404: Not Found",
		"    500: Server Error",
		"",
	}, "\n")), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "z.yml"), []byte(strings.Join([]string{
		"en:",
		"  status:",
		"    503: Service Unavailable",
		"",
	}, "\n")), 0o644); err != nil {
		t.Fatal(err)
	}

	merged, err := locales.Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	status, err := locales.Navigate(merged, "en.status")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m, ok := status.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map, got %T", status)
	}
	for key, want := range map[string]string{
		"404": "Not Found",
		"500": "Server Error",
		"503": "Service Unavailable",
	} {
		if m[key] != want {
			t.Errorf("status[%q] = %v, want %q", key, m[key], want)
		}
	}
}

func TestNavigate_NumericKey(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "en.yml"), []byte(strings.Join([]string{
		"en:",
		"  status:",
		"    404:",
		"      title: Not Found",
		"",
	}, "\n")), 0o644); err != nil {
		t.Fatal(err)
	}

	merged, err := locales.Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	node, err := locales.Navigate(merged, "en.status.404")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m, ok := node.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map, got %T", node)
	}
	if m["title"] != "Not Found" {
		t.Errorf("title = %v, want 'Not Found'", m["title"])
	}
}

func TestListScopes_NumericKeyScope(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "en.yml"), []byte(strings.Join([]string{
		"en:",
		"  status:",
		"    404:",
		"      title: Not Found",
		"",
	}, "\n")), 0o644); err != nil {
		t.Fatal(err)
	}

	merged, err := locales.Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	scopes := locales.ListScopes(merged)
	found := map[string]bool{}
	for _, s := range scopes {
		found[s] = true
	}
	if !found["en.status"] {
		t.Error("expected en.status scope")
	}
	if !found["en.status.404"] {
		t.Error("expected en.status.404 scope")
	}
}

func TestNavigate_KeyRendering(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "en.yml"), []byte(strings.Join([]string{
		"en:",
		"  labels:",
		"    true: Yes",
		"    false: No",
		"    ~: Unset",
		"    3.14: Pi",
		"    2026-01-01: New Year",
		"",
	}, "\n")), 0o644); err != nil {
		t.Fatal(err)
	}

	merged, err := locales.Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	node, err := locales.Navigate(merged, "en.labels")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m, ok := node.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map, got %T", node)
	}
	for key, want := range map[string]string{
		"true":       "Yes",
		"false":      "No",
		"null":       "Unset",
		"3.14":       "Pi",
		"2026-01-01": "New Year",
	} {
		if m[key] != want {
			t.Errorf("labels[%q] = %v, want %q", key, m[key], want)
		}
	}
}

func TestLoad_KeyCollisionStringWins(t *testing.T) {
	// "True" (unquoted, resolves to the bool true) and "true" (quoted, a
	// plain string) both render as the "true" key after normalization.
	// They're spelled differently in the source ("True" vs "true"), so
	// yaml.v3's own duplicate-key check -- which compares raw scalar text,
	// not resolved value -- doesn't reject the file; the collision only
	// appears after normalizeKey.
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "en.yml"), []byte(strings.Join([]string{
		"en:",
		"  labels:",
		"    True: Numeric",
		`    "true": String`,
		"",
	}, "\n")), 0o644); err != nil {
		t.Fatal(err)
	}

	for i := range 20 {
		merged, err := locales.Load(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		node, err := locales.Navigate(merged, "en.labels")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		m, ok := node.(map[string]interface{})
		if !ok {
			t.Fatalf("expected map, got %T", node)
		}
		if m["true"] != "String" {
			t.Fatalf("run %d: labels[true] = %v, want String (the string-authored key must win)", i, m["true"])
		}
	}
}

func TestLoad_TopLevelShapes(t *testing.T) {
	t.Run("sequence root errors", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "bad.yml"), []byte("- one\n- two\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		_, err := locales.Load(dir)
		if err == nil {
			t.Fatal("expected error for sequence root")
		}
		if !strings.Contains(err.Error(), "parsing") {
			t.Errorf("expected 'parsing' in error, got: %v", err)
		}
	})

	t.Run("scalar root errors", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "bad.yml"), []byte("just a string\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		_, err := locales.Load(dir)
		if err == nil {
			t.Fatal("expected error for scalar root")
		}
		if !strings.Contains(err.Error(), "parsing") {
			t.Errorf("expected 'parsing' in error, got: %v", err)
		}
	})

	t.Run("empty file loads clean", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "empty.yml"), []byte(""), 0o644); err != nil {
			t.Fatal(err)
		}
		merged, err := locales.Load(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(merged) != 0 {
			t.Fatalf("expected empty map, got %d keys", len(merged))
		}
	})

	t.Run("comments-only file loads clean", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "comments.yml"), []byte("# just a comment\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		merged, err := locales.Load(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(merged) != 0 {
			t.Fatalf("expected empty map, got %d keys", len(merged))
		}
	})
}

func TestLoad_AnchorsAliasesAndMergeKeys(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "en.yml"), []byte(strings.Join([]string{
		"defaults: &defaults",
		"  greeting: Hello",
		"about_title: &about_title About Us",
		"en:",
		"  home:",
		"    <<: *defaults",
		"    title: Home",
		"  about:",
		"    title: *about_title",
		"",
	}, "\n")), 0o644); err != nil {
		t.Fatal(err)
	}

	merged, err := locales.Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	home, err := locales.Navigate(merged, "en.home")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	homeMap, ok := home.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map, got %T", home)
	}
	if homeMap["greeting"] != "Hello" {
		t.Errorf("greeting = %v, want Hello (merge key << should resolve)", homeMap["greeting"])
	}
	if homeMap["title"] != "Home" {
		t.Errorf("title = %v, want Home", homeMap["title"])
	}

	about, err := locales.Navigate(merged, "en.about")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	aboutMap, ok := about.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map, got %T", about)
	}
	if aboutMap["title"] != "About Us" {
		t.Errorf("title = %v, want About Us (alias should resolve)", aboutMap["title"])
	}
}

func TestLoadUnreadableFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod has no read-blocking semantics on Windows")
	}
	if os.Getuid() == 0 {
		t.Skip("running as root; chmod 0000 has no effect")
	}

	dir := t.TempDir()
	f := filepath.Join(dir, "en.yml")
	if err := os.WriteFile(f, []byte("en:\n  key: value\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(f, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(f, 0o644) })

	_, err := locales.Load(dir)
	if err == nil {
		t.Fatal("expected error for unreadable file")
	}
	if !strings.Contains(err.Error(), "reading") {
		t.Errorf("expected 'reading' in error, got: %v", err)
	}
}
