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

	got, err := skillDir(skillTargetClaude, true, false)
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

func TestSkillDir_GlobalUsesPlatformHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		target string
		want   string
	}{
		{skillTargetClaude, filepath.Join(home, ".claude", "skills", "rails-kit")},
		{skillTargetCodex, filepath.Join(home, ".agents", "skills", "rails-kit")},
	}
	for _, test := range tests {
		t.Run(test.target, func(t *testing.T) {
			got, err := skillDir(test.target, false, false)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != test.want {
				t.Fatalf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestSkillDir_RootFlagSkipsRailsValidation(t *testing.T) {
	dir := t.TempDir() // not a Rails root — no config/application.rb

	prevRootFlag := rootFlag
	rootFlag = dir
	t.Cleanup(func() { rootFlag = prevRootFlag })

	got, err := skillDir(skillTargetCodex, true, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join(dir, ".agents", "skills", "rails-kit")
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestSkillDir_ValidatedLocalRequiresRailsRoot(t *testing.T) {
	dir := t.TempDir()

	prevRootFlag := rootFlag
	rootFlag = dir
	t.Cleanup(func() { rootFlag = prevRootFlag })

	_, err := skillDir(skillTargetClaude, true, true)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "not a Rails root") && !strings.Contains(err.Error(), "could not find Rails root") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSkillDir_ValidatedLocalUsesExplicitRailsRoot(t *testing.T) {
	root := t.TempDir()
	mustWriteSkillFile(t, filepath.Join(root, "config", "application.rb"))

	prevRootFlag := rootFlag
	rootFlag = root
	t.Cleanup(func() { rootFlag = prevRootFlag })

	got, err := skillDir(skillTargetCodex, true, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join(root, ".agents", "skills", "rails-kit")
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

func TestSkillTargets(t *testing.T) {
	tests := []struct {
		value string
		want  []string
	}{
		{skillTargetClaude, []string{skillTargetClaude}},
		{skillTargetCodex, []string{skillTargetCodex}},
		{skillTargetAll, []string{skillTargetClaude, skillTargetCodex}},
	}
	for _, test := range tests {
		t.Run(test.value, func(t *testing.T) {
			got, err := skillTargets(test.value)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Join(got, ",") != strings.Join(test.want, ",") {
				t.Fatalf("got %#v, want %#v", got, test.want)
			}
		})
	}
	if _, err := skillTargets("unknown"); err == nil {
		t.Fatal("expected invalid target error")
	}
}

func TestValidateSkillScope(t *testing.T) {
	if err := validateSkillScope(false, false); err != nil {
		t.Fatal(err)
	}
	if err := validateSkillScope(false, true); err != nil {
		t.Fatal(err)
	}
	if err := validateSkillScope(true, false); err != nil {
		t.Fatal(err)
	}
	if err := validateSkillScope(true, true); err == nil {
		t.Fatal("expected conflicting scope error")
	}
}

func TestInstallSkill_ClaudeAndCodexPayloads(t *testing.T) {
	claudeDir := filepath.Join(t.TempDir(), "claude")
	if err := installSkill(skillTargetClaude, claudeDir); err != nil {
		t.Fatal(err)
	}
	claudeContent, err := os.ReadFile(filepath.Join(claudeDir, "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(claudeContent), "allowed-tools:") || !strings.Contains(string(claudeContent), "model: haiku") {
		t.Fatal("Claude skill lost Claude-specific frontmatter")
	}

	codexDir := filepath.Join(t.TempDir(), "codex")
	if err := installSkill(skillTargetCodex, codexDir); err != nil {
		t.Fatal(err)
	}
	codexContent, err := os.ReadFile(filepath.Join(codexDir, "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(codexContent), "allowed-tools:") || strings.Contains(string(codexContent), "model: haiku") {
		t.Fatal("Codex skill contains Claude-specific frontmatter")
	}
	metadata, err := os.ReadFile(filepath.Join(codexDir, "agents", "openai.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(metadata), "display_name: \"rails-kit\"") ||
		!strings.Contains(string(metadata), "$rails-kit") {
		t.Fatalf("unexpected Codex metadata: %s", metadata)
	}
}

func TestUninstallSkill_RemovesOnlyTargetDirectory(t *testing.T) {
	parent := t.TempDir()
	target := filepath.Join(parent, "rails-kit")
	other := filepath.Join(parent, "other")
	mustWriteSkillFile(t, filepath.Join(target, "SKILL.md"))
	mustWriteSkillFile(t, filepath.Join(other, "SKILL.md"))

	if err := uninstallSkill(target); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("target still exists: %v", err)
	}
	if _, err := os.Stat(other); err != nil {
		t.Fatalf("unrelated directory was affected: %v", err)
	}
	if err := uninstallSkill(target); err != nil {
		t.Fatalf("missing installation should be non-fatal: %v", err)
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
