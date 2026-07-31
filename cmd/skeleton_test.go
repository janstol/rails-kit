package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/janstol/rails-kit/internal/config"
	"github.com/janstol/rails-kit/internal/prism"
)

func TestSkeletonCommandShowsServiceSkeleton(t *testing.T) {
	root := t.TempDir()
	mustWriteCmdFile(t, filepath.Join(root, "config", "application.rb"), "")
	mustWriteCmdFile(t, filepath.Join(root, "app", "services", "user_export_service.rb"), `class UserExportService
  DEFAULT_LIMIT = 100
  include Searchable

  def call(format: :csv)
    true
  end
end
`)

	out, errOut, err := runCmdForTest(t, skeletonCmd, root, []string{"app/services/user_export_service.rb"})
	if err != nil {
		t.Fatalf("unexpected error: %v\nstderr:%s", err, errOut)
	}
	for _, want := range []string{"app/services/user_export_service.rb", "Class UserExportService", "DEFAULT_LIMIT = 100", "include Searchable", "public def call(format: :csv)"} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
}

func TestSkeletonCommandResolvesModelNameAsJSON(t *testing.T) {
	root := t.TempDir()
	mustWriteCmdFile(t, filepath.Join(root, "config", "application.rb"), "")
	mustWriteCmdFile(t, filepath.Join(root, "app", "models", "user.rb"), "class User < ApplicationRecord\n  has_many :posts\nend\n")

	out, errOut, err := runCmdForTestJSON(t, skeletonCmd, root, []string{"user"})
	if err != nil {
		t.Fatalf("unexpected error: %v\nstderr:%s", err, errOut)
	}
	var data struct {
		Files []prism.File `json:"files"`
	}
	unwrapJSONEnvelope(t, out, "skeleton", &data)
	if len(data.Files) != 1 {
		t.Fatalf("unexpected files: %#v", data.Files)
	}
	payload := data.Files[0]
	if payload.RelPath != "app/models/user.rb" {
		t.Fatalf("rel_path = %q", payload.RelPath)
	}
	if len(payload.Classes) != 1 || payload.Classes[0].Name != "User" || payload.Classes[0].Parent != "ApplicationRecord" {
		t.Fatalf("unexpected classes: %#v", payload.Classes)
	}
	if len(payload.Classes[0].Calls) != 1 || payload.Classes[0].Calls[0].Source != "has_many :posts" {
		t.Fatalf("unexpected calls: %#v", payload.Classes[0].Calls)
	}
}

func TestSkeletonCommandRejectsNonRubyFile(t *testing.T) {
	root := t.TempDir()
	mustWriteCmdFile(t, filepath.Join(root, "config", "application.rb"), "")
	mustWriteCmdFile(t, filepath.Join(root, "README.md"), "# Test\n")

	_, _, err := runCmdForTest(t, skeletonCmd, root, []string{"README.md"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "ending in .rb") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSkeletonCommandRejectsOutsideRoot(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	mustWriteCmdFile(t, filepath.Join(root, "config", "application.rb"), "")
	mustWriteCmdFile(t, filepath.Join(outside, "service.rb"), "class Service\nend\n")

	_, _, err := runCmdForTest(t, skeletonCmd, root, []string{filepath.Join(outside, "service.rb")})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "outside Rails root") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolveSkeletonPathsExpandsSortsAndDeduplicates(t *testing.T) {
	root := t.TempDir()
	mustWriteCmdFile(t, filepath.Join(root, "config", "application.rb"), "")
	mustWriteCmdFile(t, filepath.Join(root, "app", "models", "user.rb"), "class User\nend\n")
	mustWriteCmdFile(t, filepath.Join(root, "app", "services", "zeta.rb"), "class Zeta\nend\n")
	mustWriteCmdFile(t, filepath.Join(root, "app", "services", "alpha.rb"), "class Alpha\nend\n")
	if err := os.Symlink(
		filepath.Join(root, "app", "services", "alpha.rb"),
		filepath.Join(root, "app", "services", "alpha_alias.rb"),
	); err != nil {
		t.Fatal(err)
	}

	cfg := config.Defaults()
	inputs, err := resolveSkeletonPaths(root, cfg, []string{
		"user",
		"app/services/*.rb",
		filepath.Join(root, "app", "services", "*.rb"),
	})
	if err != nil {
		t.Fatal(err)
	}
	got := make([]string, len(inputs))
	for i, input := range inputs {
		got[i] = input.relPath
	}
	want := []string{
		"app/models/user.rb",
		"app/services/alpha.rb",
		"app/services/zeta.rb",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("resolved paths:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestResolveSkeletonPathsRejectsInvalidInputs(t *testing.T) {
	root := t.TempDir()
	mustWriteCmdFile(t, filepath.Join(root, "config", "application.rb"), "")
	mustWriteCmdFile(t, filepath.Join(root, "app", "services", "valid.rb"), "class Valid\nend\n")
	if err := os.MkdirAll(filepath.Join(root, "app", "services", "directory.rb"), 0o755); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	mustWriteCmdFile(t, filepath.Join(outside, "outside.rb"), "class Outside\nend\n")
	if err := os.Symlink(
		filepath.Join(outside, "outside.rb"),
		filepath.Join(root, "app", "services", "outside.rb"),
	); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "unmatched glob", input: "app/jobs/*.rb", want: "matched no files"},
		{name: "invalid glob", input: "app/services/[.rb", want: "invalid skeleton glob"},
		{name: "empty directory", input: "app/services/directory.rb", want: "contains no Ruby files"},
		{name: "escaping symlink", input: "app/services/outside.rb", want: "outside Rails root"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := resolveSkeletonPaths(root, config.Defaults(), []string{tt.input})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want containing %q", err, tt.want)
			}
		})
	}
}

func TestResolveSkeletonDirectoryRecursesInLexicalOrder(t *testing.T) {
	root := t.TempDir()
	mustWriteCmdFile(t, filepath.Join(root, "config", "application.rb"), "")
	mustWriteCmdFile(t, filepath.Join(root, "app", "services", "zeta.rb"), "")
	mustWriteCmdFile(t, filepath.Join(root, "app", "services", "admin", "beta.rb"), "")
	mustWriteCmdFile(t, filepath.Join(root, "app", "services", "admin", "alpha.rb"), "")
	mustWriteCmdFile(t, filepath.Join(root, "app", "services", "notes.txt"), "")

	inputs, err := resolveSkeletonPaths(root, config.Defaults(), []string{
		"app/services",
		"app/services/admin/alpha.rb",
	})
	if err != nil {
		t.Fatal(err)
	}
	got := make([]string, len(inputs))
	for i, input := range inputs {
		got[i] = input.relPath
	}
	want := []string{
		"app/services/admin/alpha.rb",
		"app/services/admin/beta.rb",
		"app/services/zeta.rb",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("resolved paths:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestResolveSkeletonDirectoryAppliesExcludes(t *testing.T) {
	root := t.TempDir()
	mustWriteCmdFile(t, filepath.Join(root, "config", "application.rb"), "")
	mustWriteCmdFile(t, filepath.Join(root, "app", "services", "keep.rb"), "")
	mustWriteCmdFile(t, filepath.Join(root, "app", "services", "old_generated.rb"), "")
	mustWriteCmdFile(t, filepath.Join(root, "app", "services", "generated", "one.rb"), "")
	mustWriteCmdFile(t, filepath.Join(root, "app", "services", "generated", "nested", "two.rb"), "")

	inputs, err := resolveSkeletonPathsWithExcludes(
		root,
		config.Defaults(),
		[]string{"app/services"},
		[]string{"app/services/generated/**", "**/*_generated.rb"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(inputs) != 1 || inputs[0].relPath != "app/services/keep.rb" {
		t.Fatalf("unexpected files after exclusions: %#v", inputs)
	}

	explicit, err := resolveSkeletonPathsWithExcludes(
		root,
		config.Defaults(),
		[]string{"app/services/old_generated.rb", "app/services/generated/*.rb"},
		[]string{"**"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(explicit) != 2 {
		t.Fatalf("directory exclusions affected explicit inputs: %#v", explicit)
	}
}

func TestSkeletonExcludeMatching(t *testing.T) {
	tests := []struct {
		pattern string
		value   string
		want    bool
	}{
		{pattern: "app/services/generated/**", value: "app/services/generated", want: true},
		{pattern: "app/services/generated/**", value: "app/services/generated/nested/file.rb", want: true},
		{pattern: "**/generated/**", value: "generated/file.rb", want: true},
		{pattern: "**/generated/**", value: "app/services/generated/nested/file.rb", want: true},
		{pattern: "**/*_generated.rb", value: "app/services/old_generated.rb", want: true},
		{pattern: "app/?obs/[a-z]*.rb", value: "app/jobs/sync.rb", want: true},
		{pattern: "app/services/*.rb", value: "app/services/nested/file.rb", want: false},
	}
	for _, tt := range tests {
		if got := matchSkeletonPath(tt.pattern, tt.value); got != tt.want {
			t.Errorf("matchSkeletonPath(%q, %q) = %v, want %v", tt.pattern, tt.value, got, tt.want)
		}
	}
}

func TestResolveSkeletonPathsRejectsInvalidExcludes(t *testing.T) {
	root := t.TempDir()
	mustWriteCmdFile(t, filepath.Join(root, "config", "application.rb"), "")
	mustWriteCmdFile(t, filepath.Join(root, "app", "services", "keep.rb"), "")
	tests := []string{"", "/app/services/**", "../outside/**", "app/services/[bad"}
	for _, pattern := range tests {
		t.Run(pattern, func(t *testing.T) {
			_, err := resolveSkeletonPathsWithExcludes(
				root,
				config.Defaults(),
				[]string{"app/services"},
				[]string{pattern},
			)
			if err == nil {
				t.Fatalf("expected exclude %q to fail", pattern)
			}
		})
	}
}

func TestResolveSkeletonDirectorySymlinks(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	mustWriteCmdFile(t, filepath.Join(root, "config", "application.rb"), "")
	mustWriteCmdFile(t, filepath.Join(root, "app", "services", "keep.rb"), "")
	mustWriteCmdFile(t, filepath.Join(root, "app", "shared", "hidden.rb"), "")
	mustWriteCmdFile(t, filepath.Join(outside, "outside.rb"), "")
	if err := os.Symlink(
		filepath.Join(root, "app", "shared"),
		filepath.Join(root, "app", "services", "linked_directory"),
	); err != nil {
		t.Fatal(err)
	}

	inputs, err := resolveSkeletonPaths(root, config.Defaults(), []string{"app/services"})
	if err != nil {
		t.Fatal(err)
	}
	if len(inputs) != 1 || inputs[0].relPath != "app/services/keep.rb" {
		t.Fatalf("directory symlink was followed: %#v", inputs)
	}

	_, err = resolveSkeletonPaths(root, config.Defaults(), []string{"app/services/linked_directory"})
	if err == nil || !strings.Contains(err.Error(), "does not follow directory symlinks") {
		t.Fatalf("unexpected explicit directory symlink error: %v", err)
	}
	_, err = resolveSkeletonPaths(root, config.Defaults(), []string{outside})
	if err == nil || !strings.Contains(err.Error(), "outside Rails root") {
		t.Fatalf("unexpected outside directory error: %v", err)
	}
}

func TestResolveSkeletonDirectoryFileLimit(t *testing.T) {
	root := t.TempDir()
	mustWriteCmdFile(t, filepath.Join(root, "config", "application.rb"), "")
	for i := 0; i <= maxSkeletonFiles; i++ {
		name := fmt.Sprintf("%03d.rb", i)
		mustWriteCmdFile(t, filepath.Join(root, "bulk", name), "")
	}

	_, err := resolveSkeletonPaths(root, config.Defaults(), []string{"bulk"})
	if err == nil || !strings.Contains(err.Error(), "more than 500 unique files") {
		t.Fatalf("unexpected limit error: %v", err)
	}
	inputs, err := resolveSkeletonPathsWithExcludes(
		root,
		config.Defaults(),
		[]string{"bulk"},
		[]string{"bulk/500.rb"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(inputs) != maxSkeletonFiles {
		t.Fatalf("resolved %d files, want %d", len(inputs), maxSkeletonFiles)
	}
}

func TestSkeletonCommandParsesMultipleFilesInADirectory(t *testing.T) {
	root := t.TempDir()
	mustWriteCmdFile(t, filepath.Join(root, "config", "application.rb"), "")
	mustWriteCmdFile(t, filepath.Join(root, "app", "services", "alpha.rb"), "class Alpha\nend\n")
	mustWriteCmdFile(t, filepath.Join(root, "app", "services", "zeta.rb"), "class Zeta\nend\n")

	out, errOut, err := runCmdForTestJSON(t, skeletonCmd, root, []string{"app/services"})
	if err != nil {
		t.Fatalf("unexpected error: %v\nstderr:%s", err, errOut)
	}
	var data struct {
		Files []prism.File `json:"files"`
	}
	unwrapJSONEnvelope(t, out, "skeleton", &data)
	files := data.Files
	if len(files) != 2 ||
		files[0].RelPath != "app/services/alpha.rb" || files[0].Classes[0].Name != "Alpha" ||
		files[1].RelPath != "app/services/zeta.rb" || files[1].Classes[0].Name != "Zeta" {
		t.Fatalf("unexpected files: %#v", files)
	}
}

func TestSkeletonCommandFormatsMultipleTextSections(t *testing.T) {
	root := t.TempDir()
	mustWriteCmdFile(t, filepath.Join(root, "config", "application.rb"), "")
	mustWriteCmdFile(t, filepath.Join(root, "app", "services", "alpha.rb"), "class Alpha\nend\n")
	mustWriteCmdFile(t, filepath.Join(root, "app", "services", "zeta.rb"), "class Zeta\nend\n")

	out, errOut, err := runCmdForTest(t, skeletonCmd, root, []string{"app/services"})
	if err != nil {
		t.Fatalf("unexpected error: %v\nstderr:%s", err, errOut)
	}
	for _, want := range []string{
		"app/services/alpha.rb",
		"Class Alpha",
		"app/services/zeta.rb",
		"Class Zeta",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("text output missing %q:\n%s", want, out)
		}
	}
}

func TestSkeletonCommandSingleMatchGlobStaysAnArrayJSON(t *testing.T) {
	root := t.TempDir()
	mustWriteCmdFile(t, filepath.Join(root, "config", "application.rb"), "")
	mustWriteCmdFile(t, filepath.Join(root, "app", "jobs", "sync_job.rb"), "class SyncJob\nend\n")
	mustWriteCmdFile(t, filepath.Join(root, "app", "jobs", "ignored_job.rb"), "class IgnoredJob\nend\n")
	prevExcludes := skeletonExcludes
	skeletonExcludes = []string{"app/jobs/ignored*"}
	t.Cleanup(func() { skeletonExcludes = prevExcludes })

	out, errOut, err := runCmdForTestJSON(t, skeletonCmd, root, []string{"app/jobs"})
	if err != nil {
		t.Fatalf("unexpected error: %v\nstderr:%s", err, errOut)
	}
	var data struct {
		Files []prism.File `json:"files"`
	}
	unwrapJSONEnvelope(t, out, "skeleton", &data)
	if len(data.Files) != 1 {
		t.Fatalf("single-match result must still be a 1-element array, got: %#v", data.Files)
	}
	file := data.Files[0]
	if file.RelPath != "app/jobs/sync_job.rb" || len(file.Classes) != 1 || file.Classes[0].Name != "SyncJob" {
		t.Fatalf("unexpected file: %#v", file)
	}
}

func TestSkeletonCommandInvalidBatchFailsBeforeParsing(t *testing.T) {
	root := t.TempDir()
	mustWriteCmdFile(t, filepath.Join(root, "config", "application.rb"), "")
	mustWriteCmdFile(t, filepath.Join(root, "app", "services", "valid.rb"), "class Valid\nend\n")

	out, _, err := runCmdForTest(t, skeletonCmd, root, []string{"app/services/valid.rb", "app/jobs/*.rb"})
	if err == nil || !strings.Contains(err.Error(), "matched no files") {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "" {
		t.Fatalf("invalid batch produced stdout: %q", out)
	}
}

func TestSkeletonTimeout(t *testing.T) {
	tests := []struct {
		count int
		want  time.Duration
	}{
		{count: 0, want: 10 * time.Second},
		{count: 1, want: 10 * time.Second},
		{count: 11, want: 12 * time.Second},
		{count: 501, want: 110 * time.Second},
		{count: 1000, want: 120 * time.Second},
	}
	for _, tt := range tests {
		if got := skeletonTimeout(tt.count); got != tt.want {
			t.Errorf("skeletonTimeout(%d) = %v, want %v", tt.count, got, tt.want)
		}
	}
}

func TestSkeletonCommandRequiresAtLeastOneInput(t *testing.T) {
	if err := skeletonCmd.Args(skeletonCmd, nil); err == nil {
		t.Fatal("expected zero arguments to be rejected")
	}
}
