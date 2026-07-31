package cmd

import (
	"path/filepath"
	"strings"
	"testing"
)

const testGemfileLock = `GIT
  remote: https://github.com/example/my_gem.git
  revision: abc123def456abc123def456abc123def456abc1
  branch: main
  specs:
    my_gem (0.1.0)
      activesupport (>= 6.0)

GEM
  remote: https://rubygems.org/
  specs:
    activesupport (7.1.3)
    rails (7.1.3)
      activesupport (= 7.1.3)

PLATFORMS
  ruby

DEPENDENCIES
  my_gem!
  rails (~> 7.1)

BUNDLED WITH
   2.5.6
`

func setupGemRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	mustWriteCmdFile(t, filepath.Join(root, "config", "application.rb"), "")
	mustWriteCmdFile(t, filepath.Join(root, "Gemfile.lock"), testGemfileLock)
	return root
}

func TestGemCommandListPlain(t *testing.T) {
	root := setupGemRoot(t)

	out, errOut, err := runCmdForTest(t, gemCmd, root, []string{})
	if err != nil {
		t.Fatalf("unexpected error: %v\nstderr: %s", err, errOut)
	}
	if !strings.Contains(out, "rails (7.1.3)") {
		t.Errorf("expected 'rails (7.1.3)' in output:\n%s", out)
	}
	if !strings.Contains(out, "my_gem (0.1.0)") {
		t.Errorf("expected 'my_gem (0.1.0)' in output:\n%s", out)
	}
}

func TestGemCommandListJSON(t *testing.T) {
	root := setupGemRoot(t)

	out, errOut, err := runCmdForTestJSON(t, gemCmd, root, []string{})
	if err != nil {
		t.Fatalf("unexpected error: %v\nstderr: %s", err, errOut)
	}
	var payload struct {
		Gems []struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"gems"`
	}
	unwrapJSONEnvelope(t, out, "gem", &payload)
	entries := payload.Gems
	if len(entries) == 0 {
		t.Fatal("expected non-empty gem list")
	}
	found := false
	for _, e := range entries {
		if e.Name == "rails" && e.Version == "7.1.3" {
			found = true
		}
	}
	if !found {
		t.Errorf("rails 7.1.3 not in JSON output: %s", out)
	}
}

func TestGemCommandDetailPlain(t *testing.T) {
	root := setupGemRoot(t)

	out, errOut, err := runCmdForTest(t, gemCmd, root, []string{"rails"})
	if err != nil {
		t.Fatalf("unexpected error: %v\nstderr: %s", err, errOut)
	}
	if !strings.Contains(out, "rails (7.1.3)") {
		t.Errorf("expected name/version in output:\n%s", out)
	}
	if !strings.Contains(out, "rubygems") {
		t.Errorf("expected source type in output:\n%s", out)
	}
	if !strings.Contains(out, "rubygems.org") {
		t.Errorf("expected source URL in output:\n%s", out)
	}
	if !strings.Contains(out, "activesupport") {
		t.Errorf("expected dependency in output:\n%s", out)
	}
}

func TestGemCommandDetailJSON(t *testing.T) {
	root := setupGemRoot(t)

	out, errOut, err := runCmdForTestJSON(t, gemCmd, root, []string{"my_gem"})
	if err != nil {
		t.Fatalf("unexpected error: %v\nstderr: %s", err, errOut)
	}
	var g struct {
		Name      string `json:"name"`
		Version   string `json:"version"`
		Source    string `json:"source"`
		SourceURL string `json:"source_url"`
		Revision  string `json:"revision"`
		Branch    string `json:"branch"`
	}
	unwrapJSONEnvelope(t, out, "gem", &g)
	if g.Name != "my_gem" {
		t.Errorf("name = %q, want my_gem", g.Name)
	}
	if g.Source != "git" {
		t.Errorf("source = %q, want git", g.Source)
	}
	if g.Branch != "main" {
		t.Errorf("branch = %q, want main", g.Branch)
	}
}

func TestGemCommandNotFound(t *testing.T) {
	root := setupGemRoot(t)

	_, _, err := runCmdForTest(t, gemCmd, root, []string{"nonexistent_xyz"})
	if err == nil {
		t.Fatal("expected error for missing gem")
	}
	if !strings.Contains(err.Error(), "nonexistent_xyz") {
		t.Errorf("error should mention gem name: %v", err)
	}
}

func TestGemCommandCustomLockPath(t *testing.T) {
	root := t.TempDir()
	external := t.TempDir()
	mustWriteCmdFile(t, filepath.Join(root, "config", "application.rb"), "")
	mustWriteCmdFile(t, filepath.Join(root, ".rails-kit.yml"), "gemfile_lock_path: "+filepath.Join(external, "Gemfile.lock")+"\n")
	mustWriteCmdFile(t, filepath.Join(external, "Gemfile.lock"), testGemfileLock)

	out, errOut, err := runCmdForTest(t, gemCmd, root, []string{})
	if err != nil {
		t.Fatalf("unexpected error: %v\nstderr: %s", err, errOut)
	}
	if !strings.Contains(out, "rails") {
		t.Errorf("expected rails in output:\n%s", out)
	}
}

func TestGemCommandFileNotFound(t *testing.T) {
	root := t.TempDir()
	mustWriteCmdFile(t, filepath.Join(root, "config", "application.rb"), "")
	// No Gemfile.lock created

	_, _, err := runCmdForTest(t, gemCmd, root, []string{})
	if err == nil {
		t.Fatal("expected error when Gemfile.lock is missing")
	}
}
