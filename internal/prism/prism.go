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

// Parser parses Ruby source on demand using one pooled go-ruby-prism parser.
// It is the single-file, on-demand counterpart to Runner.ParseFiles: routes
// walks files lazily and interleaved (walk routes.rb, on `draw :admin` parse
// config/routes/admin.rb, on `concern :foo` re-walk the stored block), so a
// pre-collected batch fan-out does not fit. Pool size 1 is correct because
// that walk is single-threaded. The one-time ~150 ms cold start is paid lazily
// on the first Parse, matching the accepted routes --static budget.
type Parser struct {
	p *parser.Parser
}

// NewParser creates an on-demand parser owning a single pooled WASM instance.
func NewParser(ctx context.Context) (*Parser, error) {
	p, err := parser.NewParser(ctx, parser.WithPoolSize(1))
	if err != nil {
		return nil, fmt.Errorf("creating prism parser: %w", err)
	}
	return &Parser{p: p}, nil
}

// Parse reads and parses a single Ruby file, returning the parse result and
// the source bytes (the latter so callers can resolve node locations to source
// text and line numbers).
func (p *Parser) Parse(ctx context.Context, path string) (*parser.ParseResult, []byte, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("reading %s: %w", path, err)
	}
	result, err := p.p.Parse(ctx, src)
	if err != nil {
		return nil, nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	return result, src, nil
}

// Close releases the pooled WASM instance.
func (p *Parser) Close(ctx context.Context) error {
	return p.p.Close(ctx)
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

// ParseFiles parses paths using one shared Prism parser, which owns a pool of
// WASM instances (sized to the worker count).
//
// go-ruby-prism v1.2.0 fixed the parser-reuse memory bug present in v1.1.0
// (reusing one *parser.Parser across distinct files intermittently trapped
// with a WASM "out of bounds memory access" in pm_options_free; a fresh
// parser per file was the workaround). A v1.2.0 Parser owns a lazily-filled
// pool of WASM instances, each with its own linear memory, and Parse checks
// one out and returns it, so one Parser is safe to reuse across many calls,
// concurrently. That changes the cold-start cost from per-file (a wazero
// runtime boot plus module compile, ~150-160ms each, paid once per file) to
// per-pool-instance: the pool grows to `workers` instances and no further, so
// a batch pays at most `workers` cold starts total, then warm parses for the
// rest.
//
// ParseFiles fans out across a bounded number of goroutines. The concurrency
// bound matches the pool size, so an instance is always available when a
// goroutine calls Parse and the pool never blocks on acquire. Results are
// written to a preallocated slice by input index, so output order matches
// input order regardless of which file finishes parsing first.
func (r Runner) ParseFiles(ctx context.Context, paths []string) ([]File, error) {
	if len(paths) == 0 {
		return nil, nil
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	workers := min(runtime.NumCPU(), len(paths))
	p, err := parser.NewParser(ctx, parser.WithPoolSize(workers))
	if err != nil {
		return nil, fmt.Errorf("creating prism parser: %w", err)
	}

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

			file, err := parseOne(ctx, p, path)
			if err != nil {
				errs[i] = err
				return
			}
			files[i] = file
		}(i, path)
	}
	wg.Wait()

	// All goroutines have returned their instances to the pool, so Close can
	// drain it. A parse error takes precedence over a close error, since the
	// close error is most likely a consequence of a cancelled context anyway.
	closeErr := p.Close(ctx)

	for _, err := range errs {
		if err != nil {
			return nil, err
		}
	}
	if closeErr != nil {
		return nil, fmt.Errorf("closing prism parser: %w", closeErr)
	}
	return files, nil
}

// parseOne reads and parses a single Ruby file using the shared pooled parser.
func parseOne(ctx context.Context, p *parser.Parser, path string) (File, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return File{}, fmt.Errorf("reading %s: %w", path, err)
	}

	result, err := p.Parse(ctx, src)
	if err != nil {
		return File{}, fmt.Errorf("parsing %s: %w", path, err)
	}

	return buildFile(path, src, result), nil
}
