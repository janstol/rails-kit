package presenters

import (
	"github.com/danielgatis/go-ruby-prism/parser"

	"github.com/janstol/rails-kit/internal/astutil"
	"github.com/janstol/rails-kit/internal/prism"
)

// Parse reads a presenter file, parses it with Prism, and returns its
// structural summary. Prism is error-tolerant: recoverable syntax errors are
// attached to the summary while whatever structure Prism could recover is
// still returned.
func Parse(presenterPath, railsRoot, presentersPath string) (*Summary, error) {
	p, err := astutil.ParseFile(presenterPath)
	if err != nil {
		return nil, err
	}

	s := &Summary{}
	s.RelPath, s.ClassName = astutil.SummaryPath(presenterPath, railsRoot, presentersPath)
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

	w := presenterWalker{src: p.Src, summary: s}
	astutil.WalkClassBody(prism.BlockStatements(body), astutil.ClassBody{
		Def:                 w.handleDef,
		Call:                w.handleCall,
		Const:               w.handleConstant,
		SkipVisibilityCalls: true,
	})
	return s, nil
}

type presenterWalker struct {
	src     []byte
	summary *Summary
}

// handleDef collects an instance method (`def foo`, Receiver nil) only when
// it is public, and a singleton method (`def self.foo`, Receiver is
// *SelfNode) regardless of visibility. Every collected method is rendered as
// a signature -- a presenter's parameters are part of its useful summary,
// same reasoning as helpers, decorators, and formers.
func (w *presenterWalker) handleDef(def *parser.DefNode, visibility string) {
	if _, ok := def.Receiver.(*parser.SelfNode); ok {
		w.summary.Methods = append(w.summary.Methods, "  "+astutil.Signature(w.src, def))
		return
	}
	if def.Receiver == nil && visibility == "public" {
		w.summary.Methods = append(w.summary.Methods, "  "+astutil.Signature(w.src, def))
	}
}

// handleCall reports `include`d concerns separately, attr_accessor/reader/
// writer as Attributes -- a presenter's attr_reader line says what it wraps,
// which is the thing you actually want when you open one -- and everything
// else (delegate, and any app-specific macro) as a Macros catch-all.
// SkipVisibilityCalls keeps bare private/protected/public switches and the
// `private :a, :b` symbol-list form from leaking into Macros as noise.
func (w *presenterWalker) handleCall(call *parser.CallNode) {
	if call.Receiver != nil {
		return
	}
	switch call.Name {
	case "include":
		w.handleInclude(prism.ArgNodes(call))
	case "attr_accessor", "attr_reader", "attr_writer":
		w.summary.Attributes = append(w.summary.Attributes, w.renderCall(call))
	default:
		w.summary.Macros = append(w.summary.Macros, w.renderCall(call))
	}
}

// handleInclude reports every included module. No framework prefix to
// filter, same reasoning as services, helpers, decorators, and formers.
func (w *presenterWalker) handleInclude(args []parser.Node) {
	if name := astutil.IncludedConcern(w.src, args, nil); name != "" {
		w.summary.Concerns = append(w.summary.Concerns, "  "+name)
	}
}

// handleConstant renders a class-level constant assignment as
// `  NAME = value`, collapsing any whitespace in the value expression.
func (w *presenterWalker) handleConstant(n *parser.ConstantWriteNode) {
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
func (w *presenterWalker) renderCall(call *parser.CallNode) string {
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
