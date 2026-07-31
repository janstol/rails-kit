// Package testutil provides cross-platform test helpers shared across
// rails-kit's test suites.
package testutil

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// WriteFakeBundle writes an executable stub named "bundle" (or "bundle.bat"
// on Windows, which has no shebang/executable-bit mechanism) into dir that
// prints stdout verbatim when invoked with any arguments, and returns the
// stub's full path. exec.LookPath honors PATHEXT, so callers that put dir on
// PATH can keep looking up the bare name "bundle" on every platform.
func WriteFakeBundle(t *testing.T, dir, stdout string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		return writeWindowsBundle(t, dir, stdout)
	}
	return writeUnixBundle(t, dir, stdout)
}

func writeUnixBundle(t *testing.T, dir, stdout string) string {
	t.Helper()
	path := filepath.Join(dir, "bundle")
	script := "#!/bin/sh\nprintf '%s' " + shellQuote(stdout) + "\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

// shellQuote wraps s in single quotes for verbatim inclusion in a POSIX shell
// script, escaping any embedded single quotes. Using printf '%s' with a
// quoted literal (rather than a heredoc) reproduces stdout byte-for-byte,
// including the absence of a trailing newline when stdout doesn't have one.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// writeWindowsBundle writes stdout verbatim to a sibling data file and has
// bundle.bat stream it with `type`, which copies bytes through unmodified.
// Building the output via `echo` per line was tried first, but cmd.exe's
// echo always terminates lines with CRLF regardless of the source content,
// which broke exact-equality comparisons against LF-only expected output.
func writeWindowsBundle(t *testing.T, dir, stdout string) string {
	t.Helper()
	dataPath := filepath.Join(dir, "bundle_stdout.dat")
	if err := os.WriteFile(dataPath, []byte(stdout), 0o644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "bundle.bat")
	script := "@echo off\r\ntype \"%~dp0bundle_stdout.dat\"\r\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

// WriteFakeBundleSleep writes an executable stub that blocks for
// approximately d before exiting, for exercising command timeouts. Returns
// the stub's full path.
func WriteFakeBundleSleep(t *testing.T, dir string, d time.Duration) string {
	t.Helper()
	seconds := int(d.Seconds())
	if seconds < 1 {
		seconds = 1
	}
	if runtime.GOOS == "windows" {
		path := filepath.Join(dir, "bundle.bat")
		// ping's first probe fires immediately, so request one extra to
		// approximate a `seconds`-long sleep.
		script := fmt.Sprintf("@echo off\r\n@ping -n %d 127.0.0.1 >nul\r\n", seconds+1)
		if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
		return path
	}
	path := filepath.Join(dir, "bundle")
	script := fmt.Sprintf("#!/bin/sh\nsleep %d\n", seconds)
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}
