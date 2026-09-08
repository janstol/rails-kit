package reader_test

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/janstol/rails-kit/internal/reader"
	"github.com/janstol/rails-kit/internal/term"
)

var errAmbiguousTestName = errors.New("ambiguous test name")

func suffixedKind() reader.Kind {
	return reader.Kind{
		Noun:         "job",
		Plural:       "jobs",
		Suffix:       "_job",
		ErrAmbiguous: errAmbiguousTestName,
		IsMacro:      func(tok string) bool { return tok == "retry_on" },
	}
}

func altSuffixKind() reader.Kind {
	return reader.Kind{
		Noun:         "former",
		Plural:       "formers",
		Suffix:       "_former",
		AltSuffixes:  []string{"_form"},
		ErrAmbiguous: errAmbiguousTestName,
		IsMacro:      func(tok string) bool { return tok == "validates" },
	}
}

func noSuffixKind() reader.Kind {
	return reader.Kind{
		Noun:         "service",
		Plural:       "services",
		Suffix:       "",
		ErrAmbiguous: errAmbiguousTestName,
		IsMacro:      nil,
	}
}

// writeFile creates path (and its parent dirs) under root with placeholder
// content -- Resolve/ListNames only care that the file exists.
func writeFile(t *testing.T, root, rel string) string {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("setup MkdirAll: %v", err)
	}
	if err := os.WriteFile(full, []byte("class Placeholder\nend\n"), 0o644); err != nil {
		t.Fatalf("setup WriteFile: %v", err)
	}
	return full
}

func TestResolve_SuffixPreferredOverBareName(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "app/jobs/sync_user_job.rb")

	k := suffixedKind()
	path, err := k.Resolve(root, "app/jobs", "sync_user")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasSuffix(filepath.ToSlash(path), "app/jobs/sync_user_job.rb") {
		t.Errorf("path = %s, want app/jobs/sync_user_job.rb suffix", path)
	}
}

func TestResolve_FallsBackToBareNameWhenNoSuffixedFileExists(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "app/jobs/legacy_task.rb")

	k := suffixedKind()
	path, err := k.Resolve(root, "app/jobs", "legacy_task")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasSuffix(filepath.ToSlash(path), "app/jobs/legacy_task.rb") {
		t.Errorf("path = %s, want app/jobs/legacy_task.rb suffix", path)
	}
}

func TestResolve_EmptySuffixMatchesNameExactly(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "app/services/user_export_service.rb")

	k := noSuffixKind()
	path, err := k.Resolve(root, "app/services", "user_export_service")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasSuffix(filepath.ToSlash(path), "app/services/user_export_service.rb") {
		t.Errorf("path = %s, want app/services/user_export_service.rb suffix", path)
	}

	// Suffix-less Kind never appends anything -- "user_export" alone does
	// not resolve to "user_export_service.rb".
	if _, err := k.Resolve(root, "app/services", "user_export"); err == nil {
		t.Error("expected error resolving unsuffixed short name against a no-suffix Kind")
	}
}

func TestResolve_BasenameAnywhereInTree(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "app/jobs/admin/export_job.rb")

	k := suffixedKind()
	path, err := k.Resolve(root, "app/jobs", "export")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasSuffix(filepath.ToSlash(path), "app/jobs/admin/export_job.rb") {
		t.Errorf("path = %s, want app/jobs/admin/export_job.rb suffix", path)
	}
}

func TestResolve_AltSuffixTriedAfterPrimarySuffix(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "app/formers/session_form.rb")

	k := altSuffixKind()
	path, err := k.Resolve(root, "app/formers", "session")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasSuffix(filepath.ToSlash(path), "app/formers/session_form.rb") {
		t.Errorf("path = %s, want app/formers/session_form.rb suffix", path)
	}
}

func TestResolve_PrimarySuffixPreferredOverAltSuffix(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "app/formers/user_former.rb")
	writeFile(t, root, "app/formers/user_form.rb")

	k := altSuffixKind()
	path, err := k.Resolve(root, "app/formers", "user")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasSuffix(filepath.ToSlash(path), "app/formers/user_former.rb") {
		t.Errorf("path = %s, want app/formers/user_former.rb (Suffix candidate tried before AltSuffixes)", path)
	}
}

func TestResolve_AmbiguousWrapsCallerSentinel(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "app/jobs/admin/export_job.rb")
	writeFile(t, root, "app/jobs/billing/export_job.rb")

	k := suffixedKind()
	_, err := k.Resolve(root, "app/jobs", "export")
	if err == nil {
		t.Fatal("expected ambiguity error")
	}
	if !errors.Is(err, errAmbiguousTestName) {
		t.Errorf("error %v does not wrap the caller's sentinel", err)
	}
}

func TestResolve_RbInputAbsolutePath(t *testing.T) {
	root := t.TempDir()
	f := writeFile(t, root, "app/jobs/sync_user_job.rb")

	k := suffixedKind()
	path, err := k.Resolve(root, "app/jobs", f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if path != f {
		t.Errorf("path = %s, want %s", path, f)
	}
}

func TestResolve_RbInputConfigPrefixedRelative(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "app/jobs/sync_user_job.rb")

	k := suffixedKind()
	// Input already carries the configured dirPath prefix -- Resolve strips
	// it before joining against the resolved dir.
	path, err := k.Resolve(root, "app/jobs", "app/jobs/sync_user_job.rb")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasSuffix(filepath.ToSlash(path), "app/jobs/sync_user_job.rb") {
		t.Errorf("path = %s, want app/jobs/sync_user_job.rb suffix", path)
	}
}

func TestResolve_RbInputRootRelative(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "app/jobs/sync_user_job.rb")

	k := suffixedKind()
	// Input relative to railsRoot but without the dirPath prefix.
	path, err := k.Resolve(root, "app/jobs", filepath.Join("app", "jobs", "sync_user_job.rb"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasSuffix(filepath.ToSlash(path), "app/jobs/sync_user_job.rb") {
		t.Errorf("path = %s, want app/jobs/sync_user_job.rb suffix", path)
	}
}

func TestResolve_RbInputOutsideDirIsRejected(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "app/jobs/sync_user_job.rb")
	outside := t.TempDir()
	f := writeFile(t, outside, "elsewhere.rb")

	k := suffixedKind()
	_, err := k.Resolve(root, "app/jobs", f)
	if err == nil {
		t.Fatal("expected error for a .rb file outside the domain directory")
	}
}

func TestListNames_StripsSuffix(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "app/jobs/sync_user_job.rb")
	writeFile(t, root, "app/jobs/admin/export_job.rb")
	writeFile(t, root, "app/jobs/legacy_task.rb")

	k := suffixedKind()
	names, err := k.ListNames(root, "app/jobs")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"admin/export", "legacy_task", "sync_user"}
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("got %v, want %v", names, want)
	}
}

func TestListNames_StripsAltSuffixWhenPrimaryDoesNotMatch(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "app/formers/user_former.rb")
	writeFile(t, root, "app/formers/session_form.rb")
	writeFile(t, root, "app/formers/concerns/validatable.rb")

	k := altSuffixKind()
	names, err := k.ListNames(root, "app/formers")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"concerns/validatable", "session", "user"}
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("got %v, want %v", names, want)
	}
}

func TestListNames_DedupesWhenBothSuffixConventionsReduceToSameName(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "app/formers/user_former.rb")
	writeFile(t, root, "app/formers/user_form.rb")

	k := altSuffixKind()
	names, err := k.ListNames(root, "app/formers")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"user"}
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("got %v, want %v (both files reduce to the same short name once)", names, want)
	}
}

func TestListNames_EmptySuffixLeavesNamesVerbatim(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "app/services/user_export_service.rb")

	k := noSuffixKind()
	names, err := k.ListNames(root, "app/services")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"user_export_service"}
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("got %v, want %v", names, want)
	}
}

func TestListNames_MissingDirReturnsNilNil(t *testing.T) {
	root := t.TempDir()
	k := suffixedKind()
	names, err := k.ListNames(root, "app/jobs")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if names != nil {
		t.Fatalf("expected nil names, got %v", names)
	}
}

func TestListNames_SkipDirsExcludesGivenDirectory(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "app/controllers/users_controller.rb")
	writeFile(t, root, "app/controllers/concerns/searchable.rb")

	k := reader.Kind{
		Noun:         "controller",
		Plural:       "controllers",
		Suffix:       "_controller",
		ErrAmbiguous: errAmbiguousTestName,
	}
	concernsDir := filepath.Join(root, "app", "controllers", "concerns")
	names, err := k.ListNames(root, "app/controllers", concernsDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"users"}
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("got %v, want %v (concerns/ should be excluded)", names, want)
	}
}

func TestStyleEntry(t *testing.T) {
	st := term.NewStyler(term.ModeAlways, nil)

	t.Run("macro token gets accented", func(t *testing.T) {
		k := suffixedKind()
		got := k.StyleEntry("  retry_on StandardError", st)
		want := "  " + st.Cyan("retry_on") + " StandardError"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("non-macro token passes through unchanged", func(t *testing.T) {
		k := suffixedKind()
		entry := "  perform"
		if got := k.StyleEntry(entry, st); got != entry {
			t.Errorf("got %q, want unchanged %q", got, entry)
		}
	})

	t.Run("nil IsMacro passes every entry through unchanged", func(t *testing.T) {
		k := noSuffixKind()
		entry := "  call"
		if got := k.StyleEntry(entry, st); got != entry {
			t.Errorf("got %q, want unchanged %q", got, entry)
		}
	})

	t.Run("unindented entry passes through unchanged", func(t *testing.T) {
		k := suffixedKind()
		entry := "retry_on StandardError"
		if got := k.StyleEntry(entry, st); got != entry {
			t.Errorf("got %q, want unchanged %q", got, entry)
		}
	})

	t.Run("deeper-indented nested entry keeps macro accent", func(t *testing.T) {
		// A former's with_options children render four spaces deep rather
		// than the usual two -- StyleEntry must not assume a fixed indent
		// width to find the token.
		k := altSuffixKind()
		got := k.StyleEntry("    validates :city", st)
		want := "    " + st.Cyan("validates") + " :city"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})
}
