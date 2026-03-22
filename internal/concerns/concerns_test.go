package concerns_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/janstol/rails-kit/internal/concerns"
)

const testdataRoot = "../../testdata"

func TestListFiles_ModelConcerns(t *testing.T) {
	dir := filepath.Join(testdataRoot, "app/models/concerns")
	names, err := concerns.ListFiles(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"auditable", "searchable"}
	if len(names) != len(want) {
		t.Fatalf("got %v, want %v", names, want)
	}
	for i, n := range names {
		if n != want[i] {
			t.Errorf("names[%d] = %q, want %q", i, n, want[i])
		}
	}
}

func TestListFiles_ControllerConcerns(t *testing.T) {
	dir := filepath.Join(testdataRoot, "app/controllers/concerns")
	names, err := concerns.ListFiles(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(names) != 1 || names[0] != "authenticatable" {
		t.Fatalf("got %v, want [authenticatable]", names)
	}
}

func TestListFiles_MissingDir(t *testing.T) {
	names, err := concerns.ListFiles("/nonexistent/path/to/concerns")
	if err != nil {
		t.Fatalf("missing dir should not error, got: %v", err)
	}
	if names != nil {
		t.Fatalf("missing dir should return nil, got: %v", names)
	}
}

func TestListFiles_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	names, err := concerns.ListFiles(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(names) != 0 {
		t.Fatalf("expected empty, got %v", names)
	}
}

func TestParse_Searchable(t *testing.T) {
	path := filepath.Join(testdataRoot, "app/models/concerns/searchable.rb")
	d, err := concerns.Parse(path, "app/models/concerns/searchable.rb", "model")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.Name != "Searchable" {
		t.Errorf("Name = %q, want Searchable", d.Name)
	}
	if d.Type != "model" {
		t.Errorf("Type = %q, want model", d.Type)
	}
	if !d.HasIncludedBlock {
		t.Error("HasIncludedBlock = false, want true")
	}
	if !d.HasClassMethodsBlock {
		t.Error("HasClassMethodsBlock = false, want true")
	}
	if !containsMethod(d.Methods, "search_highlight") {
		t.Errorf("Methods should contain search_highlight, got %v", d.Methods)
	}
	if !containsMethod(d.Methods, "search_excerpt") {
		t.Errorf("Methods should contain search_excerpt, got %v", d.Methods)
	}
	if !containsMethod(d.ClassMethods, "search_all") {
		t.Errorf("ClassMethods should contain search_all, got %v", d.ClassMethods)
	}
}

func TestParse_Auditable(t *testing.T) {
	path := filepath.Join(testdataRoot, "app/models/concerns/auditable.rb")
	d, err := concerns.Parse(path, "app/models/concerns/auditable.rb", "model")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.Name != "Auditable" {
		t.Errorf("Name = %q, want Auditable", d.Name)
	}
	if !d.HasIncludedBlock {
		t.Error("HasIncludedBlock = false, want true")
	}
	if d.HasClassMethodsBlock {
		t.Error("HasClassMethodsBlock = true, want false")
	}
	if !containsMethod(d.Methods, "audit_trail") {
		t.Errorf("Methods should contain audit_trail, got %v", d.Methods)
	}
	if !containsMethod(d.Methods, "last_audited_at") {
		t.Errorf("Methods should contain last_audited_at, got %v", d.Methods)
	}
	if len(d.ClassMethods) != 0 {
		t.Errorf("ClassMethods should be empty, got %v", d.ClassMethods)
	}
}

func TestParse_Authenticatable(t *testing.T) {
	path := filepath.Join(testdataRoot, "app/controllers/concerns/authenticatable.rb")
	d, err := concerns.Parse(path, "app/controllers/concerns/authenticatable.rb", "controller")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.Name != "Authenticatable" {
		t.Errorf("Name = %q, want Authenticatable", d.Name)
	}
	if d.Type != "controller" {
		t.Errorf("Type = %q, want controller", d.Type)
	}
	if !d.HasIncludedBlock {
		t.Error("HasIncludedBlock = false, want true")
	}
	if !containsMethod(d.Methods, "authenticate_user!") {
		t.Errorf("Methods should contain authenticate_user!, got %v", d.Methods)
	}
	if !containsMethod(d.Methods, "current_user") {
		t.Errorf("Methods should contain current_user, got %v", d.Methods)
	}
}

func TestFindConcern_ModelConcern(t *testing.T) {
	modelDir := filepath.Join(testdataRoot, "app/models/concerns")
	ctrlDir := filepath.Join(testdataRoot, "app/controllers/concerns")
	root := testdataRoot

	fullPath, relPath, cType, err := concerns.FindConcern(modelDir, ctrlDir, root, "searchable")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cType != "model" {
		t.Errorf("type = %q, want model", cType)
	}
	if _, statErr := os.Stat(fullPath); statErr != nil {
		t.Errorf("fullPath %q does not exist: %v", fullPath, statErr)
	}
	if relPath == "" {
		t.Error("relPath should not be empty")
	}
}

func TestFindConcern_ControllerConcern(t *testing.T) {
	modelDir := filepath.Join(testdataRoot, "app/models/concerns")
	ctrlDir := filepath.Join(testdataRoot, "app/controllers/concerns")
	root := testdataRoot

	_, _, cType, err := concerns.FindConcern(modelDir, ctrlDir, root, "authenticatable")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cType != "controller" {
		t.Errorf("type = %q, want controller", cType)
	}
}

func TestFindConcern_QualifiedModel(t *testing.T) {
	modelDir := filepath.Join(testdataRoot, "app/models/concerns")
	ctrlDir := filepath.Join(testdataRoot, "app/controllers/concerns")
	root := testdataRoot

	_, _, cType, err := concerns.FindConcern(modelDir, ctrlDir, root, "model/searchable")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cType != "model" {
		t.Errorf("type = %q, want model", cType)
	}
}

func TestFindConcern_NotFound(t *testing.T) {
	modelDir := filepath.Join(testdataRoot, "app/models/concerns")
	ctrlDir := filepath.Join(testdataRoot, "app/controllers/concerns")
	root := testdataRoot

	_, _, _, err := concerns.FindConcern(modelDir, ctrlDir, root, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent concern")
	}
}

func TestFindConcern_Ambiguous(t *testing.T) {
	dir := t.TempDir()
	modelDir := filepath.Join(dir, "models/concerns")
	ctrlDir := filepath.Join(dir, "controllers/concerns")
	if err := os.MkdirAll(modelDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(ctrlDir, 0755); err != nil {
		t.Fatal(err)
	}
	// Create same concern in both directories
	for _, d := range []string{modelDir, ctrlDir} {
		if err := os.WriteFile(filepath.Join(d, "shared.rb"), []byte("module Shared\nend\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	_, _, _, err := concerns.FindConcern(modelDir, ctrlDir, dir, "shared")
	if err == nil {
		t.Fatal("expected ambiguity error")
	}
}

func containsMethod(methods []string, name string) bool {
	for _, m := range methods {
		if m == name {
			return true
		}
	}
	return false
}
