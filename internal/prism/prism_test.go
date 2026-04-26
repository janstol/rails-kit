package prism_test

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/janstol/rails-kit/internal/prism"
)

func TestParseFilesExtractsRubySkeleton(t *testing.T) {
	requirePrism(t)

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
	requirePrism(t)
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

func TestIsUnavailableForMissingRuby(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, err := prism.Runner{Ruby: "rails-kit-ruby-that-does-not-exist"}.ParseFiles(ctx, []string{"x.rb"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !prism.IsUnavailable(err) {
		t.Fatalf("expected unavailable error, got: %v", err)
	}
	var execErr *exec.Error
	if !errors.As(err, &execErr) && !strings.Contains(err.Error(), "executable file not found") {
		t.Fatalf("unexpected error type: %T %v", err, err)
	}
}

func TestParseFilesFallsBackToInteractiveShell(t *testing.T) {
	root := t.TempDir()
	bin := filepath.Join(root, "bin")
	marker := filepath.Join(root, "shell-used")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatalf("mkdir fake bin: %v", err)
	}
	writeExecutable(t, filepath.Join(bin, "shell"), `#!/bin/sh
if [ "$1" = "-ic" ]; then
  shift
  RAILS_KIT_SHELL_FALLBACK=1 /bin/sh -c "$1"
  exit $?
fi
exit 1
`)
	writeExecutable(t, filepath.Join(bin, "ruby"), `#!/bin/sh
if [ "$RAILS_KIT_SHELL_FALLBACK" = "1" ]; then
  cat >/dev/null
  echo yes > "$RAILS_KIT_MARKER"
  printf '{"files":[{"path":"fake.rb","classes":[{"name":"Fake","start_line":1,"end_line":1}]}]}'
  exit 0
fi
printf 'cannot load such file -- prism\n' >&2
exit 1
`)
	t.Setenv("PATH", bin+":/bin:/usr/bin")
	t.Setenv("SHELL", filepath.Join(bin, "shell"))
	t.Setenv("RAILS_KIT_MARKER", marker)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	files, err := prism.Runner{Dir: root}.ParseFiles(ctx, []string{"fake.rb"})
	if err != nil {
		t.Fatalf("ParseFiles error: %v", err)
	}
	if len(files) != 1 || len(files[0].Classes) != 1 || files[0].Classes[0].Name != "Fake" {
		t.Fatalf("unexpected files: %#v", files)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("shell fallback was not used: %v", err)
	}
}

func writeExecutable(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatalf("write executable %s: %v", path, err)
	}
}

func requirePrism(t *testing.T) {
	t.Helper()
	if err := exec.Command("ruby", "-rprism", "-e", "exit").Run(); err != nil {
		t.Skipf("ruby with prism is not available: %v", err)
	}
}
