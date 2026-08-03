package concerns

import (
	"context"
	"fmt"

	"github.com/danielgatis/go-ruby-prism/parser"
	"github.com/janstol/rails-kit/internal/prism"
)

// Parse reads a concern .rb file, parses it with Prism, and returns its
// structural detail. Prism is error-tolerant: recoverable syntax errors are
// attached to the detail while whatever structure Prism could recover is
// still returned.
func Parse(filePath, relPath, concernType string) (*ConcernDetail, error) {
	ctx := context.Background()
	p, err := prism.NewParser(ctx)
	if err != nil {
		return nil, fmt.Errorf("creating prism parser: %w", err)
	}
	defer p.Close(ctx) //nolint:errcheck

	result, src, err := p.Parse(ctx, filePath)
	if err != nil {
		return nil, err
	}

	d := &ConcernDetail{
		Path: relPath,
		Type: concernType,
	}
	for _, parseErr := range result.Errors {
		d.ParseErrors = append(d.ParseErrors, ParseDiagnostic{
			Line:    prism.LineAt(src, parseErr.Location.StartOffset),
			Message: parseErr.Message,
		})
	}
	if result.Value == nil {
		return d, nil
	}

	module := topLevelModule(result.Value)
	if module == nil {
		return d, nil
	}
	d.Name = prism.Slice(src, module.ConstantPath.GetLocation())

	w := concernWalker{detail: d}
	w.walkModuleBody(prism.BlockStatements(module.Body))
	return d, nil
}

// topLevelModule returns the first *parser.ModuleNode among the program's
// top-level statements. A concern file with true nested-keyword modules
// ("module A; module B; ...; end; end") reports the outer module's name
// only, not "A::B" -- a known divergence from the regex scanner's behavior,
// decided per real-app occurrence rather than fixed unconditionally.
func topLevelModule(program *parser.ProgramNode) *parser.ModuleNode {
	if program.Statements == nil {
		return nil
	}
	for _, node := range program.Statements.Body {
		if m, ok := node.(*parser.ModuleNode); ok {
			return m
		}
	}
	return nil
}

type concernWalker struct {
	detail *ConcernDetail
}

// walkModuleBody scans a module's top-level statements for the constructs a
// concern file is built from: instance methods, an `included do` block, a
// `class_methods do` block, and a `class << self` singleton body. A nested
// module is walked transparently (its methods still count) so a concern
// wrapped in an extra namespace does not lose its content, even though the
// reported Name only reflects the outermost module.
func (w *concernWalker) walkModuleBody(nodes []parser.Node) {
	for _, node := range nodes {
		switch n := node.(type) {
		case *parser.ModuleNode:
			w.walkModuleBody(prism.BlockStatements(n.Body))
		case *parser.DefNode:
			if n.Receiver == nil {
				w.detail.Methods = append(w.detail.Methods, n.Name)
				continue
			}
			if _, ok := n.Receiver.(*parser.SelfNode); ok && n.Name == "included" {
				w.walkLegacyIncludedHook(n)
			}
		case *parser.SingletonClassNode:
			if _, ok := n.Expression.(*parser.SelfNode); ok {
				w.detail.ClassMethods = append(w.detail.ClassMethods, defNames(prism.BlockStatements(n.Body))...)
			}
		case *parser.CallNode:
			w.walkCall(n)
		}
	}
}

func (w *concernWalker) walkCall(n *parser.CallNode) {
	if n.Receiver != nil || n.Block == nil {
		return
	}
	block, ok := n.Block.(*parser.BlockNode)
	if !ok {
		return
	}
	switch n.Name {
	case "class_methods":
		w.detail.HasClassMethodsBlock = true
		w.detail.ClassMethods = append(w.detail.ClassMethods, defNames(prism.BlockStatements(block.Body))...)
	case "included":
		w.detail.HasIncludedBlock = true
		w.detail.Methods = append(w.detail.Methods, defNames(prism.BlockStatements(block.Body))...)
	}
}

// walkLegacyIncludedHook handles the pre-ActiveSupport::Concern idiom
// `def self.included(base); base.class_eval do ... end; end`, treating a
// `class_eval`/`instance_eval` block found in its body the same as an
// `included do...end` block: the defs nested inside are instance methods
// added to the includer, not methods of the hook itself.
func (w *concernWalker) walkLegacyIncludedHook(n *parser.DefNode) {
	for _, node := range prism.BlockStatements(n.Body) {
		call, ok := node.(*parser.CallNode)
		if !ok || call.Block == nil {
			continue
		}
		if call.Name != "class_eval" && call.Name != "instance_eval" {
			continue
		}
		block, ok := call.Block.(*parser.BlockNode)
		if !ok {
			continue
		}
		w.detail.HasIncludedBlock = true
		w.detail.Methods = append(w.detail.Methods, defNames(prism.BlockStatements(block.Body))...)
		return
	}
}

// defNames returns the names of top-level, non-singleton def nodes among
// nodes, in source order.
func defNames(nodes []parser.Node) []string {
	var names []string
	for _, node := range nodes {
		if d, ok := node.(*parser.DefNode); ok && d.Receiver == nil {
			names = append(names, d.Name)
		}
	}
	return names
}
