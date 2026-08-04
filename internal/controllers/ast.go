package controllers

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"
	"github.com/janstol/rails-kit/internal/astutil"
	"github.com/janstol/rails-kit/internal/config"
	"github.com/janstol/rails-kit/internal/prism"
)

var filterMacros = map[string]bool{
	"before_action": true, "after_action": true, "around_action": true,
	"skip_before_action": true, "skip_after_action": true, "skip_around_action": true,
}

var skippedConcernPrefixes = []string{
	"ActionController", "AbstractController", "ActiveSupport",
}

// Parse reads a controller file, parses it with Prism, and returns its
// structural summary. Prism is error-tolerant: recoverable syntax errors are
// attached to the summary while whatever structure Prism could recover is
// still returned.
func Parse(controllerPath, railsRoot, controllersPath string) (*Summary, error) {
	ctx := context.Background()
	p, err := prism.NewParser(ctx)
	if err != nil {
		return nil, fmt.Errorf("creating prism parser: %w", err)
	}
	defer p.Close(ctx) //nolint:errcheck

	result, src, err := p.Parse(ctx, controllerPath)
	if err != nil {
		return nil, err
	}

	s := summaryForPath(controllerPath, railsRoot, controllersPath)
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

	w := controllerWalker{src: src, summary: s}
	w.walkClassBody(prism.BlockStatements(class.Body))
	return s, nil
}

func summaryForPath(controllerPath, railsRoot, controllersPath string) *Summary {
	s := &Summary{}
	rel, err := filepath.Rel(railsRoot, controllerPath)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		rel = controllerPath
	}
	s.RelPath = filepath.ToSlash(rel)

	controllersDir := config.ResolvePath(railsRoot, controllersPath)
	namePart, err := filepath.Rel(controllersDir, controllerPath)
	if err != nil || strings.HasPrefix(namePart, "..") {
		namePart = filepath.Base(controllerPath)
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

type controllerWalker struct {
	src     []byte
	summary *Summary
}

// walkClassBody scans the controller class's top-level statements, tracking
// `private`/`protected`/`public` visibility switches (both the bare-call form
// and the `private def foo; end` single-method form) so only public methods
// are reported as actions. Every def's body is still scanned for strong
// params regardless of visibility, since permit methods are conventionally
// private.
func (w *controllerWalker) walkClassBody(nodes []parser.Node) {
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
				}
			}
			w.handleCall(n)
		case *parser.DefNode:
			w.handleDef(n, visibility)
		}
	}
}

func (w *controllerWalker) handleDef(def *parser.DefNode, visibility string) {
	if def.Receiver == nil && visibility == "public" {
		w.summary.Actions = append(w.summary.Actions, "  "+def.Name)
	}
	w.collectStrongParams(def)
}

func (w *controllerWalker) handleCall(call *parser.CallNode) {
	if call.Receiver != nil {
		return
	}
	args := prism.ArgNodes(call)
	switch {
	case call.Name == "include":
		w.handleInclude(args)
	case filterMacros[call.Name]:
		w.handleFilter(call, args)
	case call.Name == "rescue_from":
		w.handleRescueFrom(call, args)
	case call.Name == "helper_method":
		w.handleHelperMethod(args)
	case call.Name == "layout":
		w.handleLayout(args)
	case call.Name == "respond_to" && call.Block == nil:
		w.handleRespondTo(args)
	}
}

func (w *controllerWalker) handleInclude(args []parser.Node) {
	if len(args) == 0 {
		return
	}
	name := astutil.ConstantName(w.src, args[0])
	if name == "" {
		return
	}
	for _, prefix := range skippedConcernPrefixes {
		if strings.HasPrefix(name, prefix) {
			return
		}
	}
	w.summary.Concerns = append(w.summary.Concerns, "  "+name)
}

func (w *controllerWalker) handleFilter(call *parser.CallNode, args []parser.Node) {
	var names []string
	var opts []string
	hasBlock := call.Block != nil
	for _, arg := range args {
		if name, ok := prism.SymbolValue(arg); ok {
			names = append(names, ":"+name)
			continue
		}
		if assocs := prism.KeywordAssocs(arg); len(assocs) > 0 {
			opts = append(opts, filterOptions(w.src, assocs)...)
		}
	}
	entry := "  " + call.Name
	if len(names) > 0 {
		entry += " " + strings.Join(names, ", ")
	} else if hasBlock {
		entry += " (block)"
	}
	if len(opts) > 0 {
		entry += ", " + strings.Join(opts, ", ")
	}
	w.summary.Filters = append(w.summary.Filters, entry)
}

// filterOptions extracts only:/except:/if:/unless: from a filter's trailing
// keyword hash, always in that order regardless of source order, so entries
// are deterministic.
func filterOptions(src []byte, assocs []*parser.AssocNode) []string {
	byKey := make(map[string]parser.Node, len(assocs))
	for _, assoc := range assocs {
		if key, ok := prism.SymbolValue(assoc.Key); ok {
			byKey[key] = assoc.Value
		}
	}
	var opts []string
	for _, key := range []string{"only", "except"} {
		if value, ok := byKey[key]; ok {
			opts = append(opts, key+": "+actionListValue(src, value))
		}
	}
	for _, key := range []string{"if", "unless"} {
		if value, ok := byKey[key]; ok {
			opts = append(opts, key+": "+conditionValue(src, value))
		}
	}
	return opts
}

func actionListValue(src []byte, node parser.Node) string {
	if name, ok := prism.SymbolValue(node); ok {
		return ":" + name
	}
	if arr, ok := node.(*parser.ArrayNode); ok {
		var names []string
		for _, el := range arr.Elements {
			if name, ok := prism.SymbolValue(el); ok {
				names = append(names, ":"+name)
			}
		}
		if len(names) > 0 {
			return "[" + strings.Join(names, ", ") + "]"
		}
	}
	return astutil.JoinedSource(src, node.GetLocation())
}

func conditionValue(src []byte, node parser.Node) string {
	if name, ok := prism.SymbolValue(node); ok {
		return ":" + name
	}
	return astutil.JoinedSource(src, node.GetLocation())
}

func (w *controllerWalker) handleRescueFrom(call *parser.CallNode, args []parser.Node) {
	if len(args) == 0 {
		return
	}
	var classes []string
	var withTarget string
	for _, arg := range args {
		if assocs := prism.KeywordAssocs(arg); len(assocs) > 0 {
			for _, assoc := range assocs {
				if key, ok := prism.SymbolValue(assoc.Key); ok && key == "with" {
					if name, ok := prism.SymbolValue(assoc.Value); ok {
						withTarget = name
					}
				}
			}
			continue
		}
		if name := astutil.ConstantName(w.src, arg); name != "" {
			classes = append(classes, name)
		}
	}
	if len(classes) == 0 {
		return
	}
	entry := "  rescue_from " + strings.Join(classes, ", ")
	if withTarget != "" {
		entry += ", with: :" + withTarget
	} else if call.Block != nil {
		entry += " (block)"
	}
	w.summary.RescueFrom = append(w.summary.RescueFrom, entry)
}

func (w *controllerWalker) handleHelperMethod(args []parser.Node) {
	for _, arg := range args {
		if name, ok := prism.SymbolValue(arg); ok {
			w.summary.HelperMethods = append(w.summary.HelperMethods, "  "+name)
		}
	}
}

func (w *controllerWalker) handleLayout(args []parser.Node) {
	if len(args) == 0 {
		return
	}
	switch {
	case astutil.IsFalseNode(args[0]):
		w.summary.Layout = "false"
	default:
		if name, ok := prism.StringValue(args[0]); ok {
			w.summary.Layout = "\"" + name + "\""
			return
		}
		if name, ok := prism.SymbolValue(args[0]); ok {
			w.summary.Layout = ":" + name
			return
		}
		w.summary.Layout = astutil.JoinedSource(w.src, args[0].GetLocation())
	}
}

func (w *controllerWalker) handleRespondTo(args []parser.Node) {
	var names []string
	for _, arg := range args {
		if name, ok := prism.SymbolValue(arg); ok {
			names = append(names, name)
		}
	}
	for _, n := range names {
		w.summary.RespondTo = append(w.summary.RespondTo, "  "+n)
	}
}

// collectStrongParams finds every `params.require(:x).permit(...)` chain
// reachable anywhere in def's body -- not just as its sole statement -- and
// reports it against def's name, in source order.
func (w *controllerWalker) collectStrongParams(def *parser.DefNode) {
	var calls []*parser.CallNode
	var walk func(parser.Node)
	walk = func(node parser.Node) {
		if node == nil {
			return
		}
		if call, ok := node.(*parser.CallNode); ok {
			if _, ok := requirePermitKey(call); ok {
				calls = append(calls, call)
			}
		}
		for _, child := range node.CompactChildNodes() {
			walk(child)
		}
	}
	walk(def.Body)
	sort.SliceStable(calls, func(i, j int) bool {
		return calls[i].GetLocation().StartOffset < calls[j].GetLocation().StartOffset
	})
	for _, call := range calls {
		key, _ := requirePermitKey(call)
		permitArgs := ""
		if call.Arguments != nil {
			permitArgs = astutil.JoinedSource(w.src, call.Arguments.GetLocation())
		}
		entry := fmt.Sprintf("  %s: params.require(:%s).permit(%s)", def.Name, key, permitArgs)
		w.summary.StrongParams = append(w.summary.StrongParams, entry)
	}
}

// requirePermitKey reports whether call is a `params.require(:key).permit(...)`
// chain, and if so, the required key.
func requirePermitKey(call *parser.CallNode) (string, bool) {
	if call.Name != "permit" {
		return "", false
	}
	requireCall, ok := call.Receiver.(*parser.CallNode)
	if !ok || requireCall.Name != "require" {
		return "", false
	}
	paramsCall, ok := requireCall.Receiver.(*parser.CallNode)
	if !ok || paramsCall.Name != "params" || paramsCall.Receiver != nil {
		return "", false
	}
	requireArgs := prism.ArgNodes(requireCall)
	if len(requireArgs) != 1 {
		return "", false
	}
	return prism.SymbolValue(requireArgs[0])
}
