// Package astutil holds AST-walking helpers shared across the Prism-backed
// readers (controllers, mailers, jobs, services, datagrids, model, concerns).
// Each reader's Summary type is domain-specific, but the name-mangling,
// source-rendering, top-level class-discovery, parsing, and class-body-walking
// primitives are byte-identical across them, so they live here once rather
// than being copy-pasted per package.
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

// Signature renders a method as `name` or `name(params)`, collapsing a
// multi-line parameter list onto one line. Used by readers (helpers,
// decorators) where a method's parameters are part of the useful summary,
// unlike the bare-name rendering most readers use.
func Signature(src []byte, def *parser.DefNode) string {
	if def.Parameters != nil {
		if params := JoinedSource(src, def.Parameters.Location); params != "" {
			return def.Name + "(" + params + ")"
		}
	}
	return def.Name
}

// TopLevelClass returns the first *parser.ClassNode reachable from program's
// top-level statements, descending into *parser.ModuleNode bodies (the
// `module Admin; class ReportsController; ...; end; end` idiom) but not into
// another class's body -- a nested class (e.g. a rescued error type defined
// inline) does not count and is not descended into either, mirroring the rule
// against leaking nested-class methods.
//
// Descent prefers a *pure namespace* module (see isPureNamespace) over a
// content-bearing one: a module that also defines its own methods is skipped
// in favor of a real class found among its later siblings, so that module's
// contents don't hijack the search away from the class the caller actually
// wants. Unlike TopLevelClassOrModule this function can only ever return a
// class, never a module -- so if no pure-namespace path turns up a class
// anywhere, it falls back to descending into content-bearing modules too
// (today's behavior) rather than returning nil. A hard guard here would mean
// a file that plainly contains a controller class reports nothing at all,
// which is worse than returning the nested class imperfectly.
func TopLevelClass(program *parser.ProgramNode) *parser.ClassNode {
	if program.Statements == nil {
		return nil
	}
	return findClassInStatements(program.Statements.Body)
}

func findClassInStatements(nodes []parser.Node) *parser.ClassNode {
	if c := findClass(nodes, true); c != nil {
		return c
	}
	return findClass(nodes, false)
}

func findClass(nodes []parser.Node, pureOnly bool) *parser.ClassNode {
	for _, node := range nodes {
		switch n := node.(type) {
		case *parser.ClassNode:
			return n
		case *parser.ModuleNode:
			body := prism.BlockStatements(n.Body)
			if pureOnly && !isPureNamespace(body) {
				continue
			}
			if c := findClass(body, pureOnly); c != nil {
				return c
			}
		}
	}
	return nil
}

// TopLevelClassOrModule returns the first *parser.ClassNode or *parser.ModuleNode
// reachable from program's top-level statements, descending into *parser.ModuleNode
// bodies (the `module Admin; class X; ...; end; end` idiom) but not into another
// class's or module's body -- a nested class/module (e.g. a rescued error type
// defined inline) does not count and is not descended into either, mirroring the
// anti-leak rule of TopLevelClass.
//
// Unlike TopLevelClass, when the outermost container is a module with no nested
// class it returns that module (as the module result) rather than skipping it.
// This is what lets a reader recognize module-style services
// (`module Foo; def self.bar; end; end`).
//
// Descent only happens through a *pure namespace* module -- one whose body
// holds nothing but nested class/module declarations and (optionally)
// namespace-level constants. A module that also defines its own methods
// (e.g. `module Foo; def bar; end; class Helper; ...; end; end`, a helper
// exposing its API alongside a private implementation class) is returned as
// itself rather than having its methods discarded in favor of the nested
// class -- nesting only ever hides a class's contents from its container, it
// never promotes the nested class over a container that has real content of
// its own.
//
// At most one of class or module is non-nil.
func TopLevelClassOrModule(program *parser.ProgramNode) (class *parser.ClassNode, module *parser.ModuleNode) {
	if program.Statements == nil {
		return nil, nil
	}
	return findClassOrModuleInStatements(program.Statements.Body)
}

func findClassOrModuleInStatements(nodes []parser.Node) (*parser.ClassNode, *parser.ModuleNode) {
	var found parser.Node
	for _, node := range nodes {
		switch node.(type) {
		case *parser.ClassNode, *parser.ModuleNode:
			found = node
		}
		if found != nil {
			break
		}
	}
	if found == nil {
		return nil, nil
	}
	if class, ok := found.(*parser.ClassNode); ok {
		return class, nil
	}
	module := found.(*parser.ModuleNode)
	body := prism.BlockStatements(module.Body)
	if isPureNamespace(body) {
		if c, m := findClassOrModuleInStatements(body); c != nil || m != nil {
			return c, m
		}
	}
	return nil, module
}

// isPureNamespace reports whether nodes hold nothing but nested class/module
// declarations and namespace-level constants -- the shape that makes it safe
// to descend past a module looking for the class/module it wraps, rather than
// treating the module itself as the target.
func isPureNamespace(nodes []parser.Node) bool {
	for _, node := range nodes {
		switch node.(type) {
		case *parser.ClassNode, *parser.ModuleNode, *parser.ConstantWriteNode:
			continue
		default:
			return false
		}
	}
	return true
}
