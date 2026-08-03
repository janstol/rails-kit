package prism

import (
	"bytes"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// Slice returns the source text covered by loc, clamped to src's bounds.
func Slice(src []byte, loc parser.Location) string {
	start, end := loc.StartOffset, loc.StartOffset+loc.Length
	if end > len(src) {
		end = len(src)
	}
	if start < 0 || start > end {
		return ""
	}
	return string(src[start:end])
}

// LineAt returns the 1-based line number of a byte offset within src.
func LineAt(src []byte, offset int) int {
	if offset > len(src) {
		offset = len(src)
	}
	if offset < 0 {
		offset = 0
	}
	return bytes.Count(src[:offset], []byte("\n")) + 1
}

// LineRange returns the (startLine, endLine) of a location.
func LineRange(src []byte, loc parser.Location) (int, int) {
	return LineAt(src, loc.StartOffset), LineAt(src, loc.StartOffset+loc.Length)
}

// BlockStatements returns the top-level statements of a block-like body node
// (a *StatementsNode), or nil if the node is not one. A *BeginNode body (e.g.
// one wrapped in a rescue) yields nil, matching skeleton.rb's body_nodes.
func BlockStatements(n parser.Node) []parser.Node {
	if n == nil {
		return nil
	}
	if st, ok := n.(*parser.StatementsNode); ok {
		return st.Body
	}
	return nil
}

// SymbolValue returns the unescaped value of a *SymbolNode and true; for any
// other node type it returns ("", false).
func SymbolValue(n parser.Node) (string, bool) {
	if s, ok := n.(*parser.SymbolNode); ok {
		return s.Unescaped.Value, true
	}
	return "", false
}

// StringValue returns the unescaped value of a *StringNode (a plain,
// non-interpolated string literal) and true; for any other node type,
// including interpolated strings, it returns ("", false).
func StringValue(n parser.Node) (string, bool) {
	if s, ok := n.(*parser.StringNode); ok {
		return s.Unescaped.Value, true
	}
	return "", false
}

// ArgNodes returns the argument nodes of a call, or nil if it has none.
func ArgNodes(n *parser.CallNode) []parser.Node {
	if n.Arguments == nil {
		return nil
	}
	return n.Arguments.Arguments
}

// KeywordAssocs returns the assoc nodes of a *KeywordHashNode or *HashNode,
// or nil for any other node type.
func KeywordAssocs(n parser.Node) []*parser.AssocNode {
	switch h := n.(type) {
	case *parser.KeywordHashNode:
		return assocSlice(h.Elements)
	case *parser.HashNode:
		return assocSlice(h.Elements)
	}
	return nil
}

func assocSlice(elements []parser.Node) []*parser.AssocNode {
	assocs := make([]*parser.AssocNode, 0, len(elements))
	for _, e := range elements {
		if a, ok := e.(*parser.AssocNode); ok {
			assocs = append(assocs, a)
		}
	}
	return assocs
}