package services

import (
	"github.com/danielgatis/go-ruby-prism/parser"

	"github.com/janstol/rails-kit/internal/astutil"
	"github.com/janstol/rails-kit/internal/prism"
)

// Parse reads a service file, parses it with Prism, and returns its structural
// summary. Prism is error-tolerant: recoverable syntax errors are attached to
// the summary while whatever structure Prism could recover is still returned.
func Parse(servicePath, railsRoot, servicesPath string) (*Summary, error) {
	p, err := astutil.ParseFile(servicePath)
	if err != nil {
		return nil, err
	}

	s := &Summary{}
	s.RelPath, s.ClassName = astutil.SummaryPath(servicePath, railsRoot, servicesPath)
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

	w := serviceWalker{src: p.Src, summary: s}
	astutil.WalkClassBody(prism.BlockStatements(body), astutil.ClassBody{
		Def:   w.handleDef,
		Call:  w.handleCall,
		Const: w.handleConstant,
	})
	return s, nil
}

type serviceWalker struct {
	src     []byte
	summary *Summary
}

// handleDef collects an instance method (`def foo`, Receiver nil) only when it
// is public, and a singleton method (`def self.foo`, Receiver is *SelfNode)
// regardless of visibility.
func (w *serviceWalker) handleDef(def *parser.DefNode, visibility string) {
	if _, ok := def.Receiver.(*parser.SelfNode); ok {
		w.handleSingletonDef(def)
		return
	}
	if def.Receiver == nil && visibility == "public" {
		w.summary.Methods = append(w.summary.Methods, "  "+def.Name)
	}
}

// handleSingletonDef collects a `def self.foo` method. Singleton-method
// visibility is unconventional and rare, so these are collected regardless of
// the surrounding visibility switch.
func (w *serviceWalker) handleSingletonDef(def *parser.DefNode) {
	w.summary.Methods = append(w.summary.Methods, "  "+def.Name)
}

func (w *serviceWalker) handleCall(call *parser.CallNode) {
	if call.Receiver != nil {
		return
	}
	args := prism.ArgNodes(call)
	switch call.Name {
	case "include":
		w.handleInclude(args)
	}
}

// handleInclude reports every included module. Services have no framework base
// (unlike ActionController/ActiveJob), so no prefix is filtered out.
func (w *serviceWalker) handleInclude(args []parser.Node) {
	if name := astutil.IncludedConcern(w.src, args, nil); name != "" {
		w.summary.Concerns = append(w.summary.Concerns, "  "+name)
	}
}

// handleConstant renders a class-level constant assignment as
// `  NAME = value`, collapsing any whitespace in the value expression.
func (w *serviceWalker) handleConstant(n *parser.ConstantWriteNode) {
	if n.Value == nil {
		return
	}
	w.summary.Constants = append(w.summary.Constants, "  "+n.Name+" = "+astutil.JoinedSource(w.src, n.Value.GetLocation()))
}
