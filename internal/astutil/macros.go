package astutil

import (
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"

	"github.com/janstol/rails-kit/internal/prism"
)

// IncludedConcern returns the included module's name from an `include X`
// call's args, or "" when there are no args or the name starts with one of
// skippedPrefixes (the framework bases a reader suppresses). When the first
// arg is not a plain constant -- a dotted chain like
// `include Rails.application.routes.url_helpers` is a CallNode, not a
// ConstantReadNode/ConstantPathNode -- the joined source of the arg is used
// instead of dropping it silently. Callers apply their own indent.
func IncludedConcern(src []byte, args []parser.Node, skippedPrefixes []string) string {
	if len(args) == 0 {
		return ""
	}
	name := ConstantName(src, args[0])
	if name == "" {
		name = JoinedSource(src, args[0].GetLocation())
	}
	for _, prefix := range skippedPrefixes {
		if strings.HasPrefix(name, prefix) {
			return ""
		}
	}
	return name
}

// LayoutValue renders a `layout` declaration's argument: `false` for the
// opt-out, a quoted string, a `:symbol`, or the joined source. Returns "" when
// there are no args.
func LayoutValue(src []byte, args []parser.Node) string {
	if len(args) == 0 {
		return ""
	}
	if IsFalseNode(args[0]) {
		return "false"
	}
	if name, ok := prism.StringValue(args[0]); ok {
		return "\"" + name + "\""
	}
	if name, ok := prism.SymbolValue(args[0]); ok {
		return ":" + name
	}
	return JoinedSource(src, args[0].GetLocation())
}

// AssocsBySymbolKey indexes a trailing keyword hash's assocs by their symbol
// key, so callers can emit options in a fixed order regardless of source order.
func AssocsBySymbolKey(assocs []*parser.AssocNode) map[string]parser.Node {
	byKey := make(map[string]parser.Node, len(assocs))
	for _, assoc := range assocs {
		if key, ok := prism.SymbolValue(assoc.Key); ok {
			byKey[key] = assoc.Value
		}
	}
	return byKey
}
