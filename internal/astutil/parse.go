package astutil

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"

	"github.com/janstol/rails-kit/internal/config"
	"github.com/janstol/rails-kit/internal/prism"
)

// ParseDiagnostic describes a recoverable Ruby syntax error reported by Prism.
type ParseDiagnostic struct {
	Line    int
	Message string
}

// Parsed holds what a one-shot Prism parse of a single file recovered.
// Program is nil when Prism recovered no tree at all.
type Parsed struct {
	Src         []byte
	Program     *parser.ProgramNode
	Diagnostics []ParseDiagnostic
}

// ParseFile opens a one-shot Prism parser, parses path, and returns the
// recovered program together with any recoverable syntax errors. Prism is
// error-tolerant: diagnostics and a partial tree can both be present.
func ParseFile(path string) (*Parsed, error) {
	ctx := context.Background()
	p, err := prism.NewParser(ctx)
	if err != nil {
		return nil, fmt.Errorf("creating prism parser: %w", err)
	}
	defer p.Close(ctx) //nolint:errcheck

	result, src, err := p.Parse(ctx, path)
	if err != nil {
		return nil, err
	}

	parsed := &Parsed{Src: src}
	for _, parseErr := range result.Errors {
		parsed.Diagnostics = append(parsed.Diagnostics, ParseDiagnostic{
			Line:    prism.LineAt(src, parseErr.Location.StartOffset),
			Message: parseErr.Message,
		})
	}
	parsed.Program = result.Value
	return parsed, nil
}

// SummaryPath derives the two path-shaped fields every reader's Summary
// carries: relPath is filePath relative to railsRoot (falling back to filePath
// when it lies outside), and className is the CamelCase constant name implied
// by filePath's position under dirPath.
func SummaryPath(filePath, railsRoot, dirPath string) (relPath, className string) {
	rel, err := filepath.Rel(railsRoot, filePath)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		rel = filePath
	}
	relPath = filepath.ToSlash(rel)

	resolvedDir := config.ResolvePath(railsRoot, dirPath)
	namePart, err := filepath.Rel(resolvedDir, filePath)
	if err != nil || strings.HasPrefix(namePart, "..") {
		namePart = filepath.Base(filePath)
	}
	namePart = strings.TrimSuffix(namePart, ".rb")
	classSegments := make([]string, 0, 2)
	for _, seg := range strings.Split(namePart, string(filepath.Separator)) {
		var camel string
		for _, part := range strings.Split(seg, "_") {
			if part != "" {
				camel += strings.ToUpper(part[:1]) + part[1:]
			}
		}
		classSegments = append(classSegments, camel)
	}
	className = strings.Join(classSegments, "::")
	return relPath, className
}
