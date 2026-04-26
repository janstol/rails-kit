package prism

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

//go:embed skeleton.rb
var helperScript string

// Runner invokes Ruby + Prism and returns structural summaries for Ruby files.
type Runner struct {
	Ruby string
	Dir  string
}

// File is a compact Prism-derived structural summary of a Ruby source file.
type File struct {
	Path        string     `json:"path"`
	RelPath     string     `json:"rel_path,omitempty"`
	Classes     []Class    `json:"classes,omitempty"`
	Modules     []Module   `json:"modules,omitempty"`
	Constants   []Constant `json:"constants,omitempty"`
	Calls       []Call     `json:"calls,omitempty"`
	Methods     []Method   `json:"methods,omitempty"`
	ParseErrors []string   `json:"parse_errors,omitempty"`
}

// Class describes a Ruby class and its structural children.
type Class struct {
	Name      string     `json:"name"`
	Parent    string     `json:"parent,omitempty"`
	StartLine int        `json:"start_line"`
	EndLine   int        `json:"end_line"`
	Includes  []Call     `json:"includes,omitempty"`
	Extends   []Call     `json:"extends,omitempty"`
	Prepends  []Call     `json:"prepends,omitempty"`
	Classes   []Class    `json:"classes,omitempty"`
	Modules   []Module   `json:"modules,omitempty"`
	Constants []Constant `json:"constants,omitempty"`
	Calls     []Call     `json:"calls,omitempty"`
	Methods   []Method   `json:"methods,omitempty"`
}

// Module describes a Ruby module and its structural children.
type Module struct {
	Name      string     `json:"name"`
	StartLine int        `json:"start_line"`
	EndLine   int        `json:"end_line"`
	Includes  []Call     `json:"includes,omitempty"`
	Extends   []Call     `json:"extends,omitempty"`
	Prepends  []Call     `json:"prepends,omitempty"`
	Classes   []Class    `json:"classes,omitempty"`
	Modules   []Module   `json:"modules,omitempty"`
	Constants []Constant `json:"constants,omitempty"`
	Calls     []Call     `json:"calls,omitempty"`
	Methods   []Method   `json:"methods,omitempty"`
}

// Constant is a Ruby constant assignment.
type Constant struct {
	Name      string `json:"name"`
	Source    string `json:"source"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
}

// Call is a structural call such as include, association, validation, callback,
// or a top-level DSL macro.
type Call struct {
	Name      string   `json:"name"`
	Kind      string   `json:"kind"`
	Args      []string `json:"args,omitempty"`
	Source    string   `json:"source"`
	StartLine int      `json:"start_line"`
	EndLine   int      `json:"end_line"`
}

// Method is a Ruby method signature without its body.
type Method struct {
	Name       string `json:"name"`
	Params     string `json:"params,omitempty"`
	Visibility string `json:"visibility"`
	StartLine  int    `json:"start_line"`
	EndLine    int    `json:"end_line"`
}

type request struct {
	Paths []string `json:"paths"`
}

type response struct {
	Files []File `json:"files"`
}

// ParseFiles runs Prism over paths in a single Ruby process.
func (r Runner) ParseFiles(ctx context.Context, paths []string) ([]File, error) {
	if len(paths) == 0 {
		return nil, nil
	}

	payload, err := json.Marshal(request{Paths: paths})
	if err != nil {
		return nil, fmt.Errorf("encoding prism request: %w", err)
	}

	stdout, err := r.runDirect(ctx, payload)
	if err != nil && r.Ruby == "" && IsUnavailable(err) {
		stdout, err = r.runViaShell(ctx, payload)
	}
	if err != nil {
		return nil, err
	}

	var resp response
	if err := json.Unmarshal(stdout, &resp); err != nil {
		return nil, fmt.Errorf("decoding prism output: %w", err)
	}
	if len(resp.Files) != len(paths) {
		return nil, fmt.Errorf("prism returned %d file(s), expected %d", len(resp.Files), len(paths))
	}
	return resp.Files, nil
}

func (r Runner) runDirect(ctx context.Context, payload []byte) ([]byte, error) {
	ruby := r.Ruby
	if ruby == "" {
		ruby = "ruby"
	}
	return r.run(ctx, payload, ruby, []string{"-rjson", "-e", helperScript}, nil)
}

func (r Runner) runViaShell(ctx context.Context, payload []byte) ([]byte, error) {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}
	env := []string{"RAILS_KIT_PRISM_HELPER=" + helperScript}
	return r.run(ctx, payload, shell, []string{"-ic", `exec ruby -rjson -e "$RAILS_KIT_PRISM_HELPER"`}, env)
}

func (r Runner) run(ctx context.Context, payload []byte, name string, args []string, extraEnv []string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	if r.Dir != "" {
		cmd.Dir = r.Dir
	}
	if len(extraEnv) > 0 {
		cmd.Env = append(os.Environ(), extraEnv...)
	}
	cmd.Stdin = bytes.NewReader(payload)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("running ruby prism helper: %s: %w", msg, err)
	}
	return stdout.Bytes(), nil
}

// IsUnavailable reports whether err is likely caused by Ruby or Prism being unavailable.
func IsUnavailable(err error) bool {
	if err == nil {
		return false
	}
	var execErr *exec.Error
	if errors.As(err, &execErr) {
		return true
	}
	msg := err.Error()
	return strings.Contains(msg, "cannot load such file -- prism") ||
		strings.Contains(msg, "LoadError") ||
		strings.Contains(msg, "executable file not found")
}
