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

func TestParseFilesEmptyInputDoesNotStartRuby(t *testing.T) {
	files, err := prism.Runner{Ruby: "rails-kit-ruby-that-does-not-exist"}.ParseFiles(context.Background(), nil)
	if err != nil {
		t.Fatalf("ParseFiles error: %v", err)
	}
	if files != nil {
		t.Fatalf("files = %#v, want nil", files)
	}
}

func TestParseFilesDirectSuccessDoesNotUseShell(t *testing.T) {
	root, bin := fakeBin(t)
	shellMarker := filepath.Join(root, "shell-used")
	writeExecutable(t, filepath.Join(bin, "ruby"), `#!/bin/sh
cat >/dev/null
printf '{"files":[{"path":"fake.rb"}]}'
`)
	writeExecutable(t, filepath.Join(bin, "shell"), `#!/bin/sh
echo yes > "$RAILS_KIT_SHELL_MARKER"
exit 1
`)
	t.Setenv("PATH", bin+":/bin:/usr/bin")
	t.Setenv("SHELL", filepath.Join(bin, "shell"))
	t.Setenv("RAILS_KIT_SHELL_MARKER", shellMarker)

	files, err := prism.Runner{Dir: root}.ParseFiles(context.Background(), []string{"fake.rb"})
	if err != nil {
		t.Fatalf("ParseFiles error: %v", err)
	}
	if len(files) != 1 || files[0].Path != "fake.rb" {
		t.Fatalf("unexpected files: %#v", files)
	}
	if _, err := os.Stat(shellMarker); !os.IsNotExist(err) {
		t.Fatalf("shell should not have been used, stat error = %v", err)
	}
}

func TestParseFilesExplicitRubyDoesNotFallBack(t *testing.T) {
	root, bin := fakeBin(t)
	shellMarker := filepath.Join(root, "shell-used")
	rubyPath := filepath.Join(bin, "custom-ruby")
	writeExecutable(t, rubyPath, `#!/bin/sh
printf 'cannot load such file -- prism\n' >&2
exit 1
`)
	writeExecutable(t, filepath.Join(bin, "shell"), `#!/bin/sh
echo yes > "$RAILS_KIT_SHELL_MARKER"
exit 1
`)
	t.Setenv("SHELL", filepath.Join(bin, "shell"))
	t.Setenv("RAILS_KIT_SHELL_MARKER", shellMarker)

	_, err := prism.Runner{Ruby: rubyPath, Dir: root}.ParseFiles(context.Background(), []string{"fake.rb"})
	if err == nil {
		t.Fatal("expected error")
	}
	if _, statErr := os.Stat(shellMarker); !os.IsNotExist(statErr) {
		t.Fatalf("explicit Ruby unexpectedly fell back to shell, stat error = %v", statErr)
	}
}

func TestParseFilesFallsBackToInteractiveShell(t *testing.T) {
	root, bin := fakeBin(t)
	marker := filepath.Join(root, "shell-used")
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

func TestParseFilesUsesDefaultShellWhenShellIsUnset(t *testing.T) {
	root, bin := fakeBin(t)
	writeExecutable(t, filepath.Join(bin, "ruby"), `#!/bin/sh
if [ -n "$RAILS_KIT_PRISM_HELPER" ]; then
  cat >/dev/null
  printf '{"files":[{"path":"fake.rb"}]}'
  exit 0
fi
printf 'cannot load such file -- prism\n' >&2
exit 1
`)
	t.Setenv("PATH", bin+":/bin:/usr/bin")
	t.Setenv("SHELL", "")

	files, err := prism.Runner{Dir: root}.ParseFiles(context.Background(), []string{"fake.rb"})
	if err != nil {
		t.Fatalf("ParseFiles error: %v", err)
	}
	if len(files) != 1 || files[0].Path != "fake.rb" {
		t.Fatalf("unexpected files: %#v", files)
	}
}

func TestParseFilesReturnsShellFallbackFailure(t *testing.T) {
	root, bin := fakeBin(t)
	writeExecutable(t, filepath.Join(bin, "ruby"), `#!/bin/sh
printf 'cannot load such file -- prism\n' >&2
exit 1
`)
	writeExecutable(t, filepath.Join(bin, "shell"), `#!/bin/sh
printf 'fallback shell failed\n' >&2
exit 23
`)
	t.Setenv("PATH", bin+":/bin:/usr/bin")
	t.Setenv("SHELL", filepath.Join(bin, "shell"))

	_, err := prism.Runner{Dir: root}.ParseFiles(context.Background(), []string{"fake.rb"})
	if err == nil || !strings.Contains(err.Error(), "fallback shell failed") {
		t.Fatalf("unexpected fallback error: %v", err)
	}
}

func TestParseFilesRejectsInvalidOutput(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   string
	}{
		{name: "invalid JSON", output: "not-json", want: "decoding prism output"},
		{name: "wrong file count", output: `{"files":[]}`, want: "prism returned 0 file(s), expected 1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root, bin := fakeBin(t)
			rubyPath := filepath.Join(bin, "ruby")
			writeExecutable(t, rubyPath, "#!/bin/sh\ncat >/dev/null\nprintf '%s' '"+tt.output+"'\n")

			_, err := prism.Runner{Ruby: rubyPath, Dir: root}.ParseFiles(context.Background(), []string{"fake.rb"})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want containing %q", err, tt.want)
			}
		})
	}
}

func TestParseFilesHonorsContextTimeout(t *testing.T) {
	root, bin := fakeBin(t)
	rubyPath := filepath.Join(bin, "ruby")
	writeExecutable(t, rubyPath, `#!/bin/sh
exec sleep 5
`)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := prism.Runner{Ruby: rubyPath, Dir: root}.ParseFiles(ctx, []string{"fake.rb"})
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("child process was not terminated promptly; elapsed = %v", elapsed)
	}
}

func fakeBin(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	bin := filepath.Join(root, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatalf("mkdir fake bin: %v", err)
	}
	return root, bin
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
