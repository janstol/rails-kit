package related_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/janstol/rails-kit/internal/config"
	"github.com/janstol/rails-kit/internal/related"
)

const testdataRoot = "../../testdata"

func TestFind(t *testing.T) {
	cats, err := related.Find(testdataRoot, defaultRelatedConfig(), "user", "users")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should find Model and Fixtures at minimum
	found := map[string]bool{}
	for _, c := range cats {
		found[c.Label] = true
	}
	if !found["Model"] {
		t.Error("expected Model category")
	}
	if !found["Fixtures"] {
		t.Error("expected Fixtures category")
	}
}

func TestFindNestedModel(t *testing.T) {
	cats, err := related.Find(testdataRoot, defaultRelatedConfig(), "admin/dashboard", "dashboards")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := map[string]bool{}
	for _, c := range cats {
		found[c.Label] = true
	}
	if !found["Model"] {
		t.Error("expected Model category for nested admin/dashboard")
	}
	if !found["Fixtures"] {
		t.Error("expected Fixtures category for nested admin/dashboard")
	}
}

func TestFindNamespacedController(t *testing.T) {
	dir := t.TempDir()

	// Create controllers in two namespaces
	adminCtrl := filepath.Join(dir, "app", "controllers", "admin", "dashboards_controller.rb")
	reportsCtrl := filepath.Join(dir, "app", "controllers", "reports", "dashboards_controller.rb")
	for _, f := range []string{adminCtrl, reportsCtrl} {
		if err := os.MkdirAll(filepath.Dir(f), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(f, []byte(""), 0644); err != nil {
			t.Fatal(err)
		}
	}

	cats, err := related.Find(dir, defaultRelatedConfig(), "admin/dashboard", "dashboards")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var controllerFiles []string
	for _, c := range cats {
		if c.Label == "Controller" {
			controllerFiles = c.Files
		}
	}
	if len(controllerFiles) != 1 {
		t.Fatalf("expected 1 controller, got %v", controllerFiles)
	}
	if !containsStr(controllerFiles, "admin") {
		t.Errorf("expected admin controller, got %v", controllerFiles)
	}
	for _, f := range controllerFiles {
		if containsStr([]string{f}, "reports") {
			t.Errorf("reports controller should not appear, got %v", controllerFiles)
		}
	}
}

func TestFindRootModelExcludesNamespacedControllers(t *testing.T) {
	dir := t.TempDir()

	files := []string{
		filepath.Join(dir, "app", "models", "user.rb"),
		filepath.Join(dir, "app", "controllers", "users_controller.rb"),
		filepath.Join(dir, "app", "controllers", "admin", "users_controller.rb"),
	}
	for _, f := range files {
		if err := os.MkdirAll(filepath.Dir(f), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(f, []byte(""), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	cats, err := related.Find(dir, defaultRelatedConfig(), "user", "users")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var controllerFiles []string
	for _, c := range cats {
		if c.Label == "Controller" {
			controllerFiles = c.Files
		}
	}
	if len(controllerFiles) != 1 {
		t.Fatalf("expected 1 controller, got %v", controllerFiles)
	}
	if !containsStr(controllerFiles, "app/controllers/users_controller.rb") {
		t.Fatalf("expected root controller, got %v", controllerFiles)
	}
	if containsStr(controllerFiles, "admin/users_controller.rb") {
		t.Fatalf("did not expect admin controller, got %v", controllerFiles)
	}
}

func TestFindRootModelExcludesNamespacedViews(t *testing.T) {
	dir := t.TempDir()

	files := []string{
		filepath.Join(dir, "app", "models", "user.rb"),
		filepath.Join(dir, "app", "views", "users", "index.html.erb"),
		filepath.Join(dir, "app", "views", "admin", "users", "index.html.erb"),
	}
	for _, f := range files {
		if err := os.MkdirAll(filepath.Dir(f), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(f, []byte(""), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	cats, err := related.Find(dir, defaultRelatedConfig(), "user", "users")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var viewFiles []string
	for _, c := range cats {
		if c.Label == "Views" {
			viewFiles = c.Files
		}
	}
	if len(viewFiles) != 1 {
		t.Fatalf("expected 1 view file, got %v", viewFiles)
	}
	if !containsStr(viewFiles, "app/views/users/index.html.erb") {
		t.Fatalf("expected root views, got %v", viewFiles)
	}
	if containsStr(viewFiles, "admin/users/index.html.erb") {
		t.Fatalf("did not expect namespaced views, got %v", viewFiles)
	}
}

func TestFindNamespacedModelExcludesRootViews(t *testing.T) {
	dir := t.TempDir()

	files := []string{
		filepath.Join(dir, "app", "models", "admin", "user.rb"),
		filepath.Join(dir, "app", "views", "users", "index.html.erb"),
		filepath.Join(dir, "app", "views", "admin", "users", "index.html.erb"),
	}
	for _, f := range files {
		if err := os.MkdirAll(filepath.Dir(f), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(f, []byte(""), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	cats, err := related.Find(dir, defaultRelatedConfig(), "admin/user", "users")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var viewFiles []string
	for _, c := range cats {
		if c.Label == "Views" {
			viewFiles = c.Files
		}
	}
	if len(viewFiles) != 1 {
		t.Fatalf("expected 1 namespaced view file, got %v", viewFiles)
	}
	if !containsStr(viewFiles, "app/views/admin/users/index.html.erb") {
		t.Fatalf("expected admin views, got %v", viewFiles)
	}
	if containsStr(viewFiles, "app/views/users/index.html.erb") {
		t.Fatalf("did not expect root views, got %v", viewFiles)
	}
}

func TestFindNamespacedService(t *testing.T) {
	dir := t.TempDir()

	adminSvc := filepath.Join(dir, "app", "services", "admin", "dashboard_export_service.rb")
	otherSvc := filepath.Join(dir, "app", "services", "other", "dashboard_service.rb")
	for _, f := range []string{adminSvc, otherSvc} {
		if err := os.MkdirAll(filepath.Dir(f), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(f, []byte(""), 0644); err != nil {
			t.Fatal(err)
		}
	}

	cats, err := related.Find(dir, defaultRelatedConfig(), "admin/dashboard", "dashboards")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var serviceFiles []string
	for _, c := range cats {
		if c.Label == "Service" {
			serviceFiles = c.Files
		}
	}
	if len(serviceFiles) != 1 {
		t.Fatalf("expected 1 service, got %v", serviceFiles)
	}
	if !containsStr(serviceFiles, "admin") {
		t.Errorf("expected admin service, got %v", serviceFiles)
	}
}

func TestFindRootModelExcludesNamespacedServicesAndFormers(t *testing.T) {
	dir := t.TempDir()

	files := []string{
		filepath.Join(dir, "app", "models", "user.rb"),
		filepath.Join(dir, "app", "services", "user_export_service.rb"),
		filepath.Join(dir, "app", "services", "admin", "user_export_service.rb"),
		filepath.Join(dir, "app", "formers", "user_former.rb"),
		filepath.Join(dir, "app", "formers", "admin", "user_former.rb"),
	}
	for _, f := range files {
		if err := os.MkdirAll(filepath.Dir(f), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(f, []byte(""), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	cats, err := related.Find(dir, defaultRelatedConfig(), "user", "users")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	found := map[string][]string{}
	for _, c := range cats {
		found[c.Label] = c.Files
	}
	if !containsStr(found["Service"], "app/services/user_export_service.rb") {
		t.Fatalf("expected root service, got %v", found["Service"])
	}
	if containsStr(found["Service"], "app/services/admin/user_export_service.rb") {
		t.Fatalf("did not expect namespaced service, got %v", found["Service"])
	}
	if !containsStr(found["Former"], "app/formers/user_former.rb") {
		t.Fatalf("expected root former, got %v", found["Former"])
	}
	if containsStr(found["Former"], "app/formers/admin/user_former.rb") {
		t.Fatalf("did not expect namespaced former, got %v", found["Former"])
	}
}

func TestFindNamespacedModelExcludesDeeperNamespaceServicesAndFormers(t *testing.T) {
	dir := t.TempDir()

	files := []string{
		filepath.Join(dir, "app", "models", "admin", "user.rb"),
		filepath.Join(dir, "app", "services", "admin", "user_export_service.rb"),
		filepath.Join(dir, "app", "services", "admin", "reports", "user_export_service.rb"),
		filepath.Join(dir, "app", "formers", "admin", "user_former.rb"),
		filepath.Join(dir, "app", "formers", "admin", "reports", "user_former.rb"),
	}
	for _, f := range files {
		if err := os.MkdirAll(filepath.Dir(f), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(f, []byte(""), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	cats, err := related.Find(dir, defaultRelatedConfig(), "admin/user", "users")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	found := map[string][]string{}
	for _, c := range cats {
		found[c.Label] = c.Files
	}
	if !containsStr(found["Service"], "app/services/admin/user_export_service.rb") {
		t.Fatalf("expected exact namespace service, got %v", found["Service"])
	}
	if containsStr(found["Service"], "app/services/admin/reports/user_export_service.rb") {
		t.Fatalf("did not expect deeper namespace service, got %v", found["Service"])
	}
	if !containsStr(found["Former"], "app/formers/admin/user_former.rb") {
		t.Fatalf("expected exact namespace former, got %v", found["Former"])
	}
	if containsStr(found["Former"], "app/formers/admin/reports/user_former.rb") {
		t.Fatalf("did not expect deeper namespace former, got %v", found["Former"])
	}
}

func TestFindNamespacedDatagrid(t *testing.T) {
	dir := t.TempDir()

	datagrid := filepath.Join(dir, "app", "datagrids", "admin", "dashboards_datagrid.rb")
	if err := os.MkdirAll(filepath.Dir(datagrid), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(datagrid, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	cats, err := related.Find(dir, defaultRelatedConfig(), "admin/dashboard", "dashboards")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var datagridFiles []string
	for _, c := range cats {
		if c.Label == "Datagrid" {
			datagridFiles = c.Files
		}
	}
	if len(datagridFiles) != 1 {
		t.Fatalf("expected 1 datagrid, got %v", datagridFiles)
	}
	if !containsStr(datagridFiles, "admin/dashboards_datagrid.rb") {
		t.Fatalf("unexpected datagrid files: %v", datagridFiles)
	}
}

func containsStr(slice []string, substr string) bool {
	for _, s := range slice {
		if strings.Contains(s, substr) {
			return true
		}
	}
	return false
}

func TestFindDeepNestedService(t *testing.T) {
	dir := t.TempDir()

	svc := filepath.Join(dir, "app", "services", "admin", "billing", "invoice_export_service.rb")
	otherSvc := filepath.Join(dir, "app", "services", "other", "invoice_service.rb")
	for _, f := range []string{svc, otherSvc} {
		if err := os.MkdirAll(filepath.Dir(f), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(f, []byte(""), 0644); err != nil {
			t.Fatal(err)
		}
	}

	cats, err := related.Find(dir, defaultRelatedConfig(), "admin/billing/invoice", "invoices")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var serviceFiles []string
	for _, c := range cats {
		if c.Label == "Service" {
			serviceFiles = c.Files
		}
	}
	if len(serviceFiles) != 1 {
		t.Fatalf("expected 1 service, got %v", serviceFiles)
	}
	if !containsStr(serviceFiles, "admin/billing/invoice_export_service.rb") {
		t.Errorf("expected admin/billing service, got %v", serviceFiles)
	}
}

func TestFindSpec(t *testing.T) {
	dir := t.TempDir()

	specFiles := []string{
		filepath.Join(dir, "spec", "models", "user_spec.rb"),
		filepath.Join(dir, "spec", "controllers", "users_controller_spec.rb"),
		filepath.Join(dir, "spec", "requests", "users_spec.rb"),
		filepath.Join(dir, "spec", "system", "users_spec.rb"),
		filepath.Join(dir, "spec", "helpers", "users_helper_spec.rb"),
		filepath.Join(dir, "spec", "jobs", "user_job_spec.rb"),
		filepath.Join(dir, "spec", "mailers", "user_mailer_spec.rb"),
		filepath.Join(dir, "spec", "services", "user_service_spec.rb"),
	}
	for _, f := range specFiles {
		if err := os.MkdirAll(filepath.Dir(f), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(f, []byte(""), 0644); err != nil {
			t.Fatal(err)
		}
	}

	cats, err := related.Find(dir, defaultRelatedConfig(), "user", "users")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	found := map[string]bool{}
	for _, c := range cats {
		found[c.Label] = true
	}
	for _, label := range []string{"Model spec", "Controller spec", "Request spec", "System spec", "Helper spec", "Job spec", "Mailer spec", "Service spec"} {
		if !found[label] {
			t.Errorf("expected %q category", label)
		}
	}
}

func TestWalkMatchSegment(t *testing.T) {
	dir := t.TempDir()

	files := []string{
		"user_service.rb",
		"superuser_service.rb",
		"abuser_service.rb",
		"user_export_service.rb",
		"super_user_service.rb",
	}
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(dir, f), []byte(""), 0644); err != nil {
			t.Fatal(err)
		}
	}

	matches, err := related.WalkMatchSegment(dir, "user")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	found := map[string]bool{}
	for _, m := range matches {
		found[filepath.Base(m)] = true
	}

	if !found["user_service.rb"] {
		t.Error("expected user_service.rb to match")
	}
	if !found["user_export_service.rb"] {
		t.Error("expected user_export_service.rb to match")
	}
	if found["superuser_service.rb"] {
		t.Error("superuser_service.rb should not match for 'user'")
	}
	if found["abuser_service.rb"] {
		t.Error("abuser_service.rb should not match for 'user'")
	}
	if found["super_user_service.rb"] {
		t.Error("super_user_service.rb should not match for 'user'")
	}
}

func TestFindAbsoluteConfiguredPaths(t *testing.T) {
	root := t.TempDir()
	modelsDir := filepath.Join(t.TempDir(), "models")
	fixturesDir := filepath.Join(t.TempDir(), "fixtures")

	modelFile := filepath.Join(modelsDir, "user.rb")
	fixtureFile := filepath.Join(fixturesDir, "users.yml")
	for _, path := range []string{modelFile, fixtureFile} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(modelFile, []byte("class User\nend\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fixtureFile, []byte("alice:\n  name: Alice\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := defaultRelatedConfig()
	cfg.ModelsPath = modelsDir
	cfg.FixturesPath = fixturesDir
	cats, err := related.Find(root, cfg, "user", "users")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	found := map[string][]string{}
	for _, cat := range cats {
		found[cat.Label] = cat.Files
	}
	if !containsStr(found["Model"], filepath.ToSlash(modelFile)) {
		t.Fatalf("expected model path in %v", found["Model"])
	}
	if !containsStr(found["Fixtures"], filepath.ToSlash(fixtureFile)) {
		t.Fatalf("expected fixture path in %v", found["Fixtures"])
	}
}

func TestWalkMatchSegmentDirectoryMatch(t *testing.T) {
	dir := t.TempDir()

	// Files in model-named subdirectory should match.
	userDir := filepath.Join(dir, "user")
	superuserDir := filepath.Join(dir, "superuser")
	for _, d := range []string{userDir, superuserDir} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatal(err)
		}
	}
	files := map[string]string{
		filepath.Join(userDir, "export_service.rb"):      "match",
		filepath.Join(userDir, "import_service.rb"):      "match",
		filepath.Join(superuserDir, "export_service.rb"): "no-match",
	}
	for f := range files {
		if err := os.WriteFile(f, []byte(""), 0644); err != nil {
			t.Fatal(err)
		}
	}

	matches, err := related.WalkMatchSegment(dir, "user")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	found := map[string]bool{}
	for _, m := range matches {
		found[m] = true
	}

	if !found[filepath.Join(userDir, "export_service.rb")] {
		t.Error("expected user/export_service.rb to match")
	}
	if !found[filepath.Join(userDir, "import_service.rb")] {
		t.Error("expected user/import_service.rb to match")
	}
	if found[filepath.Join(superuserDir, "export_service.rb")] {
		t.Error("superuser/export_service.rb should not match for 'user'")
	}
}

func TestFindServiceInModelNamedSubdir(t *testing.T) {
	dir := t.TempDir()

	files := []string{
		filepath.Join(dir, "app", "models", "user.rb"),
		filepath.Join(dir, "app", "services", "user", "export_service.rb"),
		filepath.Join(dir, "app", "services", "user_export_service.rb"),
	}
	for _, f := range files {
		if err := os.MkdirAll(filepath.Dir(f), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(f, []byte(""), 0644); err != nil {
			t.Fatal(err)
		}
	}

	cats, err := related.Find(dir, defaultRelatedConfig(), "user", "users")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var serviceFiles []string
	for _, c := range cats {
		if c.Label == "Service" {
			serviceFiles = c.Files
		}
	}
	if len(serviceFiles) != 2 {
		t.Fatalf("expected 2 services (flat + subdir), got %v", serviceFiles)
	}
	if !containsStr(serviceFiles, "app/services/user/export_service.rb") {
		t.Errorf("expected user/export_service.rb in services, got %v", serviceFiles)
	}
	if !containsStr(serviceFiles, "app/services/user_export_service.rb") {
		t.Errorf("expected user_export_service.rb in services, got %v", serviceFiles)
	}
}

func TestFindNamespacedServiceExcludesCompoundNames(t *testing.T) {
	dir := t.TempDir()

	files := []string{
		filepath.Join(dir, "app", "models", "admin", "user.rb"),
		filepath.Join(dir, "app", "services", "admin", "super_user_service.rb"),
		filepath.Join(dir, "app", "services", "admin", "user_export_service.rb"),
	}
	for _, f := range files {
		if err := os.MkdirAll(filepath.Dir(f), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(f, []byte(""), 0644); err != nil {
			t.Fatal(err)
		}
	}

	cats, err := related.Find(dir, defaultRelatedConfig(), "admin/user", "admin/users")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var serviceFiles []string
	for _, c := range cats {
		if c.Label == "Service" {
			serviceFiles = c.Files
		}
	}
	if containsStr(serviceFiles, "app/services/admin/super_user_service.rb") {
		t.Errorf("super_user_service.rb should not match for admin/user, got %v", serviceFiles)
	}
	if !containsStr(serviceFiles, "app/services/admin/user_export_service.rb") {
		t.Errorf("expected user_export_service.rb in services, got %v", serviceFiles)
	}
}

func TestWalkMatchSegmentExcludesNonRuby(t *testing.T) {
	dir := t.TempDir()

	files := []string{
		"user_service.rb",
		"user_service.rbs",
		"user_service.txt",
		"user_service_test.rb",
	}
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(dir, f), []byte(""), 0644); err != nil {
			t.Fatal(err)
		}
	}

	matches, err := related.WalkMatchSegment(dir, "user")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	found := map[string]bool{}
	for _, m := range matches {
		found[filepath.Base(m)] = true
	}

	if !found["user_service.rb"] {
		t.Error("expected user_service.rb to match")
	}
	if !found["user_service_test.rb"] {
		t.Error("expected user_service_test.rb to match")
	}
	if found["user_service.rbs"] {
		t.Error("user_service.rbs should not match")
	}
	if found["user_service.txt"] {
		t.Error("user_service.txt should not match")
	}
}

func TestWalkMatchNS_UnreadableSubdir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod has no read-blocking semantics on Windows")
	}
	if os.Getuid() == 0 {
		t.Skip("cannot test permission errors as root")
	}
	dir := t.TempDir()
	// Create app/controllers with a locked subdirectory so walkMatchNS hits it.
	controllersDir := filepath.Join(dir, "app", "controllers")
	locked := filepath.Join(controllersDir, "locked")
	if err := os.MkdirAll(locked, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(locked, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })

	_, err := related.Find(dir, defaultRelatedConfig(), "user", "users")
	if err == nil {
		t.Error("expected error for unreadable directory, got nil")
	}
}

func TestNormalizeName(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"user", "user"},
		{"user.rb", "user"},
		{"app/models/user.rb", "user"},
		{"users_controller.rb", "users"},
		{"users_controller_test.rb", "users"},
		{"user_test.rb", "user"},
		{"user_decorator.rb", "user"},
		{"users_datagrid.rb", "users"},
		// Single-level namespace is preserved
		{"admin/user.rb", "admin/user"},
		{"admin/user_controller.rb", "admin/user"},
		// New suffixes
		{"user_former.rb", "user"},
		{"user_service.rb", "user"},
		{"user_job.rb", "user"},
		{"user_mailer.rb", "user"},
		{"app/jobs/user_job.rb", "user"},
		{"app/mailers/user_mailer.rb", "user"},
		// RSpec suffixes
		{"user_spec.rb", "user"},
		{"spec/models/user_spec.rb", "user"},
		{"spec/controllers/users_controller_spec.rb", "users"},
		{"spec/requests/users_spec.rb", "users"},
		{"spec/system/users_spec.rb", "users"},
		{"spec/helpers/users_helper_spec.rb", "users"},
		{"spec/jobs/user_job_spec.rb", "user"},
		{"spec/mailers/user_mailer_spec.rb", "user"},
		{"spec/services/user_service_spec.rb", "user"},
		// yaml extension
		{"users.yaml", "users"},
		// Multi-level namespace is preserved
		{"admin/billing/invoice.rb", "admin/billing/invoice"},
		{"app/models/admin/billing/invoice.rb", "admin/billing/invoice"},
		{"app/controllers/admin/billing/invoices_controller.rb", "admin/billing/invoices"},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			got := related.NormalizeName(tc.in)
			if got != tc.want {
				t.Errorf("NormalizeName(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestFindJob(t *testing.T) {
	dir := t.TempDir()

	for _, f := range []string{
		filepath.Join(dir, "app", "models", "user.rb"),
		filepath.Join(dir, "app", "jobs", "user_job.rb"),
	} {
		if err := os.MkdirAll(filepath.Dir(f), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(f, []byte(""), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	cats, err := related.Find(dir, defaultRelatedConfig(), "user", "users")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var jobFiles []string
	for _, c := range cats {
		if c.Label == "Job" {
			jobFiles = c.Files
		}
	}
	if len(jobFiles) != 1 {
		t.Fatalf("expected 1 job file, got %v", jobFiles)
	}
	if !containsStr(jobFiles, "app/jobs/user_job.rb") {
		t.Errorf("expected user job, got %v", jobFiles)
	}
}

func TestFindMailer(t *testing.T) {
	dir := t.TempDir()

	for _, f := range []string{
		filepath.Join(dir, "app", "models", "user.rb"),
		filepath.Join(dir, "app", "mailers", "user_mailer.rb"),
	} {
		if err := os.MkdirAll(filepath.Dir(f), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(f, []byte(""), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	cats, err := related.Find(dir, defaultRelatedConfig(), "user", "users")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var mailerFiles []string
	for _, c := range cats {
		if c.Label == "Mailer" {
			mailerFiles = c.Files
		}
	}
	if len(mailerFiles) != 1 {
		t.Fatalf("expected 1 mailer file, got %v", mailerFiles)
	}
	if !containsStr(mailerFiles, "app/mailers/user_mailer.rb") {
		t.Errorf("expected user mailer, got %v", mailerFiles)
	}
}

func TestFindJobNamespaceIsolation(t *testing.T) {
	dir := t.TempDir()

	for _, f := range []string{
		filepath.Join(dir, "app", "models", "user.rb"),
		filepath.Join(dir, "app", "jobs", "user_job.rb"),
		filepath.Join(dir, "app", "jobs", "admin", "user_job.rb"),
	} {
		if err := os.MkdirAll(filepath.Dir(f), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(f, []byte(""), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	cats, err := related.Find(dir, defaultRelatedConfig(), "user", "users")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var jobFiles []string
	for _, c := range cats {
		if c.Label == "Job" {
			jobFiles = c.Files
		}
	}
	if len(jobFiles) != 1 {
		t.Fatalf("expected only root job, got %v", jobFiles)
	}
	if containsStr(jobFiles, "admin") {
		t.Errorf("did not expect namespaced job, got %v", jobFiles)
	}
}

func defaultRelatedConfig() config.Config {
	return config.Defaults()
}
