package gem

import (
	"os"
	"path/filepath"
	"testing"
)

const testdataLockfile = "../../testdata/Gemfile.lock"

func TestParseGEMSection(t *testing.T) {
	lf, err := Parse(testdataLockfile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	g := lf.Find("rails")
	if g == nil {
		t.Fatal("expected to find gem 'rails'")
	}
	if g.Version != "7.1.3" {
		t.Errorf("version = %q, want %q", g.Version, "7.1.3")
	}
	if g.Source != SourceRubygems {
		t.Errorf("source = %q, want %q", g.Source, SourceRubygems)
	}
	if g.SourceURL != "https://rubygems.org/" {
		t.Errorf("source_url = %q, want %q", g.SourceURL, "https://rubygems.org/")
	}
	if len(g.Dependencies) != 3 {
		t.Errorf("dependencies count = %d, want 3", len(g.Dependencies))
	}
}

func TestParseGITSection(t *testing.T) {
	lf, err := Parse(testdataLockfile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	g := lf.Find("my_gem")
	if g == nil {
		t.Fatal("expected to find gem 'my_gem'")
	}
	if g.Source != SourceGit {
		t.Errorf("source = %q, want %q", g.Source, SourceGit)
	}
	if g.SourceURL != "https://github.com/example/my_gem.git" {
		t.Errorf("source_url = %q, want %q", g.SourceURL, "https://github.com/example/my_gem.git")
	}
	if g.Revision != "abc123def456abc123def456abc123def456abc1" {
		t.Errorf("revision = %q, want abc123...", g.Revision)
	}
	if g.Branch != "main" {
		t.Errorf("branch = %q, want %q", g.Branch, "main")
	}
	if len(g.Dependencies) != 1 || g.Dependencies[0].Name != "activesupport" {
		t.Errorf("unexpected dependencies: %v", g.Dependencies)
	}
}

func TestParsePATHSection(t *testing.T) {
	lf, err := Parse(testdataLockfile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	g := lf.Find("local_gem")
	if g == nil {
		t.Fatal("expected to find gem 'local_gem'")
	}
	if g.Source != SourcePath {
		t.Errorf("source = %q, want %q", g.Source, SourcePath)
	}
	if g.SourceURL != "gems/local_gem" {
		t.Errorf("source_url = %q, want %q", g.SourceURL, "gems/local_gem")
	}
}

func TestParseDependencyConstraints(t *testing.T) {
	lf, err := Parse(testdataLockfile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	g := lf.Find("actioncable")
	if g == nil {
		t.Fatal("expected to find gem 'actioncable'")
	}
	depMap := make(map[string]string)
	for _, d := range g.Dependencies {
		depMap[d.Name] = d.Constraint
	}
	if depMap["actionpack"] != "= 7.1.3" {
		t.Errorf("actionpack constraint = %q, want %q", depMap["actionpack"], "= 7.1.3")
	}
	if depMap["nio4r"] != "~> 2.0" {
		t.Errorf("nio4r constraint = %q, want %q", depMap["nio4r"], "~> 2.0")
	}
}

func TestListSortedAlphabetically(t *testing.T) {
	lf, err := Parse(testdataLockfile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	gems := lf.List()
	if len(gems) == 0 {
		t.Fatal("expected non-empty gem list")
	}
	for i := 1; i < len(gems); i++ {
		if gems[i].Name < gems[i-1].Name {
			t.Errorf("list not sorted: %q comes after %q", gems[i].Name, gems[i-1].Name)
		}
	}
}

func TestFindCaseInsensitive(t *testing.T) {
	lf, err := Parse(testdataLockfile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if lf.Find("Rails") == nil {
		t.Error("expected Find('Rails') to match 'rails'")
	}
	if lf.Find("RAILS") == nil {
		t.Error("expected Find('RAILS') to match 'rails'")
	}
}

func TestFindMissing(t *testing.T) {
	lf, err := Parse(testdataLockfile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if lf.Find("nonexistent_gem_xyz") != nil {
		t.Error("expected nil for nonexistent gem")
	}
}

func TestParseLockfileMetadata(t *testing.T) {
	lf, err := Parse(testdataLockfile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if lf.BundlerVersion() != "2.5.6" {
		t.Errorf("bundler version = %q, want 2.5.6", lf.BundlerVersion())
	}
	wantPlatforms := []string{"arm64-darwin-23", "ruby", "x86_64-linux"}
	if got := lf.Platforms(); len(got) != len(wantPlatforms) {
		t.Fatalf("platforms = %v, want %v", got, wantPlatforms)
	} else {
		for i := range wantPlatforms {
			if got[i] != wantPlatforms[i] {
				t.Errorf("platforms = %v, want %v", got, wantPlatforms)
				break
			}
		}
	}
}

func TestParseRubyVersionMetadata(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Gemfile.lock")
	content := "RUBY VERSION\n   ruby 3.3.6p108\n\nBUNDLED WITH\n   2.6.2\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	lf, err := Parse(path)
	if err != nil {
		t.Fatal(err)
	}
	if lf.RubyVersion() != "3.3.6p108" {
		t.Errorf("ruby version = %q, want 3.3.6p108", lf.RubyVersion())
	}
}

func TestParseEmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Gemfile.lock")
	if err := os.WriteFile(path, []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}

	lf, err := Parse(path)
	if err != nil {
		t.Fatalf("unexpected error on empty file: %v", err)
	}
	if len(lf.List()) != 0 {
		t.Error("expected empty gem list for empty file")
	}
}

func TestParseFileNotFound(t *testing.T) {
	_, err := Parse("/nonexistent/path/Gemfile.lock")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestParseMultipleGEMBlocks(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Gemfile.lock")
	content := `GEM
  remote: https://rubygems.org/
  specs:
    rails (7.1.3)

GEM
  remote: https://gems.example.com/
  specs:
    private_gem (1.0.0)
      rails (>= 7.0)

PLATFORMS
  ruby

DEPENDENCIES
  rails
  private_gem

BUNDLED WITH
   2.5.6
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	lf, err := Parse(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	rails := lf.Find("rails")
	if rails == nil {
		t.Fatal("expected to find 'rails'")
	}
	if rails.SourceURL != "https://rubygems.org/" {
		t.Errorf("rails source_url = %q, want rubygems", rails.SourceURL)
	}

	pg := lf.Find("private_gem")
	if pg == nil {
		t.Fatal("expected to find 'private_gem'")
	}
	if pg.SourceURL != "https://gems.example.com/" {
		t.Errorf("private_gem source_url = %q, want example.com", pg.SourceURL)
	}
}

func TestParseGITWithTag(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Gemfile.lock")
	content := `GIT
  remote: https://github.com/example/tagged_gem.git
  revision: deadbeef
  tag: v2.0.0
  specs:
    tagged_gem (2.0.0)

PLATFORMS
  ruby

DEPENDENCIES
  tagged_gem!

BUNDLED WITH
   2.5.6
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	lf, err := Parse(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	g := lf.Find("tagged_gem")
	if g == nil {
		t.Fatal("expected to find 'tagged_gem'")
	}
	if g.Tag != "v2.0.0" {
		t.Errorf("tag = %q, want %q", g.Tag, "v2.0.0")
	}
	if g.Branch != "" {
		t.Errorf("branch should be empty, got %q", g.Branch)
	}
}
