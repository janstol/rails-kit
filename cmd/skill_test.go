package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSkillDir_LocalUsesRailsRoot(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "app", "models", "nested")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	mustWriteSkillFile(t, filepath.Join(root, "config", "application.rb"))

	prevRootFlag := rootFlag
	prevWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	rootFlag = ""
	if err := os.Chdir(nested); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		rootFlag = prevRootFlag
		_ = os.Chdir(prevWD)
	})

	got, err := skillDir(false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	gotRoot, err := filepath.EvalSymlinks(filepath.Dir(filepath.Dir(filepath.Dir(got))))
	if err != nil {
		t.Fatalf("eval symlinks for %q: %v", got, err)
	}
	wantRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatalf("eval symlinks for %q: %v", root, err)
	}
	got = filepath.Join(gotRoot, ".claude", "skills", "rails-kit")
	want := filepath.Join(wantRoot, ".claude", "skills", "rails-kit")
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestSkillDir_GlobalUsesHome(t *testing.T) {
	got, err := skillDir(true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(home, ".claude", "skills", "rails-kit")
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestSkillDir_RootFlagSkipsRailsValidation(t *testing.T) {
	dir := t.TempDir() // not a Rails root — no config/application.rb

	prevRootFlag := rootFlag
	rootFlag = dir
	t.Cleanup(func() { rootFlag = prevRootFlag })

	got, err := skillDir(false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join(dir, ".claude", "skills", "rails-kit")
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestValidatedLocalSkillDir_RequiresRailsRoot(t *testing.T) {
	dir := t.TempDir()

	prevRootFlag := rootFlag
	rootFlag = dir
	t.Cleanup(func() { rootFlag = prevRootFlag })

	_, err := validatedLocalSkillDir()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "not a Rails root") && !strings.Contains(err.Error(), "could not find Rails root") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidatedLocalSkillDir_UsesExplicitRailsRoot(t *testing.T) {
	root := t.TempDir()
	mustWriteSkillFile(t, filepath.Join(root, "config", "application.rb"))

	prevRootFlag := rootFlag
	rootFlag = root
	t.Cleanup(func() { rootFlag = prevRootFlag })

	got, err := validatedLocalSkillDir()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join(root, ".claude", "skills", "rails-kit")
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestSkillInstallRejectsExtraArgs(t *testing.T) {
	if err := skillInstallCmd.Args(skillInstallCmd, []string{"extra"}); err == nil {
		t.Fatal("expected argument validation error")
	}
}

func TestSkillUninstallRejectsExtraArgs(t *testing.T) {
	if err := skillUninstallCmd.Args(skillUninstallCmd, []string{"extra"}); err == nil {
		t.Fatal("expected argument validation error")
	}
}

func mustWriteSkillFile(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(""), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
