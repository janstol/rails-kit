package datagrids

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

// Parse reads a datagrid file, parses it with Prism, and returns its structural
// summary. Prism is error-tolerant: recoverable syntax errors are attached to
// the summary while whatever structure Prism could recover is still returned.
func Parse(datagridPath, railsRoot, datagridsPath string) (*Summary, error) {
	ctx := context.Background()
	p, err := prism.NewParser(ctx)
	if err != nil {
		return nil, fmt.Errorf("creating prism parser: %w", err)
	}
	defer p.Close(ctx) //nolint:errcheck

	result, src, err := p.Parse(ctx, datagridPath)
	if err != nil {
		return nil, err
	}

	s := summaryForPath(datagridPath, railsRoot, datagridsPath)
	for _, parseErr := range result.Errors {
		s.ParseErrors = append(s.ParseErrors, ParseDiagnostic{
			Line:    prism.LineAt(src, parseErr.Location.StartOffset),
			Message: parseErr.Message,
		})
	}
	if result.Value == nil {
		return s, nil
	}

	class := astutil.TopLevelClass(result.Value)
	if class == nil {
		return s, nil
	}
	if class.Superclass != nil {
		s.ParentClass = prism.Slice(src, class.Superclass.GetLocation())
	}

	w := datagridWalker{src: src, summary: s}
	w.walkClassBody(prism.BlockStatements(class.Body))
	return s, nil
}

func summaryForPath(datagridPath, railsRoot, datagridsPath string) *Summary {
	s := &Summary{}
	rel, err := filepath.Rel(railsRoot, datagridPath)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		rel = datagridPath
	}
	s.RelPath = filepath.ToSlash(rel)

	datagridsDir := config.ResolvePath(railsRoot, datagridsPath)
	namePart, err := filepath.Rel(datagridsDir, datagridPath)
	if err != nil || strings.HasPrefix(namePart, "..") {
		namePart = filepath.Base(datagridPath)
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

type datagridWalker struct {
	src     []byte
	summary *Summary
}

// walkClassBody scans the datagrid class's top-level statements, tracking
// `private`/`protected`/`public` visibility switches (both the bare-call form
// and the `private def foo; end` single-method form) so only public instance
// methods are reported. Singleton methods (`def self.foo`) are collected
// regardless of visibility -- singleton-method visibility is unconventional and
// rare.
func (w *datagridWalker) walkClassBody(nodes []parser.Node) {
	visibility := "public"
	for _, node := range nodes {
		switch n := node.(type) {
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
					// `private :foo, :bar` (symbol-list form) only toggles
					// visibility of named methods; it is not a datagrid
					// declaration, so drop it rather than surfacing it in the
					// Macros catch-all.
					continue
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
func (w *datagridWalker) handleDef(def *parser.DefNode, visibility string) {
	if _, ok := def.Receiver.(*parser.SelfNode); ok {
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
	if len(args) == 0 {
		return
	}
	name := astutil.ConstantName(w.src, args[0])
	if name == "" {
		return
	}
	w.summary.Concerns = append(w.summary.Concerns, "  "+name)
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
