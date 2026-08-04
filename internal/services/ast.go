package services

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"

	"github.com/janstol/rails-kit/internal/astutil"
	"github.com/janstol/rails-kit/internal/config"
	"github.com/janstol/rails-kit/internal/prism"
)

// Parse reads a service file, parses it with Prism, and returns its structural
// summary. Prism is error-tolerant: recoverable syntax errors are attached to
// the summary while whatever structure Prism could recover is still returned.
func Parse(servicePath, railsRoot, servicesPath string) (*Summary, error) {
	ctx := context.Background()
	p, err := prism.NewParser(ctx)
	if err != nil {
		return nil, fmt.Errorf("creating prism parser: %w", err)
	}
	defer p.Close(ctx) //nolint:errcheck

	result, src, err := p.Parse(ctx, servicePath)
	if err != nil {
		return nil, err
	}

	s := summaryForPath(servicePath, railsRoot, servicesPath)
	for _, parseErr := range result.Errors {
		s.ParseErrors = append(s.ParseErrors, ParseDiagnostic{
			Line:    prism.LineAt(src, parseErr.Location.StartOffset),
			Message: parseErr.Message,
		})
	}
	if result.Value == nil {
		return s, nil
	}

	class, module := astutil.TopLevelClassOrModule(result.Value)
	if class == nil && module == nil {
		return s, nil
	}

	var body parser.Node
	if class != nil {
		s.Kind = "class"
		if class.Superclass != nil {
			s.ParentClass = prism.Slice(src, class.Superclass.GetLocation())
		}
		body = class.Body
	} else {
		s.Kind = "module"
		body = module.Body
	}

	w := serviceWalker{src: src, summary: s}
	w.walkClassBody(prism.BlockStatements(body))
	return s, nil
}

func summaryForPath(servicePath, railsRoot, servicesPath string) *Summary {
	s := &Summary{}
	rel, err := filepath.Rel(railsRoot, servicePath)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		rel = servicePath
	}
	s.RelPath = filepath.ToSlash(rel)

	servicesDir := config.ResolvePath(railsRoot, servicesPath)
	namePart, err := filepath.Rel(servicesDir, servicePath)
	if err != nil || strings.HasPrefix(namePart, "..") {
		namePart = filepath.Base(servicePath)
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
	s.ClassName = strings.Join(classSegments, "::")
	return s
}

type serviceWalker struct {
	src     []byte
	summary *Summary
}

// walkClassBody scans the service class's or module's top-level statements,
// tracking `private`/`protected`/`public` visibility switches (both the
// bare-call form and the `private def foo; end` single-method form) so only
// public instance methods are reported. Singleton methods (`def self.foo`) are
// collected regardless of visibility -- singleton-method visibility is
// unconventional and rare. Class-level constants (`DEFAULT_LIMIT = 100`) are
// collected as part of the service's interface.
func (w *serviceWalker) walkClassBody(nodes []parser.Node) {
	visibility := "public"
	for _, node := range nodes {
		switch n := node.(type) {
		case *parser.ConstantWriteNode:
			w.handleConstant(n)
		case *parser.CallNode:
			if n.Receiver == nil {
				switch n.Name {
				case "private", "protected", "public":
					args := prism.ArgNodes(n)
					if len(args) == 0 && n.Block == nil {
						visibility = n.Name
						continue
					}
					if len(args) == 1 {
						if def, ok := args[0].(*parser.DefNode); ok {
							w.handleDef(def, n.Name)
							continue
						}
					}
				}
			}
			w.handleCall(n)
		case *parser.DefNode:
			w.handleDef(n, visibility)
		}
	}
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
	if len(args) == 0 {
		return
	}
	name := astutil.ConstantName(w.src, args[0])
	if name == "" {
		return
	}
	w.summary.Concerns = append(w.summary.Concerns, "  "+name)
}

// handleConstant renders a class-level constant assignment as
// `  NAME = value`, collapsing any whitespace in the value expression.
func (w *serviceWalker) handleConstant(n *parser.ConstantWriteNode) {
	if n.Value == nil {
		return
	}
	w.summary.Constants = append(w.summary.Constants, "  "+n.Name+" = "+astutil.JoinedSource(w.src, n.Value.GetLocation()))
}
