package cmd

import (
	"path/filepath"
	"strings"
	"testing"
)

func setColorFlag(t *testing.T, value string) {
	t.Helper()
	prev := colorFlag
	colorFlag = value
	t.Cleanup(func() {
		colorFlag = prev
	})
}

func TestColorFlag_AlwaysAddsEscapes(t *testing.T) {
	fixtureRoot, err := filepath.Abs("../testdata")
	if err != nil {
		t.Fatal(err)
	}
	setColorFlag(t, "always")
	t.Setenv("NO_COLOR", "")

	out, errOut, err := runCmdForTest(t, schemaCmd, fixtureRoot, []string{"users"})
	if err != nil {
		t.Fatalf("unexpected error: %v\nstderr:%s", err, errOut)
	}
	if !strings.Contains(out, "\x1b[") {
		t.Errorf("expected ANSI escapes in `schema users --color=always` output:\n%s", out)
	}

	out, errOut, err = runCmdForTest(t, modelCmd, fixtureRoot, []string{"user"})
	if err != nil {
		t.Fatalf("unexpected error: %v\nstderr:%s", err, errOut)
	}
	if !strings.Contains(out, "\x1b[") {
		t.Errorf("expected ANSI escapes in `model user --color=always` output:\n%s", out)
	}
}

func TestColorFlag_NeverStripsEscapes(t *testing.T) {
	fixtureRoot, err := filepath.Abs("../testdata")
	if err != nil {
		t.Fatal(err)
	}
	setColorFlag(t, "never")
	t.Setenv("NO_COLOR", "")

	out, errOut, err := runCmdForTest(t, schemaCmd, fixtureRoot, []string{"users"})
	if err != nil {
		t.Fatalf("unexpected error: %v\nstderr:%s", err, errOut)
	}
	if strings.Contains(out, "\x1b[") {
		t.Errorf("expected no ANSI escapes in `schema users --color=never` output:\n%s", out)
	}

	out, errOut, err = runCmdForTest(t, modelCmd, fixtureRoot, []string{"user"})
	if err != nil {
		t.Fatalf("unexpected error: %v\nstderr:%s", err, errOut)
	}
	if strings.Contains(out, "\x1b[") {
		t.Errorf("expected no ANSI escapes in `model user --color=never` output:\n%s", out)
	}
}

func TestColorFlag_AutoOverPipeStripsEscapes(t *testing.T) {
	fixtureRoot, err := filepath.Abs("../testdata")
	if err != nil {
		t.Fatal(err)
	}
	setColorFlag(t, "auto")
	t.Setenv("NO_COLOR", "")

	// runCmdForTest always swaps os.Stdout for a pipe, so auto must resolve
	// to disabled here — this is the invariant that keeps golden files
	// uncolored without any test-harness changes.
	out, errOut, err := runCmdForTest(t, schemaCmd, fixtureRoot, []string{"users"})
	if err != nil {
		t.Fatalf("unexpected error: %v\nstderr:%s", err, errOut)
	}
	if strings.Contains(out, "\x1b[") {
		t.Errorf("expected no ANSI escapes in `schema users --color=auto` piped output:\n%s", out)
	}

	out, errOut, err = runCmdForTest(t, modelCmd, fixtureRoot, []string{"user"})
	if err != nil {
		t.Fatalf("unexpected error: %v\nstderr:%s", err, errOut)
	}
	if strings.Contains(out, "\x1b[") {
		t.Errorf("expected no ANSI escapes in `model user --color=auto` piped output:\n%s", out)
	}
}

func TestColorFlag_JSONNeverColored(t *testing.T) {
	fixtureRoot, err := filepath.Abs("../testdata")
	if err != nil {
		t.Fatal(err)
	}
	setColorFlag(t, "always")
	t.Setenv("NO_COLOR", "")

	out, errOut, err := runCmdForTestJSON(t, schemaCmd, fixtureRoot, []string{"users"})
	if err != nil {
		t.Fatalf("unexpected error: %v\nstderr:%s", err, errOut)
	}
	if strings.Contains(out, "\x1b[") {
		t.Errorf("expected no ANSI escapes in `--json schema users --color=always` output:\n%s", out)
	}

	out, errOut, err = runCmdForTestJSON(t, modelCmd, fixtureRoot, []string{"user"})
	if err != nil {
		t.Fatalf("unexpected error: %v\nstderr:%s", err, errOut)
	}
	if strings.Contains(out, "\x1b[") {
		t.Errorf("expected no ANSI escapes in `--json model user --color=always` output:\n%s", out)
	}
}

func TestColorFlag_InvalidValueRejected(t *testing.T) {
	setColorFlag(t, "bogus")

	if err := rootCmd.PersistentPreRunE(rootCmd, nil); err == nil {
		t.Fatal("expected an error for --color=bogus")
	}
}
