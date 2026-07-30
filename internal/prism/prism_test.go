package prism_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/janstol/rails-kit/internal/prism"
)

func TestParseFilesExtractsRubySkeleton(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	files, err := prism.Runner{}.ParseFiles(ctx, []string{filepath.Join("..", "..", "testdata", "app", "services", "user_export_service.rb")})
	if err != nil {
		t.Fatalf("ParseFiles error: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("files len = %d, want 1", len(files))
	}
	file := files[0]
	if len(file.Classes) != 1 || file.Classes[0].Name != "UserExportService" {
		t.Fatalf("unexpected classes: %#v", file.Classes)
	}
	class := file.Classes[0]
	if len(class.Constants) != 1 || class.Constants[0].Name != "DEFAULT_LIMIT" {
		t.Fatalf("unexpected constants: %#v", class.Constants)
	}
	if len(class.Includes) != 1 || class.Includes[0].Source != "include Searchable" {
		t.Fatalf("unexpected includes: %#v", class.Includes)
	}
	if len(class.Methods) != 3 {
		t.Fatalf("methods len = %d, want 3: %#v", len(class.Methods), class.Methods)
	}
	if class.Methods[2].Name != "export" || class.Methods[2].Visibility != "private" {
		t.Fatalf("private method not detected: %#v", class.Methods[2])
	}
}

func TestParseFilesSurfacesParseErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "broken.rb")
	if err := os.WriteFile(path, []byte("class Broken\n  def call(\nend\n"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	files, err := prism.Runner{}.ParseFiles(ctx, []string{path})
	if err != nil {
		t.Fatalf("ParseFiles error: %v", err)
	}
	if len(files) != 1 || len(files[0].ParseErrors) == 0 {
		t.Fatalf("expected parse errors, got: %#v", files)
	}
}

func TestParseFilesEmptyInputReturnsNil(t *testing.T) {
	files, err := prism.Runner{}.ParseFiles(context.Background(), nil)
	if err != nil {
		t.Fatalf("ParseFiles error: %v", err)
	}
	if files != nil {
		t.Fatalf("files = %#v, want nil", files)
	}
}

// TestParseFilesBatchOfRealFilesAllSucceed guards against reintroducing a
// shared Prism parser instance across a batch: reusing one go-ruby-prism
// v1.1.0 Parser across several real files intermittently trips "out of
// bounds memory access" WASM traps on otherwise-valid input. Parsing every
// fixture under testdata/app in one batch reproduced that ~50% of the time
// when the Runner reused one parser; it must be 100% reliable now that each
// file gets its own parser instance.
func TestParseFilesBatchOfRealFilesAllSucceed(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "app")
	var paths []string
	if err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, ".rb") {
			paths = append(paths, path)
		}
		return nil
	}); err != nil {
		t.Fatalf("walking %s: %v", root, err)
	}
	if len(paths) < 2 {
		t.Fatalf("expected multiple fixture files under %s, found %d", root, len(paths))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	files, err := prism.Runner{}.ParseFiles(ctx, paths)
	if err != nil {
		t.Fatalf("ParseFiles error: %v", err)
	}
	if len(files) != len(paths) {
		t.Fatalf("files len = %d, want %d", len(files), len(paths))
	}
	for i, f := range files {
		if len(f.ParseErrors) != 0 {
			t.Errorf("file %s: unexpected parse errors: %v", paths[i], f.ParseErrors)
		}
	}
}

func TestParseFilesReadErrorForMissingFile(t *testing.T) {
	_, err := prism.Runner{}.ParseFiles(context.Background(), []string{filepath.Join(t.TempDir(), "missing.rb")})
	if err == nil {
		t.Fatal("expected error")
	}
}
