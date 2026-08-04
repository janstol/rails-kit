// Package astutil holds AST-walking helpers shared across the Prism-backed
// readers (controllers, mailers, jobs). Each reader's Summary type is
// domain-specific, but the name-mangling, source-rendering, and top-level
// class-discovery primitives are byte-identical across them, so they live here
// once rather than being copy-pasted per package.
package astutil

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"

	"github.com/janstol/rails-kit/internal/prism"
)

var (
	reHasUpper        = regexp.MustCompile(`[A-Z]`)
	reAcronymBoundary = regexp.MustCompile(`([A-Z\d]+)([A-Z][a-z])`)
	reWordBoundary    = regexp.MustCompile(`([a-z\d])([A-Z])`)
)

// Underscore converts a CamelCase or namespaced class name into its Rails
// autoloading-style snake_case path, mirroring ActiveSupport's `underscore`.
// Inputs without uppercase letters (already snake_case) are returned unchanged.
func Underscore(input string) string {
	if !reHasUpper.MatchString(input) {
		return input
	}
	result := strings.ReplaceAll(input, "::", "/")
	result = reAcronymBoundary.ReplaceAllString(result, "${1}_${2}")
	result = reWordBoundary.ReplaceAllString(result, "${1}_${2}")
	return strings.ToLower(result)
}

// NormalizeLookupName converts a lookup name's separators to the OS path
// separator and cleans it, so a name given with either slash form resolves the
// same way regardless of platform.
func NormalizeLookupName(name string) string {
	replaced := strings.ReplaceAll(name, "\\", string(filepath.Separator))
	replaced = strings.ReplaceAll(replaced, "/", string(filepath.Separator))
	return filepath.Clean(replaced)
}

// ConstantName returns the source text of node when it is a constant read or
// constant path (e.g. `Retryable`, `ActiveJob::DeserializationError`), or "" for
// anything else. Used to read concern/exception class names from call args.
func ConstantName(src []byte, node parser.Node) string {
	switch node.(type) {
	case *parser.ConstantReadNode, *parser.ConstantPathNode:
		return strings.TrimSpace(prism.Slice(src, node.GetLocation()))
	default:
		return ""
	}
}

// JoinedSource returns the source text at loc with all whitespace runs
// (including newlines from multi-line expressions) collapsed to a single
// space, for compact one-line display.
func JoinedSource(src []byte, loc parser.Location) string {
	return strings.Join(strings.Fields(prism.Slice(src, loc)), " ")
}

// IsFalseNode reports whether node is a `false` literal, used to detect
// `layout false` and similar opt-out declarations.
func IsFalseNode(n parser.Node) bool {
	_, ok := n.(*parser.FalseNode)
	return ok
}

// TopLevelClass returns the first *parser.ClassNode reachable from program's
// top-level statements, descending into *parser.ModuleNode bodies (the
// `module Admin; class ReportsController; ...; end; end` idiom) but not into
// another class's body -- a nested class (e.g. a rescued error type defined
// inline) does not count and is not descended into either, mirroring the rule
// against leaking nested-class methods.
func TopLevelClass(program *parser.ProgramNode) *parser.ClassNode {
	if program.Statements == nil {
		return nil
	}
	return findClassInStatements(program.Statements.Body)
}

func findClassInStatements(nodes []parser.Node) *parser.ClassNode {
	for _, node := range nodes {
		switch n := node.(type) {
		case *parser.ClassNode:
			return n
		case *parser.ModuleNode:
			if c := findClassInStatements(prism.BlockStatements(n.Body)); c != nil {
				return c
			}
		}
	}
	return nil
}
