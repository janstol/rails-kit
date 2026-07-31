package prism

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"sync"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// Runner parses Ruby files with an in-process Prism (via WASM) parser and
// returns structural summaries for them.
type Runner struct{}

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

// ParseFiles parses paths, using a fresh Prism parser instance per file.
//
// go-ruby-prism v1.1.0's Parser.Parse is not safe to reuse across multiple
// calls: reusing one instance across a batch of real files intermittently
// trips WASM "out of bounds memory access" traps on otherwise-valid Ruby
// input (~50% of files in one repeated test, deterministic per input order
// but not a permanent wedge — a failing call could be followed by a
// succeeding one on the same instance). The exact mechanism inside the WASM
// runtime is not confirmed. A fresh parser per file, used once and
// discarded, was 100% reliable across the same input; there is no cheaper
// reset/recycle primitive on Parser to avoid paying a full cold start.
//
// Because that cold start (~150-160ms, a wazero runtime boot plus WASM module
// compilation) dominates and is paid per file regardless of batch size,
// ParseFiles fans out across a bounded number of goroutines. Each goroutine
// constructs and owns its own parser.NewParser instance with an isolated
// wazero runtime, so distinct parsers running concurrently do not share any
// state — only the serial per-instance reuse above is unsafe. Results are
// written to a preallocated slice by input index, so output order matches
// input order regardless of which file finishes parsing first.
func (r Runner) ParseFiles(ctx context.Context, paths []string) ([]File, error) {
	if len(paths) == 0 {
		return nil, nil
	}

	workers := min(runtime.NumCPU(), len(paths))
	sem := make(chan struct{}, workers)

	files := make([]File, len(paths))
	errs := make([]error, len(paths))

	var wg sync.WaitGroup
	for i, path := range paths {
		if ctx.Err() != nil {
			errs[i] = ctx.Err()
			continue
		}

		sem <- struct{}{}
		wg.Add(1)
		go func(i int, path string) {
			defer wg.Done()
			defer func() { <-sem }()

			if ctx.Err() != nil {
				errs[i] = ctx.Err()
				return
			}

			file, err := parseOne(ctx, path)
			if err != nil {
				errs[i] = err
				return
			}
			files[i] = file
		}(i, path)
	}
	wg.Wait()

	for _, err := range errs {
		if err != nil {
			return nil, err
		}
	}
	return files, nil
}

// parseOne reads and parses a single Ruby file with a fresh Prism parser
// instance, see the ParseFiles doc comment for why the parser is not reused.
func parseOne(ctx context.Context, path string) (File, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return File{}, fmt.Errorf("reading %s: %w", path, err)
	}

	p, err := parser.NewParser(ctx)
	if err != nil {
		return File{}, fmt.Errorf("creating prism parser: %w", err)
	}
	result, err := p.Parse(ctx, src)
	closeErr := p.Close(ctx)
	if err != nil {
		return File{}, fmt.Errorf("parsing %s: %w", path, err)
	}
	if closeErr != nil {
		return File{}, fmt.Errorf("closing prism parser after %s: %w", path, closeErr)
	}

	return buildFile(path, src, result), nil
}
