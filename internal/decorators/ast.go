package decorators

import (
	"github.com/danielgatis/go-ruby-prism/parser"

	"github.com/janstol/rails-kit/internal/astutil"
	"github.com/janstol/rails-kit/internal/prism"
)

// Parse reads a decorator file, parses it with Prism, and returns its
// structural summary. Prism is error-tolerant: recoverable syntax errors are
// attached to the summary while whatever structure Prism could recover is
// still returned.
func Parse(decoratorPath, railsRoot, decoratorsPath string) (*Summary, error) {
	p, err := astutil.ParseFile(decoratorPath)
	if err != nil {
		return nil, err
	}

	s := &Summary{}
	s.RelPath, s.ClassName = astutil.SummaryPath(decoratorPath, railsRoot, decoratorsPath)
	s.ParseErrors = p.Diagnostics
	if p.Program == nil {
		return s, nil
	}

	class, module := astutil.TopLevelClassOrModule(p.Program)
	if class == nil && module == nil {
		return s, nil
	}

	var body parser.Node
	if class != nil {
		s.Kind = "class"
		if class.Superclass != nil {
			s.ParentClass = prism.Slice(p.Src, class.Superclass.GetLocation())
		}
		body = class.Body
	} else {
		s.Kind = "module"
		body = module.Body
	}

	w := decoratorWalker{src: p.Src, summary: s}
	astutil.WalkClassBody(prism.BlockStatements(body), astutil.ClassBody{
		Def:                 w.handleDef,
		Call:                w.handleCall,
		Const:               w.handleConstant,
		SkipVisibilityCalls: true,
	})
	return s, nil
}

type decoratorWalker struct {
	src     []byte
	summary *Summary
}

// handleDef collects an instance method (`def foo`, Receiver nil) only when
// it is public, and a class method (`def self.foo`, or a def inside a
// `class << self` block) regardless of visibility. Every collected method is
// rendered as a signature -- a decorator's parameters are part of its useful
// summary, same reasoning as helpers.
func (w *decoratorWalker) handleDef(def *parser.DefNode, visibility string, singleton bool) {
	if singleton {
		w.summary.Methods = append(w.summary.Methods, "  "+astutil.Signature(w.src, def))
		return
	}
	if def.Receiver == nil && visibility == "public" {
		w.summary.Methods = append(w.summary.Methods, "  "+astutil.Signature(w.src, def))
	}
}

// handleCall reports `include`d concerns separately and every other
// class-level call (delegate_all, decorates_association, and anything a
// custom decorator implementation makes) as a Macros catch-all, mirroring
// datagrids' handling of its own DSL. SkipVisibilityCalls keeps bare
// private/protected/public switches and the `private :a, :b` symbol-list
// form from leaking into Macros as noise.
func (w *decoratorWalker) handleCall(call *parser.CallNode) {
	if call.Receiver != nil {
		return
	}
	switch call.Name {
	case "include":
		w.handleInclude(prism.ArgNodes(call))
	default:
		w.summary.Macros = append(w.summary.Macros, w.renderCall(call))
	}
}

// handleInclude reports every included module. No framework prefix to
// filter, same reasoning as services and helpers.
func (w *decoratorWalker) handleInclude(args []parser.Node) {
	if name := astutil.IncludedConcern(w.src, args, nil); name != "" {
		w.summary.Concerns = append(w.summary.Concerns, "  "+name)
	}
}

// handleConstant renders a class-level constant assignment as
// `  NAME = value`, collapsing any whitespace in the value expression.
func (w *decoratorWalker) handleConstant(n *parser.ConstantWriteNode) {
	if n.Value == nil {
		return
	}
	w.summary.Constants = append(w.summary.Constants, "  "+n.Name+" = "+astutil.JoinedSource(w.src, n.Value.GetLocation()))
}

// renderCall renders a class-level call as `  name args` with whitespace
// collapsed, appending ` (block)` for a real block literal (`do…end` / `{…}`).
// A block-pass (`&:sym` / `&expr`) is carried on call.Block as a
// BlockArgumentNode, separate from the ArgumentsNode location, so it is folded
// into the rendered argument list rather than noted as a block.
func (w *decoratorWalker) renderCall(call *parser.CallNode) string {
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
