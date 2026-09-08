package datagrids

import (
	"github.com/danielgatis/go-ruby-prism/parser"

	"github.com/janstol/rails-kit/internal/astutil"
	"github.com/janstol/rails-kit/internal/prism"
)

// Parse reads a datagrid file, parses it with Prism, and returns its structural
// summary. Prism is error-tolerant: recoverable syntax errors are attached to
// the summary while whatever structure Prism could recover is still returned.
func Parse(datagridPath, railsRoot, datagridsPath string) (*Summary, error) {
	p, err := astutil.ParseFile(datagridPath)
	if err != nil {
		return nil, err
	}

	s := &Summary{}
	s.RelPath, s.ClassName = astutil.SummaryPath(datagridPath, railsRoot, datagridsPath)
	s.ParseErrors = p.Diagnostics
	if p.Program == nil {
		return s, nil
	}

	class := astutil.TopLevelClass(p.Program)
	if class == nil {
		return s, nil
	}
	if class.Superclass != nil {
		s.ParentClass = prism.Slice(p.Src, class.Superclass.GetLocation())
	}

	w := datagridWalker{src: p.Src, summary: s}
	astutil.WalkClassBody(prism.BlockStatements(class.Body), astutil.ClassBody{
		Def:                 w.handleDef,
		Call:                w.handleCall,
		SkipVisibilityCalls: true,
	})
	return s, nil
}

type datagridWalker struct {
	src     []byte
	summary *Summary
}

// handleDef collects an instance method (`def foo`, Receiver nil) only when it
// is public, and a class method (`def self.foo`, or a def inside a
// `class << self` block) regardless of visibility.
func (w *datagridWalker) handleDef(def *parser.DefNode, visibility string, singleton bool) {
	if singleton {
		w.summary.Methods = append(w.summary.Methods, "  "+def.Name)
		return
	}
	if def.Receiver == nil && visibility == "public" {
		w.summary.Methods = append(w.summary.Methods, "  "+def.Name)
	}
}

func (w *datagridWalker) handleCall(call *parser.CallNode) {
	if call.Receiver != nil {
		return
	}
	switch call.Name {
	case "include":
		w.handleInclude(prism.ArgNodes(call))
	case "filter":
		w.summary.Filters = append(w.summary.Filters, w.renderCall(call))
	case "column":
		w.summary.Columns = append(w.summary.Columns, w.renderCall(call))
	case "decorate":
		if block, ok := call.Block.(*parser.BlockNode); ok {
			w.summary.Decorate = w.decorateValue(block)
			return
		}
		w.summary.Macros = append(w.summary.Macros, w.renderCall(call))
	case "scope":
		if _, ok := call.Block.(*parser.BlockNode); ok {
			w.summary.Scope = "(block)"
			return
		}
		w.summary.Macros = append(w.summary.Macros, w.renderCall(call))
	default:
		w.summary.Macros = append(w.summary.Macros, w.renderCall(call))
	}
}

// handleInclude reports every included module. Datagrids have no framework base
// to suppress (unlike controllers' ActionController/ActiveSupport prefixes), so
// no prefix is filtered out.
func (w *datagridWalker) handleInclude(args []parser.Node) {
	if name := astutil.IncludedConcern(w.src, args, nil); name != "" {
		w.summary.Concerns = append(w.summary.Concerns, "  "+name)
	}
}

// renderCall renders a class-level call as `  name args` with whitespace
// collapsed, appending ` (block)` for a real block literal (`do…end` / `{…}`).
// A block-pass (`&:sym` / `&expr`) is carried on call.Block as a
// BlockArgumentNode, separate from the ArgumentsNode location, so it is folded
// into the rendered argument list rather than noted as a block.
func (w *datagridWalker) renderCall(call *parser.CallNode) string {
	entry := "  " + call.Name
	argSrc := ""
	if call.Arguments != nil {
		argSrc = astutil.JoinedSource(w.src, call.Arguments.GetLocation())
	}
	if bp, ok := call.Block.(*parser.BlockArgumentNode); ok {
		passSrc := astutil.JoinedSource(w.src, bp.GetLocation())
		if argSrc == "" {
			argSrc = passSrc
		} else {
			argSrc += ", " + passSrc
		}
	}
	if argSrc != "" {
		entry += " " + argSrc
	}
	if _, ok := call.Block.(*parser.BlockNode); ok {
		entry += " (block)"
	}
	return entry
}

// decorateValue extracts the decorator class from a `decorate { X }` block body
// when it is a single constant (e.g. `ExampleDecorator`); otherwise it falls
// back to "(block)".
func (w *datagridWalker) decorateValue(block *parser.BlockNode) string {
	stmts := prism.BlockStatements(block.Body)
	if len(stmts) == 1 {
		if name := astutil.ConstantName(w.src, stmts[0]); name != "" {
			return name
		}
	}
	return "(block)"
}
