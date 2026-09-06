package astutil

import (
	"sort"

	"github.com/danielgatis/go-ruby-prism/parser"

	"github.com/janstol/rails-kit/internal/prism"
)

// ClassBody holds the per-reader callbacks WalkClassBody dispatches to. A nil
// callback means that node kind is ignored.
type ClassBody struct {
	// Def receives every def in the body along with the visibility in effect
	// ("public"/"private"/"protected"). Readers decide for themselves whether
	// to filter on it.
	Def func(def *parser.DefNode, visibility string)
	// Call receives every receiverless class-level call.
	Call func(call *parser.CallNode)
	// Const receives class-level constant assignments (services only).
	Const func(n *parser.ConstantWriteNode)
	// SkipVisibilityCalls drops the `private :foo, :bar` symbol-list form
	// rather than forwarding it to Call. datagrids sets this so the form does
	// not leak into its Macros catch-all.
	SkipVisibilityCalls bool
}

// WalkClassBody scans a class's or module's top-level statements, tracking
// `private`/`protected`/`public` visibility switches (both the bare-call form
// and the `private def foo; end` single-method form) and dispatching each
// def, receiverless call, and constant assignment to body's callbacks.
func WalkClassBody(nodes []parser.Node, body ClassBody) {
	visibility := "public"
	for _, node := range nodes {
		switch n := node.(type) {
		case *parser.ConstantWriteNode:
			if body.Const != nil {
				body.Const(n)
			}
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
							if body.Def != nil {
								body.Def(def, n.Name)
							}
							continue
						}
					}
					if body.SkipVisibilityCalls {
						// `private :foo, :bar` (symbol-list form) only
						// toggles visibility of named methods; it is not a
						// class-level declaration, so drop it rather than
						// forwarding it to Call.
						continue
					}
				}
			}
			if n.Receiver == nil && body.Call != nil {
				body.Call(n)
			}
		case *parser.DefNode:
			if body.Def != nil {
				body.Def(n, visibility)
			}
		}
	}
}

// CollectCalls walks node's subtree and returns every CallNode matching, in
// source order. Used to find declarations that can appear anywhere inside a
// method body rather than only as its sole statement.
func CollectCalls(node parser.Node, match func(*parser.CallNode) bool) []*parser.CallNode {
	var calls []*parser.CallNode
	var walk func(parser.Node)
	walk = func(n parser.Node) {
		if n == nil {
			return
		}
		if call, ok := n.(*parser.CallNode); ok && match(call) {
			calls = append(calls, call)
		}
		for _, child := range n.CompactChildNodes() {
			walk(child)
		}
	}
	walk(node)
	sort.SliceStable(calls, func(i, j int) bool {
		return calls[i].GetLocation().StartOffset < calls[j].GetLocation().StartOffset
	})
	return calls
}
