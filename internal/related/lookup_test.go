package related_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/janstol/rails-kit/internal/config"
	"github.com/janstol/rails-kit/internal/pluralize"
	"github.com/janstol/rails-kit/internal/related"
)

func TestResolveLookup_AmbiguousBareName(t *testing.T) {
	root := t.TempDir()
	mustWriteLookupFile(t, filepath.Join(root, "app", "models", "admin", "dashboard.rb"), "class Admin::Dashboard\nend\n")
	mustWriteLookupFile(t, filepath.Join(root, "app", "models", "analytics", "dashboard.rb"), "class Analytics::Dashboard\nend\n")

	_, _, err := related.ResolveLookup(root, defaultLookupTestConfig(), "dashboard", pluralize.Default())
	if err == nil {
		t.Fatal("expected ambiguity error")
	}
	if !strings.Contains(err.Error(), "ambiguous model name") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolveLookup_UniqueNamespacedModel(t *testing.T) {
	root := t.TempDir()
	mustWriteLookupFile(t, filepath.Join(root, "app", "models", "admin", "dashboard.rb"), "class Admin::Dashboard\nend\n")

	name, plural, err := related.ResolveLookup(root, defaultLookupTestConfig(), "dashboard", pluralize.Default())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "admin/dashboard" {
		t.Fatalf("name = %q, want admin/dashboard", name)
	}
	if plural != "dashboards" {
		t.Fatalf("plural = %q, want dashboards", plural)
	}
}

func TestResolveLookup_MissingModelsPath(t *testing.T) {
	root := t.TempDir()

	cfg := defaultLookupTestConfig()
	cfg.ModelsPath = "lib/missing_models"
	_, _, err := related.ResolveLookup(root, cfg, "user", pluralize.Default())
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "models path") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolveLookup_NamespacedModelMustExist(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "app", "models"), 0o755); err != nil {
		t.Fatal(err)
	}
	mustWriteLookupFile(t, filepath.Join(root, "app", "controllers", "admin", "dashboards_controller.rb"), "class Admin::DashboardsController\nend\n")

	_, _, err := related.ResolveLookup(root, defaultLookupTestConfig(), "admin/dashboard", pluralize.Default())
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "model file not found") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolveLookup_ControllerPath(t *testing.T) {
	root := t.TempDir()
	mustWriteLookupFile(t, filepath.Join(root, "app", "models", "user.rb"), "class User\nend\n")
	mustWriteLookupFile(t, filepath.Join(root, "app", "controllers", "users_controller.rb"), "class UsersController\nend\n")

	name, plural, err := related.ResolveLookup(root, defaultLookupTestConfig(), "app/controllers/users_controller.rb", pluralize.Default())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "user" {
		t.Fatalf("name = %q, want user", name)
	}
	if plural != "users" {
		t.Fatalf("plural = %q, want users", plural)
	}
}

func TestResolveLookup_ViewPath(t *testing.T) {
	root := t.TempDir()
	mustWriteLookupFile(t, filepath.Join(root, "app", "models", "user.rb"), "class User\nend\n")
	mustWriteLookupFile(t, filepath.Join(root, "app", "views", "users", "show.html.erb"), "")

	name, plural, err := related.ResolveLookup(root, defaultLookupTestConfig(), "app/views/users/show.html.erb", pluralize.Default())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "user" {
		t.Fatalf("name = %q, want user", name)
	}
	if plural != "users" {
		t.Fatalf("plural = %q, want users", plural)
	}
}

func TestResolveLookup_NamespacedDeepViewPath(t *testing.T) {
	root := t.TempDir()
	mustWriteLookupFile(t, filepath.Join(root, "app", "models", "admin", "billing", "invoice.rb"), "class Admin::Billing::Invoice\nend\n")
	mustWriteLookupFile(t, filepath.Join(root, "app", "views", "admin", "billing", "invoices", "shared", "_form.html.erb"), "")

	name, plural, err := related.ResolveLookup(root, defaultLookupTestConfig(), "app/views/admin/billing/invoices/shared/_form.html.erb", pluralize.Default())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "admin/billing/invoice" {
		t.Fatalf("name = %q, want admin/billing/invoice", name)
	}
	if plural != "invoices" {
		t.Fatalf("plural = %q, want invoices", plural)
	}
}

func TestResolveLookup_ServicePath(t *testing.T) {
	root := t.TempDir()
	mustWriteLookupFile(t, filepath.Join(root, "app", "models", "user.rb"), "class User\nend\n")
	mustWriteLookupFile(t, filepath.Join(root, "app", "services", "user_export_service.rb"), "")

	name, plural, err := related.ResolveLookup(root, defaultLookupTestConfig(), "app/services/user_export_service.rb", pluralize.Default())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "user" {
		t.Fatalf("name = %q, want user", name)
	}
	if plural != "users" {
		t.Fatalf("plural = %q, want users", plural)
	}
}

func TestResolveLookup_FormerPath(t *testing.T) {
	root := t.TempDir()
	mustWriteLookupFile(t, filepath.Join(root, "app", "models", "user.rb"), "class User\nend\n")
	mustWriteLookupFile(t, filepath.Join(root, "app", "formers", "user_former.rb"), "")

	name, plural, err := related.ResolveLookup(root, defaultLookupTestConfig(), "app/formers/user_former.rb", pluralize.Default())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "user" {
		t.Fatalf("name = %q, want user", name)
	}
	if plural != "users" {
		t.Fatalf("plural = %q, want users", plural)
	}
}

func TestResolveLookup_DecoratorPath(t *testing.T) {
	root := t.TempDir()
	mustWriteLookupFile(t, filepath.Join(root, "app", "models", "user.rb"), "class User\nend\n")
	mustWriteLookupFile(t, filepath.Join(root, "app", "decorators", "user_decorator.rb"), "")

	name, plural, err := related.ResolveLookup(root, defaultLookupTestConfig(), "app/decorators/user_decorator.rb", pluralize.Default())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "user" {
		t.Fatalf("name = %q, want user", name)
	}
	if plural != "users" {
		t.Fatalf("plural = %q, want users", plural)
	}
}

func TestResolveLookup_DatagridPath(t *testing.T) {
	root := t.TempDir()
	mustWriteLookupFile(t, filepath.Join(root, "app", "models", "admin", "dashboard.rb"), "class Admin::Dashboard\nend\n")
	mustWriteLookupFile(t, filepath.Join(root, "app", "datagrids", "admin", "dashboards_datagrid.rb"), "")

	name, plural, err := related.ResolveLookup(root, defaultLookupTestConfig(), "app/datagrids/admin/dashboards_datagrid.rb", pluralize.Default())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "admin/dashboard" {
		t.Fatalf("name = %q, want admin/dashboard", name)
	}
	if plural != "dashboards" {
		t.Fatalf("plural = %q, want dashboards", plural)
	}
}

func TestResolveLookup_ControllerSpecPath(t *testing.T) {
	root := t.TempDir()
	mustWriteLookupFile(t, filepath.Join(root, "app", "models", "user.rb"), "class User\nend\n")
	mustWriteLookupFile(t, filepath.Join(root, "spec", "controllers", "users_controller_spec.rb"), "")

	name, plural, err := related.ResolveLookup(root, defaultLookupTestConfig(), "spec/controllers/users_controller_spec.rb", pluralize.Default())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "user" {
		t.Fatalf("name = %q, want user", name)
	}
	if plural != "users" {
		t.Fatalf("plural = %q, want users", plural)
	}
}

func TestResolveLookup_UnsupportedPath(t *testing.T) {
	root := t.TempDir()

	_, _, err := related.ResolveLookup(root, defaultLookupTestConfig(), "app/unknown/user_thing.rb", pluralize.Default())
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "unsupported related path") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolveLookup_JobPath(t *testing.T) {
	root := t.TempDir()
	mustWriteLookupFile(t, filepath.Join(root, "app", "models", "user.rb"), "class User\nend\n")
	mustWriteLookupFile(t, filepath.Join(root, "app", "jobs", "user_job.rb"), "")

	name, plural, err := related.ResolveLookup(root, defaultLookupTestConfig(), "app/jobs/user_job.rb", pluralize.Default())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "user" {
		t.Fatalf("name = %q, want user", name)
	}
	if plural != "users" {
		t.Fatalf("plural = %q, want users", plural)
	}
}

func TestResolveLookup_MailerPath(t *testing.T) {
	root := t.TempDir()
	mustWriteLookupFile(t, filepath.Join(root, "app", "models", "user.rb"), "class User\nend\n")
	mustWriteLookupFile(t, filepath.Join(root, "app", "mailers", "user_mailer.rb"), "")

	name, plural, err := related.ResolveLookup(root, defaultLookupTestConfig(), "app/mailers/user_mailer.rb", pluralize.Default())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "user" {
		t.Fatalf("name = %q, want user", name)
	}
	if plural != "users" {
		t.Fatalf("plural = %q, want users", plural)
	}
}

func TestResolveLookup_FixturePath(t *testing.T) {
	root := t.TempDir()
	mustWriteLookupFile(t, filepath.Join(root, "app", "models", "user.rb"), "class User\nend\n")
	mustWriteLookupFile(t, filepath.Join(root, "test", "fixtures", "users.yml"), "alice:\n  name: Alice\n")

	name, plural, err := related.ResolveLookup(root, defaultLookupTestConfig(), "test/fixtures/users.yml", pluralize.Default())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "user" {
		t.Fatalf("name = %q, want user", name)
	}
	if plural != "users" {
		t.Fatalf("plural = %q, want users", plural)
	}
}

func TestResolveLookup_BareYML(t *testing.T) {
	root := t.TempDir()
	mustWriteLookupFile(t, filepath.Join(root, "app", "models", "user.rb"), "class User\nend\n")

	name, plural, err := related.ResolveLookup(root, defaultLookupTestConfig(), "users.yml", pluralize.Default())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "user" {
		t.Fatalf("name = %q, want user", name)
	}
	if plural != "users" {
		t.Fatalf("plural = %q, want users", plural)
	}
}

func TestResolveLookup_DeepNestedNamespace(t *testing.T) {
	root := t.TempDir()
	mustWriteLookupFile(t, filepath.Join(root, "app", "models", "admin", "billing", "invoice.rb"), "class Admin::Billing::Invoice\nend\n")

	name, plural, err := related.ResolveLookup(root, defaultLookupTestConfig(), "app/models/admin/billing/invoice.rb", pluralize.Default())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "admin/billing/invoice" {
		t.Fatalf("name = %q, want admin/billing/invoice", name)
	}
	if plural != "invoices" {
		t.Fatalf("plural = %q, want invoices", plural)
	}
}

func TestResolveLookup_BackslashNamespaceInput(t *testing.T) {
	root := t.TempDir()
	mustWriteLookupFile(t, filepath.Join(root, "app", "models", "admin", "dashboard.rb"), "class Admin::Dashboard\nend\n")

	name, plural, err := related.ResolveLookup(root, defaultLookupTestConfig(), `admin\dashboard`, pluralize.Default())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if filepath.ToSlash(name) != "admin/dashboard" {
		t.Fatalf("name = %q, want admin/dashboard", name)
	}
	if plural != "dashboards" {
		t.Fatalf("plural = %q, want dashboards", plural)
	}
}

func TestResolveLookup_AbsoluteFixturePath(t *testing.T) {
	root := t.TempDir()
	mustWriteLookupFile(t, filepath.Join(root, "app", "models", "user.rb"), "class User\nend\n")
	mustWriteLookupFile(t, filepath.Join(root, "test", "fixtures", "users.yml"), "alice:\n  name: Alice\n")

	absPath := filepath.Join(root, "test", "fixtures", "users.yml")
	name, plural, err := related.ResolveLookup(root, defaultLookupTestConfig(), absPath, pluralize.Default())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "user" {
		t.Fatalf("name = %q, want user", name)
	}
	if plural != "users" {
		t.Fatalf("plural = %q, want users", plural)
	}
}

func TestResolveLookup_ConfiguredSpecFixtures(t *testing.T) {
	root := t.TempDir()
	mustWriteLookupFile(t, filepath.Join(root, "app", "models", "user.rb"), "class User\nend\n")
	mustWriteLookupFile(t, filepath.Join(root, "spec", "fixtures", "users.yml"), "alice:\n  name: Alice\n")

	cfg := defaultLookupTestConfig()
	cfg.FixturesPath = "spec/fixtures"
	name, plural, err := related.ResolveLookup(root, cfg, "spec/fixtures/users.yml", pluralize.Default())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "user" {
		t.Fatalf("name = %q, want user", name)
	}
	if plural != "users" {
		t.Fatalf("plural = %q, want users", plural)
	}
}

func TestResolveLookup_SpecModelPath(t *testing.T) {
	root := t.TempDir()
	mustWriteLookupFile(t, filepath.Join(root, "app", "models", "user.rb"), "class User\nend\n")
	mustWriteLookupFile(t, filepath.Join(root, "spec", "models", "user_spec.rb"), "")

	name, plural, err := related.ResolveLookup(root, defaultLookupTestConfig(), "spec/models/user_spec.rb", pluralize.Default())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "user" {
		t.Fatalf("name = %q, want user", name)
	}
	if plural != "users" {
		t.Fatalf("plural = %q, want users", plural)
	}
}

func TestResolveLookup_AbsoluteControllerPath(t *testing.T) {
	root := t.TempDir()
	mustWriteLookupFile(t, filepath.Join(root, "app", "models", "user.rb"), "class User\nend\n")
	mustWriteLookupFile(t, filepath.Join(root, "app", "controllers", "users_controller.rb"), "class UsersController\nend\n")

	absPath := filepath.Join(root, "app", "controllers", "users_controller.rb")
	name, plural, err := related.ResolveLookup(root, defaultLookupTestConfig(), absPath, pluralize.Default())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "user" {
		t.Fatalf("name = %q, want user", name)
	}
	if plural != "users" {
		t.Fatalf("plural = %q, want users", plural)
	}
}

func TestResolveLookup_AbsoluteViewPath(t *testing.T) {
	root := t.TempDir()
	mustWriteLookupFile(t, filepath.Join(root, "app", "models", "user.rb"), "class User\nend\n")
	mustWriteLookupFile(t, filepath.Join(root, "app", "views", "users", "show.html.erb"), "")

	absPath := filepath.Join(root, "app", "views", "users", "show.html.erb")
	name, plural, err := related.ResolveLookup(root, defaultLookupTestConfig(), absPath, pluralize.Default())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "user" {
		t.Fatalf("name = %q, want user", name)
	}
	if plural != "users" {
		t.Fatalf("plural = %q, want users", plural)
	}
}

func TestResolveLookup_AbsoluteServicePath(t *testing.T) {
	root := t.TempDir()
	mustWriteLookupFile(t, filepath.Join(root, "app", "models", "user.rb"), "class User\nend\n")
	mustWriteLookupFile(t, filepath.Join(root, "app", "services", "user_export_service.rb"), "")

	absPath := filepath.Join(root, "app", "services", "user_export_service.rb")
	name, plural, err := related.ResolveLookup(root, defaultLookupTestConfig(), absPath, pluralize.Default())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "user" {
		t.Fatalf("name = %q, want user", name)
	}
	if plural != "users" {
		t.Fatalf("plural = %q, want users", plural)
	}
}

func TestResolveLookup_RequestSpecPath(t *testing.T) {
	root := t.TempDir()
	mustWriteLookupFile(t, filepath.Join(root, "app", "models", "user.rb"), "class User\nend\n")
	mustWriteLookupFile(t, filepath.Join(root, "spec", "requests", "users_spec.rb"), "")

	name, plural, err := related.ResolveLookup(root, defaultLookupTestConfig(), "spec/requests/users_spec.rb", pluralize.Default())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "user" {
		t.Fatalf("name = %q, want user", name)
	}
	if plural != "users" {
		t.Fatalf("plural = %q, want users", plural)
	}
}

func TestResolveLookup_SystemSpecPath(t *testing.T) {
	root := t.TempDir()
	mustWriteLookupFile(t, filepath.Join(root, "app", "models", "user.rb"), "class User\nend\n")
	mustWriteLookupFile(t, filepath.Join(root, "spec", "system", "users_spec.rb"), "")

	name, plural, err := related.ResolveLookup(root, defaultLookupTestConfig(), "spec/system/users_spec.rb", pluralize.Default())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "user" {
		t.Fatalf("name = %q, want user", name)
	}
	if plural != "users" {
		t.Fatalf("plural = %q, want users", plural)
	}
}

func TestResolveLookup_HelperSpecPath(t *testing.T) {
	root := t.TempDir()
	mustWriteLookupFile(t, filepath.Join(root, "app", "models", "user.rb"), "class User\nend\n")
	mustWriteLookupFile(t, filepath.Join(root, "spec", "helpers", "users_helper_spec.rb"), "")

	name, plural, err := related.ResolveLookup(root, defaultLookupTestConfig(), "spec/helpers/users_helper_spec.rb", pluralize.Default())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "user" {
		t.Fatalf("name = %q, want user", name)
	}
	if plural != "users" {
		t.Fatalf("plural = %q, want users", plural)
	}
}

func TestResolveLookup_JobSpecPath(t *testing.T) {
	root := t.TempDir()
	mustWriteLookupFile(t, filepath.Join(root, "app", "models", "user.rb"), "class User\nend\n")
	mustWriteLookupFile(t, filepath.Join(root, "spec", "jobs", "user_job_spec.rb"), "")

	name, plural, err := related.ResolveLookup(root, defaultLookupTestConfig(), "spec/jobs/user_job_spec.rb", pluralize.Default())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "user" {
		t.Fatalf("name = %q, want user", name)
	}
	if plural != "users" {
		t.Fatalf("plural = %q, want users", plural)
	}
}

func TestResolveLookup_MailerSpecPath(t *testing.T) {
	root := t.TempDir()
	mustWriteLookupFile(t, filepath.Join(root, "app", "models", "user.rb"), "class User\nend\n")
	mustWriteLookupFile(t, filepath.Join(root, "spec", "mailers", "user_mailer_spec.rb"), "")

	name, plural, err := related.ResolveLookup(root, defaultLookupTestConfig(), "spec/mailers/user_mailer_spec.rb", pluralize.Default())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "user" {
		t.Fatalf("name = %q, want user", name)
	}
	if plural != "users" {
		t.Fatalf("plural = %q, want users", plural)
	}
}

func TestResolveLookup_ServiceSpecPath(t *testing.T) {
	root := t.TempDir()
	mustWriteLookupFile(t, filepath.Join(root, "app", "models", "user.rb"), "class User\nend\n")
	mustWriteLookupFile(t, filepath.Join(root, "spec", "services", "user_export_service_spec.rb"), "")

	name, plural, err := related.ResolveLookup(root, defaultLookupTestConfig(), "spec/services/user_export_service_spec.rb", pluralize.Default())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "user" {
		t.Fatalf("name = %q, want user", name)
	}
	if plural != "users" {
		t.Fatalf("plural = %q, want users", plural)
	}
}

func mustWriteLookupFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func defaultLookupTestConfig() config.Config {
	return config.Defaults()
}
