package cmd

import (
	"encoding/json"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/janstol/rails-kit/internal/prism"
)

func TestSkeletonCommandShowsServiceSkeleton(t *testing.T) {
	requireCmdPrism(t)
	prevRunner := prismRunner
	prismRunner = prism.Runner{}
	t.Cleanup(func() { prismRunner = prevRunner })

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
	requireCmdPrism(t)
	prevRunner := prismRunner
	prismRunner = prism.Runner{}
	t.Cleanup(func() { prismRunner = prevRunner })

	root := t.TempDir()
	mustWriteCmdFile(t, filepath.Join(root, "config", "application.rb"), "")
	mustWriteCmdFile(t, filepath.Join(root, "app", "models", "user.rb"), "class User < ApplicationRecord\n  has_many :posts\nend\n")

	out, errOut, err := runCmdForTestJSON(t, skeletonCmd, root, []string{"user"})
	if err != nil {
		t.Fatalf("unexpected error: %v\nstderr:%s", err, errOut)
	}
	var payload prism.File
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("unmarshal json: %v\noutput:%s", err, out)
	}
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

func TestSkeletonCommandReportsMissingPrism(t *testing.T) {
	prevRunner := prismRunner
	prismRunner = prism.Runner{Ruby: "rails-kit-ruby-that-does-not-exist"}
	t.Cleanup(func() { prismRunner = prevRunner })

	root := t.TempDir()
	mustWriteCmdFile(t, filepath.Join(root, "config", "application.rb"), "")
	mustWriteCmdFile(t, filepath.Join(root, "app", "services", "user_export_service.rb"), "class UserExportService\nend\n")

	_, _, err := runCmdForTest(t, skeletonCmd, root, []string{"app/services/user_export_service.rb"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "prism is not available") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func requireCmdPrism(t *testing.T) {
	t.Helper()
	if err := exec.Command("ruby", "-rprism", "-e", "exit").Run(); err != nil {
		t.Skipf("ruby with prism is not available: %v", err)
	}
}
