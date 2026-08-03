package cmd

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
)

// TestJSONContractInvariants guards the two invariants that make the --json
// contract simple for consumers: data always decodes as a JSON object, and
// the envelope's schema_version/command fields are always present and
// correct. It exercises both list and detail modes for every JSON-emitting
// command against the shared fixture Rails root under ../testdata.
func TestJSONContractInvariants(t *testing.T) {
	fixtureRoot, err := filepath.Abs("../testdata")
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name  string
		cmd   *cobra.Command
		args  []string
		flags map[string]string
	}{
		{name: "about", cmd: aboutCmd},
		{name: "version", cmd: versionCmd},
		{name: "schema_list", cmd: schemaCmd},
		{name: "schema_extract", cmd: schemaCmd, args: []string{"users"}},
		{name: "routes_static", cmd: routesCmd, flags: map[string]string{"static": "true"}},
		{name: "model", cmd: modelCmd, args: []string{"user"}},
		{name: "related", cmd: relatedCmd, args: []string{"user"}},
		{name: "locales_list", cmd: localesCmd},
		{name: "locales_scope", cmd: localesCmd, args: []string{"en.views.users"}},
		{name: "gem_list", cmd: gemCmd},
		{name: "gem_detail", cmd: gemCmd, args: []string{"rails"}},
		{name: "concerns_list", cmd: concernsCmd},
		{name: "concerns_detail", cmd: concernsCmd, args: []string{"model/searchable"}},
		{name: "fixtures_list", cmd: fixturesCmd},
		{name: "fixtures_detail", cmd: fixturesCmd, args: []string{"users"}},
		{name: "skeleton", cmd: skeletonCmd, args: []string{"app/models/user.rb"}},
		{name: "controllers_list", cmd: controllersCmd},
		{name: "controllers_detail", cmd: controllersCmd, args: []string{"users"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			applyGoldenFlags(t, tc.cmd, tc.flags)

			out, errOut, err := runCmdForTestJSON(t, tc.cmd, fixtureRoot, tc.args)
			if err != nil {
				t.Fatalf("unexpected error: %v\nstderr:%s", err, errOut)
			}

			var data map[string]any
			unwrapJSONEnvelope(t, out, tc.cmd.Name(), &data)
		})
	}
}

// runTopLevelForTest drives the real Execute() path (via run()) rather than
// calling a command's RunE directly, so the top-level error-envelope
// formatting in formatExecuteError is under test. cobra's persistent flags
// (rootFlag, jsonFlag, colorFlag) don't reset to default between Parse
// calls, so they're saved and reset here rather than relying on args alone.
func runTopLevelForTest(t *testing.T, args []string) (stdout, stderr string, exitCode int) {
	t.Helper()

	prevRootFlag, prevJSONFlag, prevColorFlag := rootFlag, jsonFlag, colorFlag
	rootFlag, jsonFlag, colorFlag = "", false, "auto"
	t.Cleanup(func() {
		rootFlag, jsonFlag, colorFlag = prevRootFlag, prevJSONFlag, prevColorFlag
	})

	oldStdout, oldStderr := os.Stdout, os.Stderr
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

	var stdoutBytes, stderrBytes []byte
	done := make(chan struct{})
	go func() {
		defer close(done)
		stdoutBytes, _ = io.ReadAll(stdoutR)
	}()
	stderrDone := make(chan struct{})
	go func() {
		defer close(stderrDone)
		stderrBytes, _ = io.ReadAll(stderrR)
	}()

	exitCode = run(args)

	_ = stdoutW.Close()
	_ = stderrW.Close()
	<-done
	<-stderrDone

	return string(stdoutBytes), string(stderrBytes), exitCode
}

type jsonErrorEnvelopeForTest struct {
	SchemaVersion int    `json:"schema_version"`
	Command       string `json:"command"`
	Error         struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func assertJSONErrorEnvelope(t *testing.T, stdout, stderr string, exitCode int, wantCommand, wantCode string) {
	t.Helper()
	if exitCode != 1 {
		t.Fatalf("exit code = %d, want 1", exitCode)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got: %q", stdout)
	}
	var envelope jsonErrorEnvelopeForTest
	if err := json.Unmarshal([]byte(stderr), &envelope); err != nil {
		t.Fatalf("unmarshal error envelope: %v\nstderr:%s", err, stderr)
	}
	if envelope.SchemaVersion != jsonSchemaVersion {
		t.Fatalf("schema_version = %d, want %d", envelope.SchemaVersion, jsonSchemaVersion)
	}
	if envelope.Command != wantCommand {
		t.Fatalf("command = %q, want %q", envelope.Command, wantCommand)
	}
	if envelope.Error.Code != wantCode {
		t.Fatalf("error.code = %q, want %q", envelope.Error.Code, wantCode)
	}
}

func TestJSONErrorEnvelopeMissingModel(t *testing.T) {
	root := t.TempDir()
	mustWriteCmdFile(t, filepath.Join(root, "config", "application.rb"), "")

	stdout, stderr, exitCode := runTopLevelForTest(t, []string{"--root", root, "--json", "model", "nope"})
	assertJSONErrorEnvelope(t, stdout, stderr, exitCode, "model", codeNotFound)
}

func TestJSONErrorEnvelopeMissingGem(t *testing.T) {
	root := t.TempDir()
	mustWriteCmdFile(t, filepath.Join(root, "config", "application.rb"), "")
	mustWriteCmdFile(t, filepath.Join(root, "Gemfile.lock"), testGemfileLock)

	stdout, stderr, exitCode := runTopLevelForTest(t, []string{"--root", root, "--json", "gem", "nonexistent_xyz"})
	assertJSONErrorEnvelope(t, stdout, stderr, exitCode, "gem", codeNotFound)
}

func TestJSONErrorEnvelopeNonRailsRoot(t *testing.T) {
	root := t.TempDir() // no config/application.rb

	stdout, stderr, exitCode := runTopLevelForTest(t, []string{"--root", root, "--json", "about"})
	assertJSONErrorEnvelope(t, stdout, stderr, exitCode, "about", codeNotARailsRoot)
}
