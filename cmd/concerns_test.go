package cmd

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

const searchableRB = `module Searchable
  extend ActiveSupport::Concern

  included do
    scope :search, ->(query) { where("name LIKE ?", "%#{query}%") }
  end

  class_methods do
    def search_all(query)
      search(query).to_a
    end
  end

  def search_highlight
    # highlights search terms
  end
end
`

const auditableRB = `module Auditable
  extend ActiveSupport::Concern

  included do
    has_many :audit_logs, as: :auditable
  end

  def audit_trail
    audit_logs.order(created_at: :desc)
  end
end
`

const authenticatableRB = `module Authenticatable
  extend ActiveSupport::Concern

  included do
    before_action :authenticate_user!
  end

  private

  def authenticate_user!
    redirect_to login_path unless current_user
  end
end
`

func setupConcernsRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	mustWriteCmdFile(t, filepath.Join(root, "config", "application.rb"), "")
	mustWriteCmdFile(t, filepath.Join(root, "app", "models", "concerns", "searchable.rb"), searchableRB)
	mustWriteCmdFile(t, filepath.Join(root, "app", "models", "concerns", "auditable.rb"), auditableRB)
	mustWriteCmdFile(t, filepath.Join(root, "app", "controllers", "concerns", "authenticatable.rb"), authenticatableRB)
	return root
}

func TestConcernsCommandListsAll(t *testing.T) {
	root := setupConcernsRoot(t)
	out, errOut, err := runCmdForTest(t, concernsCmd, root, []string{})
	if err != nil {
		t.Fatalf("unexpected error: %v\nstderr:%s", err, errOut)
	}
	if !strings.Contains(out, "Model concerns:") {
		t.Fatalf("expected 'Model concerns:' in output:\n%s", out)
	}
	if !strings.Contains(out, "Controller concerns:") {
		t.Fatalf("expected 'Controller concerns:' in output:\n%s", out)
	}
	if !strings.Contains(out, "searchable") {
		t.Fatalf("expected searchable in output:\n%s", out)
	}
	if !strings.Contains(out, "auditable") {
		t.Fatalf("expected auditable in output:\n%s", out)
	}
	if !strings.Contains(out, "authenticatable") {
		t.Fatalf("expected authenticatable in output:\n%s", out)
	}
}

func TestConcernsCommandListsAllJSON(t *testing.T) {
	root := setupConcernsRoot(t)
	out, errOut, err := runCmdForTestJSON(t, concernsCmd, root, []string{})
	if err != nil {
		t.Fatalf("unexpected error: %v\nstderr:%s", err, errOut)
	}

	var payload struct {
		ModelConcerns      []string `json:"model_concerns"`
		ControllerConcerns []string `json:"controller_concerns"`
	}
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("unmarshal json: %v\noutput:%s", err, out)
	}
	if len(payload.ModelConcerns) != 2 {
		t.Fatalf("model_concerns = %v, want 2 items", payload.ModelConcerns)
	}
	if len(payload.ControllerConcerns) != 1 || payload.ControllerConcerns[0] != "authenticatable" {
		t.Fatalf("controller_concerns = %v, want [authenticatable]", payload.ControllerConcerns)
	}
}

func TestConcernsCommandShowsDetail(t *testing.T) {
	root := setupConcernsRoot(t)
	out, errOut, err := runCmdForTest(t, concernsCmd, root, []string{"searchable"})
	if err != nil {
		t.Fatalf("unexpected error: %v\nstderr:%s", err, errOut)
	}
	if !strings.Contains(out, "Searchable") {
		t.Fatalf("expected module name in output:\n%s", out)
	}
	if !strings.Contains(out, "Type: model") {
		t.Fatalf("expected 'Type: model' in output:\n%s", out)
	}
	if !strings.Contains(out, "Included block: yes") {
		t.Fatalf("expected 'Included block: yes' in output:\n%s", out)
	}
	if !strings.Contains(out, "search_highlight") {
		t.Fatalf("expected method name in output:\n%s", out)
	}
	if !strings.Contains(out, "search_all") {
		t.Fatalf("expected class method in output:\n%s", out)
	}
}

func TestConcernsCommandShowsDetailJSON(t *testing.T) {
	root := setupConcernsRoot(t)
	out, errOut, err := runCmdForTestJSON(t, concernsCmd, root, []string{"auditable"})
	if err != nil {
		t.Fatalf("unexpected error: %v\nstderr:%s", err, errOut)
	}

	var payload struct {
		Name             string   `json:"name"`
		Type             string   `json:"type"`
		Methods          []string `json:"methods"`
		HasIncludedBlock bool     `json:"has_included_block"`
	}
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("unmarshal json: %v\noutput:%s", err, out)
	}
	if payload.Name != "Auditable" {
		t.Fatalf("name = %q, want Auditable", payload.Name)
	}
	if payload.Type != "model" {
		t.Fatalf("type = %q, want model", payload.Type)
	}
	if !payload.HasIncludedBlock {
		t.Fatal("has_included_block = false, want true")
	}
	found := false
	for _, m := range payload.Methods {
		if m == "audit_trail" {
			found = true
		}
	}
	if !found {
		t.Fatalf("methods should contain audit_trail, got %v", payload.Methods)
	}
}

func TestConcernsCommandNotFound(t *testing.T) {
	root := setupConcernsRoot(t)
	_, _, err := runCmdForTest(t, concernsCmd, root, []string{"nonexistent"})
	if err == nil {
		t.Fatal("expected error for nonexistent concern")
	}
}

func TestConcernsCommandEmptyDirs(t *testing.T) {
	root := t.TempDir()
	mustWriteCmdFile(t, filepath.Join(root, "config", "application.rb"), "")

	out, errOut, err := runCmdForTest(t, concernsCmd, root, []string{})
	if err != nil {
		t.Fatalf("unexpected error: %v\nstderr:%s", err, errOut)
	}
	if !strings.Contains(out, "No concerns found.") {
		t.Fatalf("expected 'No concerns found.' in output:\n%s", out)
	}
}

func TestConcernsCommandOnlyModelConcerns(t *testing.T) {
	root := t.TempDir()
	mustWriteCmdFile(t, filepath.Join(root, "config", "application.rb"), "")
	mustWriteCmdFile(t, filepath.Join(root, "app", "models", "concerns", "searchable.rb"), searchableRB)

	out, errOut, err := runCmdForTest(t, concernsCmd, root, []string{})
	if err != nil {
		t.Fatalf("unexpected error: %v\nstderr:%s", err, errOut)
	}
	if !strings.Contains(out, "Model concerns:") {
		t.Fatalf("expected 'Model concerns:' in output:\n%s", out)
	}
	if strings.Contains(out, "Controller concerns:") {
		t.Fatalf("did not expect 'Controller concerns:' in output:\n%s", out)
	}
}

func TestConcernsCommandSupportsAbsolutePaths(t *testing.T) {
	root := t.TempDir()
	concernsDir := t.TempDir()
	mustWriteCmdFile(t, filepath.Join(root, "config", "application.rb"), "")
	mustWriteCmdFile(t, filepath.Join(root, ".rails-kit.yml"), "model_concerns_path: "+concernsDir+"\n")
	mustWriteCmdFile(t, filepath.Join(concernsDir, "taggable.rb"), "module Taggable\n  extend ActiveSupport::Concern\nend\n")

	out, errOut, err := runCmdForTest(t, concernsCmd, root, []string{})
	if err != nil {
		t.Fatalf("unexpected error: %v\nstderr:%s", err, errOut)
	}
	if !strings.Contains(out, "taggable") {
		t.Fatalf("expected taggable in output:\n%s", out)
	}
}
