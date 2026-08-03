package cmd

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestSchemaCommandSupportsAbsoluteSchemaPath(t *testing.T) {
	root := t.TempDir()
	external := t.TempDir()
	mustWriteCmdFile(t, filepath.Join(root, "config", "application.rb"), "")
	mustWriteCmdFile(t, filepath.Join(root, ".rails-kit.yml"), "schema_path: "+filepath.Join(external, "schema.rb")+"\n")
	mustWriteCmdFile(t, filepath.Join(external, "schema.rb"), strings.Join([]string{
		`ActiveRecord::Schema[7.2].define(version: 2024_01_01_000001) do`,
		`  create_table "users", force: :cascade do |t|`,
		`  end`,
		`end`,
		"",
	}, "\n"))

	out, errOut, err := runCmdForTest(t, schemaCmd, root, []string{})
	if err != nil {
		t.Fatalf("unexpected error: %v\nstderr:%s", err, errOut)
	}
	if !strings.Contains(out, "users") {
		t.Fatalf("unexpected output: %s", out)
	}
}

func TestFixturesCommandSupportsNamespacedIrregularFixture(t *testing.T) {
	root := t.TempDir()
	mustWriteCmdFile(t, filepath.Join(root, "config", "application.rb"), "")
	mustWriteCmdFile(t, filepath.Join(root, "test", "fixtures", "admin", "people.yml"), "alice:\n  name: Alice\n")

	out, errOut, err := runCmdForTest(t, fixturesCmd, root, []string{"admin/person"})
	if err != nil {
		t.Fatalf("unexpected error: %v\nstderr:%s", err, errOut)
	}
	if !strings.Contains(out, "admin/people.yml") {
		t.Fatalf("unexpected output: %s", out)
	}
	if !strings.Contains(out, "alice: name: Alice") {
		t.Fatalf("unexpected output: %s", out)
	}
}

func TestFixturesCommandSupportsAbsoluteFixturesPath(t *testing.T) {
	root := t.TempDir()
	fixturesDir := filepath.Join(t.TempDir(), "fixtures")
	mustWriteCmdFile(t, filepath.Join(root, "config", "application.rb"), "")
	mustWriteCmdFile(t, filepath.Join(root, ".rails-kit.yml"), "fixtures_path: "+fixturesDir+"\n")
	mustWriteCmdFile(t, filepath.Join(fixturesDir, "users.yml"), "alice:\n  name: Alice\n")

	out, errOut, err := runCmdForTest(t, fixturesCmd, root, []string{"user"})
	if err != nil {
		t.Fatalf("unexpected error: %v\nstderr:%s", err, errOut)
	}
	if !strings.Contains(out, "users.yml") {
		t.Fatalf("unexpected output: %s", out)
	}
}

func TestFixturesCommandFailsForMissingConfiguredDir(t *testing.T) {
	root := t.TempDir()
	mustWriteCmdFile(t, filepath.Join(root, "config", "application.rb"), "")
	mustWriteCmdFile(t, filepath.Join(root, ".rails-kit.yml"), "fixtures_path: missing/fixtures\n")

	_, _, err := runCmdForTest(t, fixturesCmd, root, []string{"user"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "fixtures path") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFixturesCommandOmitsMetadataAndNormalizesERB(t *testing.T) {
	root := t.TempDir()
	mustWriteCmdFile(t, filepath.Join(root, "config", "application.rb"), "")
	mustWriteCmdFile(t, filepath.Join(root, "test", "fixtures", "users.yml"), "_fixture:\n  ignore: true\n_hidden_but_real:\n  name: Hidden User\nalice:\n  name: '<%= ENV[\"USER\"] %>'\n")

	out, errOut, err := runCmdForTest(t, fixturesCmd, root, []string{"user"})
	if err != nil {
		t.Fatalf("unexpected error: %v\nstderr:%s", err, errOut)
	}
	if strings.Contains(out, "_fixture") {
		t.Fatalf("metadata should be omitted, got:\n%s", out)
	}
	if !strings.Contains(out, "alice: name: __ERB__") {
		t.Fatalf("expected normalized ERB placeholder, got:\n%s", out)
	}
	if !strings.Contains(out, "_hidden_but_real: name: Hidden User") {
		t.Fatalf("expected real underscore fixture to remain visible, got:\n%s", out)
	}
}

func TestFixturesCommandRejectsStructuralERB(t *testing.T) {
	root := t.TempDir()
	mustWriteCmdFile(t, filepath.Join(root, "config", "application.rb"), "")
	mustWriteCmdFile(t, filepath.Join(root, "test", "fixtures", "users.yml"), "<% 2.times do |n| %>\nuser_<%= n %>:\n  email: user_<%= n %>@example.com\n<% end %>\n")

	out, errOut, err := runCmdForTest(t, fixturesCmd, root, []string{"user"})
	if err == nil {
		t.Fatal("expected error")
	}
	if out != "" {
		t.Fatalf("expected no stdout, got:\n%s", out)
	}
	if errOut != "" {
		t.Fatalf("expected no stderr, got:\n%s", errOut)
	}
	if !strings.Contains(err.Error(), "cannot be summarized safely") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFixturesCommandSupportsBlockScalarERB(t *testing.T) {
	root := t.TempDir()
	mustWriteCmdFile(t, filepath.Join(root, "config", "application.rb"), "")
	mustWriteCmdFile(t, filepath.Join(root, "test", "fixtures", "users.yml"), "alice:\n  bio: |\n    Hello\n    <%= ENV[\"USER\"] %>\n")

	out, errOut, err := runCmdForTest(t, fixturesCmd, root, []string{"user"})
	if err != nil {
		t.Fatalf("unexpected error: %v\nstderr:%s", err, errOut)
	}
	if !strings.Contains(out, "alice: bio: Hello") || !strings.Contains(out, "__ERB__") {
		t.Fatalf("expected normalized block scalar output, got:\n%s", out)
	}
}

func TestFixturesCommandSupportsListItemERB(t *testing.T) {
	root := t.TempDir()
	mustWriteCmdFile(t, filepath.Join(root, "config", "application.rb"), "")
	mustWriteCmdFile(t, filepath.Join(root, "test", "fixtures", "users.yml"), "alice:\n  tags:\n    - <%= ENV[\"USER\"] %>\n")

	out, errOut, err := runCmdForTest(t, fixturesCmd, root, []string{"user"})
	if err != nil {
		t.Fatalf("unexpected error: %v\nstderr:%s", err, errOut)
	}
	if !strings.Contains(out, "alice: tags: [__ERB__]") {
		t.Fatalf("expected normalized list item output, got:\n%s", out)
	}
}

func TestFixturesCommandRejectsMixedControlFlowERB(t *testing.T) {
	root := t.TempDir()
	mustWriteCmdFile(t, filepath.Join(root, "config", "application.rb"), "")
	mustWriteCmdFile(t, filepath.Join(root, "test", "fixtures", "users.yml"), "alice:\n  <% if ENV[\"SHOW_EMAIL\"] %>email: <%= ENV[\"SHOW_EMAIL\"] %><% end %>\n")

	out, errOut, err := runCmdForTest(t, fixturesCmd, root, []string{"user"})
	if err == nil {
		t.Fatal("expected error")
	}
	if out != "" {
		t.Fatalf("expected no stdout, got:\n%s", out)
	}
	if errOut != "" {
		t.Fatalf("expected no stderr, got:\n%s", errOut)
	}
	if !strings.Contains(err.Error(), "cannot be summarized safely") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFixturesCommandListsFilesAsJSON(t *testing.T) {
	root := t.TempDir()
	mustWriteCmdFile(t, filepath.Join(root, "config", "application.rb"), "")
	mustWriteCmdFile(t, filepath.Join(root, "test", "fixtures", "users.yml"), "alice:\n  name: Alice\n")
	mustWriteCmdFile(t, filepath.Join(root, "test", "fixtures", "admin", "dashboards.yml"), "main:\n  name: Main\n")

	out, errOut, err := runCmdForTestJSON(t, fixturesCmd, root, []string{})
	if err != nil {
		t.Fatalf("unexpected error: %v\nstderr:%s", err, errOut)
	}

	var payload struct {
		Files []string `json:"files"`
	}
	unwrapJSONEnvelope(t, out, "fixtures", &payload)
	if len(payload.Files) != 2 || payload.Files[0] != "admin/dashboards" || payload.Files[1] != "users" {
		t.Fatalf("unexpected files: %#v", payload.Files)
	}
}

func TestFixturesCommandShowsEntriesAsJSON(t *testing.T) {
	root := t.TempDir()
	mustWriteCmdFile(t, filepath.Join(root, "config", "application.rb"), "")
	mustWriteCmdFile(t, filepath.Join(root, "test", "fixtures", "users.yml"), "_fixture:\n  ignore: true\nalice:\n  name: '<%= ENV[\"USER\"] %>'\n  tags:\n    - admin\n")

	out, errOut, err := runCmdForTestJSON(t, fixturesCmd, root, []string{"user"})
	if err != nil {
		t.Fatalf("unexpected error: %v\nstderr:%s", err, errOut)
	}

	var payload struct {
		File    string                 `json:"file"`
		Entries map[string]interface{} `json:"entries"`
	}
	unwrapJSONEnvelope(t, out, "fixtures", &payload)
	if payload.File != "users.yml" {
		t.Fatalf("file = %q, want users.yml", payload.File)
	}
	if _, ok := payload.Entries["_fixture"]; ok {
		t.Fatalf("metadata should be omitted, got: %#v", payload.Entries)
	}
	alice, ok := payload.Entries["alice"].(map[string]interface{})
	if !ok {
		t.Fatalf("alice entry missing or wrong type: %#v", payload.Entries["alice"])
	}
	if alice["name"] != "__ERB__" {
		t.Fatalf("name = %#v, want __ERB__", alice["name"])
	}
}

func TestLocalesCommandSupportsAbsoluteLocalesPath(t *testing.T) {
	root := t.TempDir()
	external := t.TempDir()
	mustWriteCmdFile(t, filepath.Join(root, "config", "application.rb"), "")
	mustWriteCmdFile(t, filepath.Join(root, ".rails-kit.yml"), "locales_path: "+filepath.Join(external, "locales")+"\n")
	mustWriteCmdFile(t, filepath.Join(external, "locales", "en.yml"), "en:\n  views:\n    users:\n      title: Users\n")

	out, errOut, err := runCmdForTest(t, localesCmd, root, []string{"en.views.users"})
	if err != nil {
		t.Fatalf("unexpected error: %v\nstderr:%s", err, errOut)
	}
	if !strings.Contains(out, "# en.views.users") {
		t.Fatalf("unexpected output: %s", out)
	}
	if !strings.Contains(out, "title: Users") {
		t.Fatalf("unexpected output: %s", out)
	}
}

func TestLocalesCommandListsScopesAsJSON(t *testing.T) {
	root := t.TempDir()
	mustWriteCmdFile(t, filepath.Join(root, "config", "application.rb"), "")
	mustWriteCmdFile(t, filepath.Join(root, "config", "locales", "en.yml"), "en:\n  views:\n    users:\n      title: Users\n  admin:\n    dashboards:\n      title: Dashboards\n")

	out, errOut, err := runCmdForTestJSON(t, localesCmd, root, []string{})
	if err != nil {
		t.Fatalf("unexpected error: %v\nstderr:%s", err, errOut)
	}

	var payload struct {
		Scopes []string `json:"scopes"`
	}
	unwrapJSONEnvelope(t, out, "locales", &payload)
	if len(payload.Scopes) != 4 || payload.Scopes[0] != "en.admin" || payload.Scopes[1] != "en.admin.dashboards" || payload.Scopes[2] != "en.views" || payload.Scopes[3] != "en.views.users" {
		t.Fatalf("unexpected scopes: %#v", payload.Scopes)
	}
}

func TestLocalesCommandShowsScopeAsJSON(t *testing.T) {
	root := t.TempDir()
	mustWriteCmdFile(t, filepath.Join(root, "config", "application.rb"), "")
	mustWriteCmdFile(t, filepath.Join(root, "config", "locales", "en.yml"), "en:\n  views:\n    users:\n      title: Users\n      labels:\n        - first\n        - second\n")

	out, errOut, err := runCmdForTestJSON(t, localesCmd, root, []string{"en.views.users"})
	if err != nil {
		t.Fatalf("unexpected error: %v\nstderr:%s", err, errOut)
	}

	var payload struct {
		Scope string                 `json:"scope"`
		Value map[string]interface{} `json:"value"`
	}
	unwrapJSONEnvelope(t, out, "locales", &payload)
	if payload.Scope != "en.views.users" {
		t.Fatalf("scope = %q, want en.views.users", payload.Scope)
	}
	if payload.Value["title"] != "Users" {
		t.Fatalf("title = %#v, want Users", payload.Value["title"])
	}
	labels, ok := payload.Value["labels"].([]interface{})
	if !ok || len(labels) != 2 || labels[0] != "first" || labels[1] != "second" {
		t.Fatalf("labels = %#v, want [first second]", payload.Value["labels"])
	}
}

func TestLocalesCommandFailsForMissingConfiguredDir(t *testing.T) {
	root := t.TempDir()
	mustWriteCmdFile(t, filepath.Join(root, "config", "application.rb"), "")
	mustWriteCmdFile(t, filepath.Join(root, ".rails-kit.yml"), "locales_path: missing/locales\n")

	_, _, err := runCmdForTest(t, localesCmd, root, []string{})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "locales path") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLocalesCommandPrintsYamlLikeArrays(t *testing.T) {
	root := t.TempDir()
	mustWriteCmdFile(t, filepath.Join(root, "config", "application.rb"), "")
	mustWriteCmdFile(t, filepath.Join(root, "config", "locales", "en.yml"), "en:\n  views:\n    users:\n      labels:\n        - first\n        - second\n")

	out, errOut, err := runCmdForTest(t, localesCmd, root, []string{"en.views.users"})
	if err != nil {
		t.Fatalf("unexpected error: %v\nstderr:%s", err, errOut)
	}
	for _, fragment := range []string{
		"# en.views.users\n",
		"labels:\n",
		"  - first\n",
		"  - second\n",
	} {
		if !strings.Contains(out, fragment) {
			t.Fatalf("expected fragment %q in output:\n%s", fragment, out)
		}
	}
}

func TestModelCommandSupportsAbsoluteModelsPath(t *testing.T) {
	root := t.TempDir()
	modelsDir := filepath.Join(t.TempDir(), "models")
	mustWriteCmdFile(t, filepath.Join(root, "config", "application.rb"), "")
	mustWriteCmdFile(t, filepath.Join(root, ".rails-kit.yml"), "models_path: "+modelsDir+"\n")
	mustWriteCmdFile(t, filepath.Join(modelsDir, "user.rb"), "class User\nend\n")

	out, errOut, err := runCmdForTest(t, modelCmd, root, []string{"user"})
	if err != nil {
		t.Fatalf("unexpected error: %v\nstderr:%s", err, errOut)
	}
	if !strings.Contains(out, "User (") {
		t.Fatalf("unexpected output: %s", out)
	}
}

func TestModelCommandJSONIncludesParentClassAndTableName(t *testing.T) {
	root := t.TempDir()
	mustWriteCmdFile(t, filepath.Join(root, "config", "application.rb"), "")
	mustWriteCmdFile(t, filepath.Join(root, "app", "models", "report.rb"), strings.Join([]string{
		"class Report < ApplicationRecord",
		`  self.table_name = "legacy_reports"`,
		"end",
		"",
	}, "\n"))

	out, errOut, err := runCmdForTestJSON(t, modelCmd, root, []string{"report"})
	if err != nil {
		t.Fatalf("unexpected error: %v\nstderr:%s", err, errOut)
	}

	var payload struct {
		ClassName   string `json:"class_name"`
		ParentClass string `json:"parent_class"`
		TableName   string `json:"table_name"`
	}
	unwrapJSONEnvelope(t, out, "model", &payload)
	if payload.ClassName != "Report" || payload.ParentClass != "ApplicationRecord" || payload.TableName != "legacy_reports" {
		t.Fatalf("unexpected payload: %#v", payload)
	}
}

func TestModelCommandReturnsPartialJSONAndWarnsOnParseErrors(t *testing.T) {
	root := t.TempDir()
	modelPath := filepath.Join(root, "app", "models", "broken.rb")
	mustWriteCmdFile(t, filepath.Join(root, "config", "application.rb"), "")
	mustWriteCmdFile(t, modelPath, "class Broken < ApplicationRecord\n  validates :name, presence: true\n  def call(\nend\n")

	out, errOut, err := runCmdForTestJSON(t, modelCmd, root, []string{"broken"})
	if err != nil {
		t.Fatalf("unexpected error: %v\nstderr:%s", err, errOut)
	}
	var payload struct {
		ClassName   string   `json:"class_name"`
		ParentClass string   `json:"parent_class"`
		Validations []string `json:"validations"`
		ParseErrors []string `json:"parse_errors"`
	}
	unwrapJSONEnvelope(t, out, "model", &payload)
	if payload.ClassName != "Broken" || payload.ParentClass != "ApplicationRecord" || len(payload.Validations) != 1 {
		t.Fatalf("unexpected partial payload: %#v", payload)
	}
	if payload.ParseErrors != nil {
		t.Fatalf("parse_errors leaked into JSON contract: %#v", payload.ParseErrors)
	}
	if !strings.Contains(filepath.ToSlash(errOut), "Warning: ") || !strings.Contains(filepath.ToSlash(errOut), "/app/models/broken.rb:") {
		t.Fatalf("expected path- and line-specific warning, got %q", errOut)
	}
}

func TestRelatedCommandSupportsAbsoluteConfiguredPaths(t *testing.T) {
	root := t.TempDir()
	modelsDir := filepath.Join(t.TempDir(), "models")
	fixturesDir := filepath.Join(t.TempDir(), "fixtures")
	mustWriteCmdFile(t, filepath.Join(root, "config", "application.rb"), "")
	mustWriteCmdFile(t, filepath.Join(root, ".rails-kit.yml"), "models_path: "+modelsDir+"\nfixtures_path: "+fixturesDir+"\n")
	mustWriteCmdFile(t, filepath.Join(modelsDir, "user.rb"), "class User\nend\n")
	mustWriteCmdFile(t, filepath.Join(fixturesDir, "users.yml"), "alice:\n  name: Alice\n")

	out, errOut, err := runCmdForTest(t, relatedCmd, root, []string{"user"})
	if err != nil {
		t.Fatalf("unexpected error: %v\nstderr:%s", err, errOut)
	}
	if !strings.Contains(out, "Model:\n") {
		t.Fatalf("unexpected output: %s", out)
	}
	if !strings.Contains(out, "Fixtures:\n") {
		t.Fatalf("unexpected output: %s", out)
	}
}

func TestRelatedCommandSupportsConfiguredServicePath(t *testing.T) {
	root := t.TempDir()
	mustWriteCmdFile(t, filepath.Join(root, "config", "application.rb"), "")
	mustWriteCmdFile(t, filepath.Join(root, ".rails-kit.yml"), "services_path: app/workflows\n")
	mustWriteCmdFile(t, filepath.Join(root, "app", "models", "user.rb"), "class User\nend\n")
	mustWriteCmdFile(t, filepath.Join(root, "app", "workflows", "user_export_service.rb"), "")

	out, errOut, err := runCmdForTest(t, relatedCmd, root, []string{"app/workflows/user_export_service.rb"})
	if err != nil {
		t.Fatalf("unexpected error: %v\nstderr:%s", err, errOut)
	}
	if !strings.Contains(out, "Model:\n") || !strings.Contains(out, "app/models/user.rb") {
		t.Fatalf("unexpected output: %s", out)
	}
	if !strings.Contains(out, "Service:\n") || !strings.Contains(out, "app/workflows/user_export_service.rb") {
		t.Fatalf("unexpected output: %s", out)
	}
}

func TestRelatedCommandExcludesNamespacedMatchesForRootModel(t *testing.T) {
	root := t.TempDir()
	mustWriteCmdFile(t, filepath.Join(root, "config", "application.rb"), "")
	mustWriteCmdFile(t, filepath.Join(root, "app", "models", "user.rb"), "class User\nend\n")
	mustWriteCmdFile(t, filepath.Join(root, "app", "controllers", "users_controller.rb"), "class UsersController\nend\n")
	mustWriteCmdFile(t, filepath.Join(root, "app", "controllers", "admin", "users_controller.rb"), "class Admin::UsersController\nend\n")
	mustWriteCmdFile(t, filepath.Join(root, "app", "views", "users", "index.html.erb"), "root\n")
	mustWriteCmdFile(t, filepath.Join(root, "app", "views", "admin", "users", "index.html.erb"), "admin\n")

	out, errOut, err := runCmdForTest(t, relatedCmd, root, []string{"user"})
	if err != nil {
		t.Fatalf("unexpected error: %v\nstderr:%s", err, errOut)
	}
	if strings.Contains(out, "app/controllers/admin/users_controller.rb") {
		t.Fatalf("did not expect namespaced controller in output:\n%s", out)
	}
	if strings.Contains(out, "app/views/admin/users/index.html.erb") {
		t.Fatalf("did not expect namespaced views in output:\n%s", out)
	}
	if !strings.Contains(out, "app/controllers/users_controller.rb") {
		t.Fatalf("expected root controller in output:\n%s", out)
	}
	if !strings.Contains(out, "app/views/users/index.html.erb") {
		t.Fatalf("expected root views in output:\n%s", out)
	}
}

func TestRelatedCommandSupportsViewPath(t *testing.T) {
	root := t.TempDir()
	mustWriteCmdFile(t, filepath.Join(root, "config", "application.rb"), "")
	mustWriteCmdFile(t, filepath.Join(root, "app", "models", "admin", "billing", "invoice.rb"), "class Admin::Billing::Invoice\nend\n")
	mustWriteCmdFile(t, filepath.Join(root, "app", "views", "admin", "billing", "invoices", "shared", "_form.html.erb"), "")

	out, errOut, err := runCmdForTest(t, relatedCmd, root, []string{"app/views/admin/billing/invoices/shared/_form.html.erb"})
	if err != nil {
		t.Fatalf("unexpected error: %v\nstderr:%s", err, errOut)
	}
	if !strings.Contains(out, "app/models/admin/billing/invoice.rb") {
		t.Fatalf("expected model in output:\n%s", out)
	}
	if !strings.Contains(out, "app/views/admin/billing/invoices/shared/_form.html.erb") {
		t.Fatalf("expected view path in output:\n%s", out)
	}
}

func TestRelatedCommandSupportsServiceAndFormerPaths(t *testing.T) {
	root := t.TempDir()
	mustWriteCmdFile(t, filepath.Join(root, "config", "application.rb"), "")
	mustWriteCmdFile(t, filepath.Join(root, "app", "models", "admin", "user.rb"), "class Admin::User\nend\n")
	mustWriteCmdFile(t, filepath.Join(root, "app", "services", "admin", "user_export_service.rb"), "class Admin::UserExportService\nend\n")
	mustWriteCmdFile(t, filepath.Join(root, "app", "services", "admin", "reports", "user_export_service.rb"), "class Admin::Reports::UserExportService\nend\n")
	mustWriteCmdFile(t, filepath.Join(root, "app", "formers", "admin", "user_former.rb"), "class Admin::UserFormer\nend\n")
	mustWriteCmdFile(t, filepath.Join(root, "app", "formers", "admin", "reports", "user_former.rb"), "class Admin::Reports::UserFormer\nend\n")

	for _, arg := range []string{"app/services/admin/user_export_service.rb", "app/formers/admin/user_former.rb"} {
		out, errOut, err := runCmdForTest(t, relatedCmd, root, []string{arg})
		if err != nil {
			t.Fatalf("unexpected error for %s: %v\nstderr:%s", arg, err, errOut)
		}
		if !strings.Contains(out, "app/models/admin/user.rb") {
			t.Fatalf("expected model in output for %s:\n%s", arg, out)
		}
		if strings.Contains(out, "app/services/admin/reports/user_export_service.rb") {
			t.Fatalf("did not expect deeper namespace service for %s:\n%s", arg, out)
		}
		if strings.Contains(out, "app/formers/admin/reports/user_former.rb") {
			t.Fatalf("did not expect deeper namespace former for %s:\n%s", arg, out)
		}
	}
}

func TestRelatedCommandExcludesNamespacedServicesAndFormersForRootModel(t *testing.T) {
	root := t.TempDir()
	mustWriteCmdFile(t, filepath.Join(root, "config", "application.rb"), "")
	mustWriteCmdFile(t, filepath.Join(root, "app", "models", "user.rb"), "class User\nend\n")
	mustWriteCmdFile(t, filepath.Join(root, "app", "services", "user_export_service.rb"), "class UserExportService\nend\n")
	mustWriteCmdFile(t, filepath.Join(root, "app", "services", "admin", "user_export_service.rb"), "class Admin::UserExportService\nend\n")
	mustWriteCmdFile(t, filepath.Join(root, "app", "formers", "user_former.rb"), "class UserFormer\nend\n")
	mustWriteCmdFile(t, filepath.Join(root, "app", "formers", "admin", "user_former.rb"), "class Admin::UserFormer\nend\n")

	out, errOut, err := runCmdForTest(t, relatedCmd, root, []string{"user"})
	if err != nil {
		t.Fatalf("unexpected error: %v\nstderr:%s", err, errOut)
	}
	if strings.Contains(out, "app/services/admin/user_export_service.rb") {
		t.Fatalf("did not expect namespaced service in output:\n%s", out)
	}
	if strings.Contains(out, "app/formers/admin/user_former.rb") {
		t.Fatalf("did not expect namespaced former in output:\n%s", out)
	}
	if !strings.Contains(out, "app/services/user_export_service.rb") {
		t.Fatalf("expected root service in output:\n%s", out)
	}
	if !strings.Contains(out, "app/formers/user_former.rb") {
		t.Fatalf("expected root former in output:\n%s", out)
	}
}

func TestRelatedCommandExcludesDeeperNamespaceServicesAndFormers(t *testing.T) {
	root := t.TempDir()
	mustWriteCmdFile(t, filepath.Join(root, "config", "application.rb"), "")
	mustWriteCmdFile(t, filepath.Join(root, "app", "models", "admin", "user.rb"), "class Admin::User\nend\n")
	mustWriteCmdFile(t, filepath.Join(root, "app", "services", "admin", "user_export_service.rb"), "class Admin::UserExportService\nend\n")
	mustWriteCmdFile(t, filepath.Join(root, "app", "services", "admin", "reports", "user_export_service.rb"), "class Admin::Reports::UserExportService\nend\n")
	mustWriteCmdFile(t, filepath.Join(root, "app", "formers", "admin", "user_former.rb"), "class Admin::UserFormer\nend\n")
	mustWriteCmdFile(t, filepath.Join(root, "app", "formers", "admin", "reports", "user_former.rb"), "class Admin::Reports::UserFormer\nend\n")

	out, errOut, err := runCmdForTest(t, relatedCmd, root, []string{"admin/user"})
	if err != nil {
		t.Fatalf("unexpected error: %v\nstderr:%s", err, errOut)
	}
	if strings.Contains(out, "app/services/admin/reports/user_export_service.rb") {
		t.Fatalf("did not expect deeper namespace service in output:\n%s", out)
	}
	if strings.Contains(out, "app/formers/admin/reports/user_former.rb") {
		t.Fatalf("did not expect deeper namespace former in output:\n%s", out)
	}
	if !strings.Contains(out, "app/services/admin/user_export_service.rb") {
		t.Fatalf("expected exact namespace service in output:\n%s", out)
	}
	if !strings.Contains(out, "app/formers/admin/user_former.rb") {
		t.Fatalf("expected exact namespace former in output:\n%s", out)
	}
}

// unwrapJSONEnvelope decodes out as a jsonEnvelope, asserts schema_version
// and command, and unmarshals its data field into v.
func unwrapJSONEnvelope(t *testing.T, out, command string, v any) {
	t.Helper()
	var envelope struct {
		SchemaVersion int             `json:"schema_version"`
		Command       string          `json:"command"`
		Data          json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal([]byte(out), &envelope); err != nil {
		t.Fatalf("unmarshal envelope: %v\noutput:%s", err, out)
	}
	if envelope.SchemaVersion != jsonSchemaVersion {
		t.Fatalf("schema_version = %d, want %d", envelope.SchemaVersion, jsonSchemaVersion)
	}
	if envelope.Command != command {
		t.Fatalf("command = %q, want %q", envelope.Command, command)
	}
	if err := json.Unmarshal(envelope.Data, v); err != nil {
		t.Fatalf("unmarshal data: %v\ndata:%s", err, envelope.Data)
	}
}

func runCmdForTest(t *testing.T, c *cobra.Command, root string, args []string) (string, string, error) {
	t.Helper()

	prevWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	prevRootFlag := rootFlag
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	rootFlag = ""
	t.Cleanup(func() {
		_ = os.Chdir(prevWD)
		rootFlag = prevRootFlag
	})

	oldStdout := os.Stdout
	oldStderr := os.Stderr
	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	stderrR, stderrW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = stdoutW
	os.Stderr = stderrW
	t.Cleanup(func() {
		os.Stdout = oldStdout
		os.Stderr = oldStderr
	})

	// Drain both pipes concurrently with RunE: their OS buffer is small
	// enough (especially on Windows) that a command writing more than
	// that before returning would otherwise deadlock the writer.
	var stdoutBytes, stderrBytes []byte
	var stdoutErr, stderrErr error
	done := make(chan struct{})
	go func() {
		defer close(done)
		stdoutBytes, stdoutErr = io.ReadAll(stdoutR)
	}()
	stderrDone := make(chan struct{})
	go func() {
		defer close(stderrDone)
		stderrBytes, stderrErr = io.ReadAll(stderrR)
	}()

	err = c.RunE(c, args)

	_ = stdoutW.Close()
	_ = stderrW.Close()
	<-done
	<-stderrDone
	if stdoutErr != nil {
		t.Fatal(stdoutErr)
	}
	if stderrErr != nil {
		t.Fatal(stderrErr)
	}
	return string(stdoutBytes), string(stderrBytes), err
}

func runCmdForTestJSON(t *testing.T, c *cobra.Command, root string, args []string) (string, string, error) {
	t.Helper()

	prevJSONFlag := jsonFlag
	jsonFlag = true
	t.Cleanup(func() {
		jsonFlag = prevJSONFlag
	})

	return runCmdForTest(t, c, root, args)
}

func mustWriteCmdFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
