package prism_test

import (
	"context"
	"fmt"
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

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
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

// TestParseFilesResultsAreInInputOrder guards the fan-out invariant that
// results are written to a preallocated slice by input index, not appended
// as goroutines complete. Every file has a distinct class name so a mixed-up
// index would surface as a mismatched name rather than passing by accident.
func TestParseFilesResultsAreInInputOrder(t *testing.T) {
	const n = 24
	dir := t.TempDir()
	paths := make([]string, n)
	for i := range n {
		path := filepath.Join(dir, fmt.Sprintf("model_%d.rb", i))
		src := fmt.Sprintf("class Model%d\nend\n", i)
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatalf("write fixture %d: %v", i, err)
		}
		paths[i] = path
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	files, err := prism.Runner{}.ParseFiles(ctx, paths)
	if err != nil {
		t.Fatalf("ParseFiles error: %v", err)
	}
	if len(files) != n {
		t.Fatalf("files len = %d, want %d", len(files), n)
	}
	for i, f := range files {
		want := fmt.Sprintf("Model%d", i)
		if len(f.Classes) != 1 || f.Classes[0].Name != want {
			t.Fatalf("file %d: classes = %#v, want single class %q", i, f.Classes, want)
		}
	}
}

// TestParseFilesErrorIsLowestFailingIndexStable guards the invariant that the
// returned error is chosen by input index, not by which failing goroutine
// happens to finish first. Run repeatedly since a completion-order bug would
// only surface intermittently.
func TestParseFilesErrorIsLowestFailingIndexStable(t *testing.T) {
	dir := t.TempDir()
	goodPath := filepath.Join("..", "..", "testdata", "app", "models", "user.rb")
	if _, err := os.Stat(goodPath); err != nil {
		t.Skipf("fixture not found: %v", err)
	}

	paths := []string{
		goodPath,
		goodPath,
		filepath.Join(dir, "missing_low.rb"),
		goodPath,
		goodPath,
		filepath.Join(dir, "missing_high.rb"),
		goodPath,
	}

	for attempt := range 20 {
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		_, err := prism.Runner{}.ParseFiles(ctx, paths)
		cancel()
		if err == nil {
			t.Fatalf("attempt %d: expected error", attempt)
		}
		if !strings.Contains(err.Error(), "missing_low.rb") {
			t.Fatalf("attempt %d: error = %v, want it to name missing_low.rb (lowest failing index)", attempt, err)
		}
		if strings.Contains(err.Error(), "missing_high.rb") {
			t.Fatalf("attempt %d: error = %v, must not name missing_high.rb (higher failing index)", attempt, err)
		}
	}
}

// TestParseFilesCancelledContextReturnsPromptly guards against paying a cold
// start per remaining file once the context is already done: a regression
// here would make a timed-out skeleton call take as long as the full serial
// cost instead of failing fast.
func TestParseFilesCancelledContextReturnsPromptly(t *testing.T) {
	goodPath := filepath.Join("..", "..", "testdata", "app", "models", "user.rb")
	if _, err := os.Stat(goodPath); err != nil {
		t.Skipf("fixture not found: %v", err)
	}

	paths := make([]string, 40)
	for i := range paths {
		paths[i] = goodPath
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	start := time.Now()
	_, err := prism.Runner{}.ParseFiles(ctx, paths)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
	// A single cold start alone costs ~150ms; paying it for all 40 queued
	// files would take several seconds. 1s leaves generous headroom while
	// still catching a regression to serial cold-start-per-file behavior.
	if elapsed > time.Second {
		t.Fatalf("ParseFiles took %v with an already-cancelled context, want well under 1s", elapsed)
	}
}

// TestParseFilesStressRepeatedBatchesNoTraps repeatedly parses every real
// fixture under testdata/app in one batch, guarding against reintroducing
// any shared state across the concurrent parser instances: the parser-reuse
// bug that motivated one-parser-per-file trips WASM "out of bounds memory
// access" traps intermittently, so a single clean run would not be enough
// to catch a concurrency-safety regression. Run under -race for the primary
// safety signal.
func TestParseFilesStressRepeatedBatchesNoTraps(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in -short mode")
	}

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

	for attempt := range 20 {
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		files, err := prism.Runner{}.ParseFiles(ctx, paths)
		cancel()
		if err != nil {
			t.Fatalf("attempt %d: ParseFiles error: %v", attempt, err)
		}
		if len(files) != len(paths) {
			t.Fatalf("attempt %d: files len = %d, want %d", attempt, len(files), len(paths))
		}
		for i, f := range files {
			if len(f.ParseErrors) != 0 {
				t.Fatalf("attempt %d: file %s: unexpected parse errors: %v", attempt, paths[i], f.ParseErrors)
			}
		}
	}
}
