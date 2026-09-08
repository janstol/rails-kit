package helpers

import (
	"github.com/danielgatis/go-ruby-prism/parser"

	"github.com/janstol/rails-kit/internal/astutil"
	"github.com/janstol/rails-kit/internal/prism"
)

// Parse reads a helper file, parses it with Prism, and returns its structural
// summary. Prism is error-tolerant: recoverable syntax errors are attached to
// the summary while whatever structure Prism could recover is still returned.
func Parse(helperPath, railsRoot, helpersPath string) (*Summary, error) {
	p, err := astutil.ParseFile(helperPath)
	if err != nil {
		return nil, err
	}

	s := &Summary{}
	s.RelPath, s.ClassName = astutil.SummaryPath(helperPath, railsRoot, helpersPath)
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

	w := helperWalker{src: p.Src, summary: s}
	astutil.WalkClassBody(prism.BlockStatements(body), astutil.ClassBody{
		Def:   w.handleDef,
		Call:  w.handleCall,
		Const: w.handleConstant,
	})
	return s, nil
}

type helperWalker struct {
	src     []byte
	summary *Summary
}

// handleDef collects an instance method (`def foo`, Receiver nil) only when it
// is public, and a class method (`def self.foo`, or a def inside a
// `class << self` block) regardless of visibility. Every collected method is
// rendered as a signature, since a helper's parameters are the useful part of
// its API.
func (w *helperWalker) handleDef(def *parser.DefNode, visibility string, singleton bool) {
	if singleton {
		w.summary.Methods = append(w.summary.Methods, "  "+astutil.Signature(w.src, def))
		return
	}
	if def.Receiver == nil && visibility == "public" {
		w.summary.Methods = append(w.summary.Methods, "  "+astutil.Signature(w.src, def))
	}
}

func (w *helperWalker) handleCall(call *parser.CallNode) {
	if call.Receiver != nil {
		return
	}
	args := prism.ArgNodes(call)
	switch call.Name {
	case "include":
		w.handleInclude(args)
	}
}

// handleInclude reports every included module. No framework prefix to filter,
// same reasoning as services.
func (w *helperWalker) handleInclude(args []parser.Node) {
	if name := astutil.IncludedConcern(w.src, args, nil); name != "" {
		w.summary.Concerns = append(w.summary.Concerns, "  "+name)
	}
}

// handleConstant renders a class-level constant assignment as
// `  NAME = value`, collapsing any whitespace in the value expression.
func (w *helperWalker) handleConstant(n *parser.ConstantWriteNode) {
	if n.Value == nil {
		return
	}
	w.summary.Constants = append(w.summary.Constants, "  "+n.Name+" = "+astutil.JoinedSource(w.src, n.Value.GetLocation()))
}
