package routes

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"
	"github.com/janstol/rails-kit/internal/prism"
)

// This file is the Prism AST driver for `routes --static`. It replaces the
// hand-rolled regex line scanner that previously lived in static.go. The
// semantic resolution core (resolveVerbDeclaration, expandResources,
// resolveMountDeclaration, applyResourceScopePath, etc.) is unchanged; this
// driver only builds the existing intermediate structs (verbDeclaration,
// scopeDeclaration, mountDeclaration, resourceOptions, pathConstraint,
// redirectDeclaration) from AST nodes and feeds them to those resolvers.
//
// Block recursion replaces the scanner's depth/stack discipline: each
// block-opening handler (namespace, resources, scope, ...) recurses into the
// block body with a derived routeFrame. `do...end` and `{...}` blocks are both
// *parser.BlockNode; the inline `{...}` form of namespace/member/collection
// gets a support check (single recognized route call, no nested block)
// matching the old parseInlineRouteBlock path, while `do` blocks walk every
// statement.

// routeDSLNames is the set of call names treated as a supported single
// statement inside an inline (`{...}`) namespace/member/collection block.
var routeDSLNames = map[string]bool{
	"get": true, "post": true, "put": true, "patch": true, "delete": true, "match": true,
	"resources": true, "resource": true, "root": true, "mount": true, "concerns": true, "draw": true,
}

// parseFileAST parses a single routes file via the Prism AST and walks it with
// the inherited frame. It is the AST counterpart to the old parseFile. The
// canonical path is registered in p.active for the draw cycle guard.
func (p *staticParser) parseFileAST(routesPath string, initialFrame routeFrame) (StaticResult, error) {
	canonicalPath, err := filepath.Abs(routesPath)
	if err != nil {
		return StaticResult{}, fmt.Errorf("resolving route file: %w", err)
	}
	if resolved, resolveErr := filepath.EvalSymlinks(canonicalPath); resolveErr == nil {
		canonicalPath = resolved
	}
	p.active[canonicalPath] = true
	defer delete(p.active, canonicalPath)

	result, src, err := p.prismParser.Parse(p.ctx, routesPath)
	if err != nil {
		return StaticResult{}, err
	}
	if result.Value == nil || result.Value.Statements == nil {
		return StaticResult{}, nil
	}
	return p.walkStatements(routesPath, src, result.Value.Statements.Body, initialFrame, initialFrame, result.Errors), nil
}

// walkStatements dispatches a list of AST statements under one frame,
// accumulating entries and warnings. outerFrame is the frame one level above
// the block being walked; when a child returns a result with corrupt set (a
// malformed-namespace frame-stack bug, see StaticResult.corrupt), the
// remaining siblings are walked at outerFrame. errs are the file's parse
// errors, carried along for the unterminated-verb detection.
func (p *staticParser) walkStatements(routesPath string, src []byte, nodes []parser.Node, frame, outerFrame routeFrame, errs []parser.ParseError) StaticResult {
	var result StaticResult
	current := frame
	for _, node := range nodes {
		var child StaticResult
		switch n := node.(type) {
		case *parser.IfNode:
			child = p.handleConditional(routesPath, src, n, current, outerFrame, errs)
		case *parser.UnlessNode:
			child = p.handleConditional(routesPath, src, unlessAsIf(n), current, outerFrame, errs)
		case *parser.CallNode:
			child = p.handleCall(routesPath, src, n, current, outerFrame, errs)
		default:
			// Unknown top-level construct: ignore.
		}
		result.append(child)
		if child.corrupt {
			// The enclosing block's frame has been "popped" for the remaining
			// siblings; continue at the outer frame.
			current = outerFrame
		}
	}
	return result
}

func (s *StaticResult) append(other StaticResult) {
	s.Entries = append(s.Entries, other.Entries...)
	s.Warnings = append(s.Warnings, other.Warnings...)
}

// unlessAsIf adapts an UnlessNode to the IfNode shape used by handleConditional.
func unlessAsIf(n *parser.UnlessNode) *parser.IfNode {
	return &parser.IfNode{
		Location:     n.Location,
		IfKeywordLoc: &n.KeywordLoc,
		Predicate:    n.Predicate,
		Statements:   n.Statements,
	}
}

// handleConditional processes a postfix `if`/`unless` modifier (which Prism
// represents as an IfNode wrapping the modified call) and block if/unless.
//
// A postfix modifier has the keyword on the same line as its single
// statement. For a mount modifier with a same-line predicate, the mount is
// processed and a "conditional mount" warning is appended. When the `if`/`
// `unless` had no condition on its line, Prism recovers by absorbing the next
// statement as the predicate (its line is later than the keyword's); that is
// the "malformed postfix condition" case: the mount itself is skipped and the
// absorbed statement is walked normally. Any other call under a modifier is
// processed with the condition ignored, matching the old line scanner, which
// only ever surfaced postfix conditions for mount.
func (p *staticParser) handleConditional(routesPath string, src []byte, n *parser.IfNode, frame, outerFrame routeFrame, errs []parser.ParseError) StaticResult {
	stmts := prism.BlockStatements(n.Statements)
	if len(stmts) != 1 {
		// block if/unless: walk the body, ignoring the condition
		return p.walkStatements(routesPath, src, stmts, frame, outerFrame, errs)
	}
	stmt := stmts[0]
	stmtLine := prism.LineAt(src, stmt.GetLocation().StartOffset)
	keywordLine := stmtLine
	if n.IfKeywordLoc != nil {
		keywordLine = prism.LineAt(src, n.IfKeywordLoc.StartOffset)
	}
	postfix := keywordLine == stmtLine
	predLine := prism.LineAt(src, n.Predicate.GetLocation().StartOffset)
	malformed := postfix && predLine > keywordLine

	if call, ok := stmt.(*parser.CallNode); ok && call.Name == "mount" {
		if malformed {
			result := StaticResult{Warnings: []StaticWarning{staticWarning(routesPath, keywordLine, "mount has a malformed postfix condition: "+firstLine(src, n.Location))}}
			// The absorbed predicate is the next statement; walk it normally.
			result.append(p.walkOne(routesPath, src, n.Predicate, frame, outerFrame, errs))
			return result
		}
		if postfix {
			res := p.handleMount(routesPath, src, call, frame)
			res.Warnings = append(res.Warnings, staticWarning(routesPath, keywordLine, "conditional mount is included without evaluating its condition"))
			return res
		}
	}

	// Non-mount modifier (or a block if with a single statement): walk the
	// statement, ignoring the condition. If the modifier was malformed, also
	// walk the absorbed predicate so the next line is not lost. The statement's
	// own corrupt flag (e.g. a postfix-modified malformed namespace) propagates
	// to the caller via the direct assignment.
	result := p.walkOne(routesPath, src, stmt, frame, outerFrame, errs)
	if malformed {
		result.append(p.walkOne(routesPath, src, n.Predicate, frame, outerFrame, errs))
	}
	return result
}

// handleCall dispatches a single route-DSL call node.
func (p *staticParser) handleCall(routesPath string, src []byte, call *parser.CallNode, frame, outerFrame routeFrame, errs []parser.ParseError) StaticResult {
	switch call.Name {
	case "namespace":
		return p.handleNamespace(routesPath, src, call, frame, outerFrame, errs)
	case "member", "collection":
		return p.handleMemberCollection(routesPath, src, call, frame, outerFrame, errs)
	case "resources", "resource":
		return p.handleResources(routesPath, src, call, frame, outerFrame, errs)
	case "scope":
		return p.handleScope(routesPath, src, call, frame, outerFrame, errs)
	case "controller":
		return p.handleController(routesPath, src, call, frame, outerFrame, errs)
	case "root":
		return p.handleRoot(routesPath, src, call, frame)
	case "mount":
		return p.handleMount(routesPath, src, call, frame)
	case "draw":
		return p.handleDraw(routesPath, src, call, frame, outerFrame, errs)
	case "concern":
		return p.handleConcern(routesPath, src, call, frame)
	case "concerns":
		return p.handleConcerns(routesPath, src, call, frame, outerFrame)
	case "get", "post", "put", "patch", "delete", "match":
		return p.handleVerb(routesPath, src, call, frame, errs)
	default:
		if isUnsupportedDSL(call.Name) {
			line := prism.LineAt(src, call.Location.StartOffset)
			warning := staticWarning(routesPath, line, "unsupported route DSL: "+firstLine(src, call.Location))
			// The old scanner emitted the warning and, for a `do` block, still
			// walked the inner lines at the current frame (depth++ with no frame
			// push). Reproduce that so routes nested in e.g. `devise_scope` are
			// not lost.
			if bn, ok := call.Block.(*parser.BlockNode); ok && blockKind(src, call.Block) == "do" {
				inner := p.walkStatements(routesPath, src, prism.BlockStatements(bn.Body), frame, outerFrame, errs)
				inner.Warnings = append([]StaticWarning{warning}, inner.Warnings...)
				return inner
			}
			return StaticResult{Warnings: []StaticWarning{warning}}
		}
		// Unrecognized call: walk any block at the current frame so nested
		// routes are not lost, matching the old scanner's fall-through.
		if call.Block != nil {
			if bn, ok := call.Block.(*parser.BlockNode); ok {
				return p.walkStatements(routesPath, src, prism.BlockStatements(bn.Body), frame, outerFrame, errs)
			}
		}
		return StaticResult{}
	}
}

func isUnsupportedDSL(name string) bool {
	if name == "constraints" || name == "constraint" || name == "direct" || name == "resolve" {
		return true
	}
	return strings.HasPrefix(name, "devise_")
}

// blockKind reports the block's opening form: "brace" for `{...}`, "do" for
// `do...end`, "" for no block.
func blockKind(src []byte, block parser.Node) string {
	bn, ok := block.(*parser.BlockNode)
	if !ok {
		return ""
	}
	if prism.Slice(src, bn.OpeningLoc) == "{" {
		return "brace"
	}
	return "do"
}

// braceMalformed reports whether a brace block failed to close (the closing
// `}` is absent), which Prism signals with a zero-length ClosingLoc.
func braceMalformed(src []byte, block parser.Node) bool {
	bn, ok := block.(*parser.BlockNode)
	if !ok {
		return false
	}
	return blockKind(src, block) == "brace" && bn.ClosingLoc.Length == 0
}

// firstLine returns the trimmed first source line covered by loc.
func firstLine(src []byte, loc parser.Location) string {
	s := prism.Slice(src, loc)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}

// scalar returns the normalized scalar value of a node, matching the old
// normalizeOptionScalar applied to the node's source slice.
func scalar(src []byte, n parser.Node) string {
	if n == nil {
		return ""
	}
	return normalizeOptionScalar(prism.Slice(src, n.GetLocation()))
}

// stringOrSymbol returns the unescaped value of a plain StringNode or
// SymbolNode and true; false for anything else (interpolated strings, calls,
// variables, nil).
func stringOrSymbol(n parser.Node) (string, bool) {
	if v, ok := prism.StringValue(n); ok {
		return v, true
	}
	if v, ok := prism.SymbolValue(n); ok {
		return v, true
	}
	return "", false
}

// routePathValue extracts the verb route path from a node, returning the
// path string and whether it was a symbol. Plain strings and symbols use their
// unescaped values; interpolated strings keep their source text (minus the
// surrounding quotes) so `#{...}` segments round-trip, matching the old
// normalizeOptionScalar behavior.
func routePathValue(src []byte, n parser.Node) (string, bool, bool) {
	if v, ok := prism.StringValue(n); ok {
		return v, false, true
	}
	if v, ok := prism.SymbolValue(n); ok {
		return v, true, true
	}
	if _, ok := n.(*parser.InterpolatedStringNode); ok {
		return normalizeOptionScalar(prism.Slice(src, n.GetLocation())), false, true
	}
	return "", false, false
}

// assocValue finds the value node for a keyword in a list of assocs.
func assocValue(assocs []*parser.AssocNode, key string) parser.Node {
	for _, a := range assocs {
		if k, ok := prism.SymbolValue(a.Key); ok && k == key {
			return a.Value
		}
	}
	return nil
}

// handleNamespace handles `namespace :name do ... end` and the inline
// `namespace(:name) { ... }` brace form.
func (p *staticParser) handleNamespace(routesPath string, src []byte, call *parser.CallNode, frame, outerFrame routeFrame, errs []parser.ParseError) StaticResult {
	args := prism.ArgNodes(call)
	line := prism.LineAt(src, call.Location.StartOffset)

	if call.Block == nil {
		return StaticResult{}
	}
	kind := blockKind(src, call.Block)
	if kind == "brace" {
		if braceMalformed(src, call.Block) {
			return p.malformedInlineBlock(routesPath, src, call, "namespace", frame, outerFrame, errs)
		}
		// Inline namespace: require a single symbol arg and a supported body.
		name, ok := symName(args)
		if !ok {
			return StaticResult{Warnings: []StaticWarning{staticWarning(routesPath, line, "dynamic inline namespace block is not modeled")}}
		}
		body := prism.BlockStatements(call.Block.(*parser.BlockNode).Body)
		if !supportedInlineBody(body) {
			return StaticResult{Warnings: []StaticWarning{staticWarning(routesPath, line, "dynamic inline namespace block is not modeled")}}
		}
		return p.walkStatements(routesPath, src, body, namespaceRouteFrame(frame, name), frame, errs)
	}
	// do block. The old regex scanner captured only the leading `:name` and
	// ignored any trailing options, so `namespace :admin, only: [] do` still
	// pushed the admin namespace. A `{...}` hash literal in the args, however,
	// was mistaken for an inline brace block: the scanner emitted "malformed
	// inline namespace block", did not push, and the `do`'s matching `end` then
	// popped the enclosing namespace's frame too. We mark this result corrupt so
	// the enclosing block walks the namespace's subsequent siblings at the outer
	// frame, reproducing that frame-stack bug for byte-identical output.
	name, isSym := namespaceName(args)
	if !isSym {
		// Non-symbol namespace name (string, dynamic): walk the body at the
		// current frame, matching the old scanner's fall-through.
		return p.walkBlock(routesPath, src, call.Block, frame, outerFrame, errs)
	}
	if namespaceArgsHaveHashLiteral(args) {
		result := StaticResult{Warnings: []StaticWarning{staticWarning(routesPath, line, "malformed inline namespace block")}}
		result.append(p.walkBlock(routesPath, src, call.Block, frame, outerFrame, errs))
		result.corrupt = true
		return result
	}
	return p.walkBlock(routesPath, src, call.Block, namespaceRouteFrame(frame, name), frame, errs)
}

// namespaceName returns the symbol name from the first arg when it is a
// *SymbolNode, regardless of any additional option args (`namespace :admin,
// only: [] do`). The old regex captured only this leading name.
func namespaceName(args []parser.Node) (string, bool) {
	if len(args) == 0 {
		return "", false
	}
	return prism.SymbolValue(args[0])
}

// namespaceArgsHaveHashLiteral reports whether the namespace args contain a
// `{...}` hash literal — as a bare arg or as a keyword option's value. The old
// regex scanner mistook such a literal for an inline brace block.
func namespaceArgsHaveHashLiteral(args []parser.Node) bool {
	for _, a := range args {
		if _, ok := a.(*parser.HashNode); ok {
			return true
		}
		for _, assoc := range prism.KeywordAssocs(a) {
			if _, ok := assoc.Value.(*parser.HashNode); ok {
				return true
			}
		}
	}
	return false
}

// symName returns the single symbol arg name from args (a lone *SymbolNode).
func symName(args []parser.Node) (string, bool) {
	if len(args) != 1 {
		return "", false
	}
	return prism.SymbolValue(args[0])
}

// supportedInlineBody reports whether an inline block body is exactly one
// recognized route-DSL call with no nested block.
func supportedInlineBody(body []parser.Node) bool {
	if len(body) != 1 {
		return false
	}
	call, ok := body[0].(*parser.CallNode)
	if !ok {
		return false
	}
	return routeDSLNames[call.Name] && call.Block == nil
}

// malformedInlineBlock handles an unterminated inline `{...}` block. The old
// scanner emitted "malformed inline <kind> block" for the line and then
// processed the following lines at the current frame. Prism absorbs the
// following lines into the unterminated block body, so we walk only the body
// statements on lines after the block's opening line (the first statement is
// on the malformed line itself and is skipped).
func (p *staticParser) malformedInlineBlock(routesPath string, src []byte, call *parser.CallNode, kind string, frame, outerFrame routeFrame, errs []parser.ParseError) StaticResult {
	opening := prism.LineAt(src, call.Location.StartOffset)
	var result StaticResult
	result.Warnings = append(result.Warnings, staticWarning(routesPath, opening, "malformed inline "+kind+" block"))
	bn := call.Block.(*parser.BlockNode)
	for _, stmt := range prism.BlockStatements(bn.Body) {
		if _, missing := stmt.(*parser.MissingNode); missing {
			continue
		}
		if prism.LineAt(src, stmt.GetLocation().StartOffset) <= opening {
			continue
		}
		result.append(p.walkOne(routesPath, src, stmt, frame, outerFrame, errs))
	}
	return result
}

// walkOne dispatches a single node (used by malformedInlineBlock).
func (p *staticParser) walkOne(routesPath string, src []byte, node parser.Node, frame, outerFrame routeFrame, errs []parser.ParseError) StaticResult {
	if call, ok := node.(*parser.CallNode); ok {
		return p.handleCall(routesPath, src, call, frame, outerFrame, errs)
	}
	if ifn, ok := node.(*parser.IfNode); ok {
		return p.handleConditional(routesPath, src, ifn, frame, outerFrame, errs)
	}
	if un, ok := node.(*parser.UnlessNode); ok {
		return p.handleConditional(routesPath, src, unlessAsIf(un), frame, outerFrame, errs)
	}
	return StaticResult{}
}

// walkBlock walks a block body under frame, threading outerFrame for the
// frame-corruption reproduction (see walkStatements).
func (p *staticParser) walkBlock(routesPath string, src []byte, block parser.Node, frame, outerFrame routeFrame, errs []parser.ParseError) StaticResult {
	bn, ok := block.(*parser.BlockNode)
	if !ok {
		return StaticResult{}
	}
	return p.walkStatements(routesPath, src, prism.BlockStatements(bn.Body), frame, outerFrame, errs)
}

// handleMemberCollection handles `member do`/`collection do` and the inline
// `member { ... }`/`collection { ... }` brace forms.
func (p *staticParser) handleMemberCollection(routesPath string, src []byte, call *parser.CallNode, frame, outerFrame routeFrame, errs []parser.ParseError) StaticResult {
	kind := call.Name
	line := prism.LineAt(src, call.Location.StartOffset)
	args := prism.ArgNodes(call)

	if call.Block == nil {
		return StaticResult{}
	}
	if blockKind(src, call.Block) == "brace" {
		if braceMalformed(src, call.Block) {
			return p.malformedInlineBlock(routesPath, src, call, kind, frame, outerFrame, errs)
		}
		if len(args) > 0 {
			return StaticResult{Warnings: []StaticWarning{staticWarning(routesPath, line, "dynamic inline "+kind+" block is not modeled")}}
		}
		if !frameHasScope(frame, kind) {
			return StaticResult{Warnings: []StaticWarning{staticWarning(routesPath, line, kind+" block outside a resource was skipped")}}
		}
		body := prism.BlockStatements(call.Block.(*parser.BlockNode).Body)
		if !supportedInlineBody(body) {
			return StaticResult{Warnings: []StaticWarning{staticWarning(routesPath, line, "dynamic inline "+kind+" block is not modeled")}}
		}
		return p.walkStatements(routesPath, src, body, memberCollectionFrame(frame, kind), frame, errs)
	}
	// do block
	if len(args) > 0 {
		return p.walkBlock(routesPath, src, call.Block, frame, outerFrame, errs)
	}
	if !frameHasScope(frame, kind) {
		return StaticResult{Warnings: []StaticWarning{staticWarning(routesPath, line, kind+" block outside a resource was skipped")}}
	}
	return p.walkBlock(routesPath, src, call.Block, memberCollectionFrame(frame, kind), frame, errs)
}

func frameHasScope(frame routeFrame, kind string) bool {
	if kind == "member" {
		return frame.memberPath != ""
	}
	return frame.collectionPath != ""
}

func memberCollectionFrame(frame routeFrame, kind string) routeFrame {
	if kind == "member" {
		frame.pathPrefix = frame.memberPath
	} else {
		frame.pathPrefix = frame.collectionPath
	}
	frame.routeMode = kind
	return frame
}

// handleResources handles `resources`/`resource` declarations and their
// nested `do...end` block.
func (p *staticParser) handleResources(routesPath string, src []byte, call *parser.CallNode, frame, outerFrame routeFrame, errs []parser.ParseError) StaticResult {
	args := prism.ArgNodes(call)
	if len(args) == 0 {
		return StaticResult{}
	}
	name, ok := prism.SymbolValue(args[0])
	if !ok {
		return StaticResult{}
	}
	singularResource := call.Name == "resource"
	line := prism.LineAt(src, call.Location.StartOffset)
	cf := frame

	options := parseResourceOptionsAST(src, args, singularResource)

	pathSegment := name
	if options.hasPath {
		pathSegment = options.path
	}
	collectionPath := joinRoutePath(cf.pathPrefix, pathSegment)
	resourceController := name
	if options.controller != "" {
		resourceController = options.controller
	}
	resourceController = qualifyController(cf.controllerPrefix, resourceController)

	helperBase := name
	if options.helper != "" {
		helperBase = options.helper
	}
	collectionHelper := cf.helperPrefix + helperBase
	memberHelper := cf.helperPrefix + p.pluralizer.Singularize(helperBase)
	param := options.param
	if param == "" {
		param = "id"
	}

	var result StaticResult
	result.Entries = append(result.Entries, expandResources(
		resourceController,
		collectionPath,
		collectionHelper,
		memberHelper,
		param,
		options.actions,
		singularResource,
	)...)

	singularName := p.pluralizer.Singularize(name)
	memberPath := collectionPath
	nestedPath := collectionPath
	if !singularResource {
		memberPath = joinRoutePath(collectionPath, ":"+param)
		nestedPath = joinRoutePath(collectionPath, ":"+singularName+"_"+param)
	}
	blockFrame := cf
	blockFrame.pathPrefix = nestedPath
	blockFrame.helperPrefix = memberHelper + "_"
	blockFrame.collectionPath = collectionPath
	blockFrame.memberPath = memberPath
	blockFrame.resourcePath = nestedPath
	blockFrame.resourceName = helperBase
	blockFrame.resourceSingular = p.pluralizer.Singularize(helperBase)
	blockFrame.resourceParam = param
	blockFrame.resourceController = resourceController
	blockFrame.collectionHelper = collectionHelper
	blockFrame.memberHelper = memberHelper
	blockFrame.routeMode = "resource"
	blockFrame.defaultController = resourceController

	if len(options.concerns) > 0 {
		result.append(p.expandConcerns(options.concerns, routesPath, line, blockFrame, frame))
	}

	if blockKind(src, call.Block) == "do" {
		result.append(p.walkBlock(routesPath, src, call.Block, blockFrame, frame, errs))
	}
	return result
}

// parseResourceOptionsAST builds a resourceOptions from a resources call's
// args, mirroring the old parseResourceOptions.
func parseResourceOptionsAST(src []byte, args []parser.Node, singular bool) resourceOptions {
	options := resourceOptions{actions: allResourceActions(singular)}
	var assocs []*parser.AssocNode
	if len(args) >= 2 {
		assocs = prism.KeywordAssocs(args[1])
	}
	if only := assocValue(assocs, "only"); only != nil {
		if actions := symbolList(only); actions != nil {
			options.actions = actionSet(actions)
		}
	} else if except := assocValue(assocs, "except"); except != nil {
		if excluded := symbolList(except); excluded != nil {
			for _, action := range excluded {
				delete(options.actions, action)
			}
		}
	}
	if v := assocValue(assocs, "path"); v != nil {
		// A present path option overrides the segment even when empty
		// (`path: ''` collapses the resource name out of the URL).
		options.path = scalar(src, v)
		options.hasPath = true
	}
	if v := assocValue(assocs, "controller"); v != nil {
		options.controller = scalar(src, v)
	}
	if v := assocValue(assocs, "as"); v != nil {
		options.helper = scalar(src, v)
	}
	if v := assocValue(assocs, "param"); v != nil {
		options.param = scalar(src, v)
	}
	if v := assocValue(assocs, "concerns"); v != nil {
		options.concerns = symbolList(v)
	}
	return options
}

// symbolList extracts a list of names from an option value node: an ArrayNode
// of symbols/strings, or a single symbol/string. A nil result means absent.
func symbolList(n parser.Node) []string {
	if n == nil {
		return nil
	}
	if arr, ok := n.(*parser.ArrayNode); ok {
		out := make([]string, 0, len(arr.Elements))
		for _, e := range arr.Elements {
			if v, ok := stringOrSymbol(e); ok {
				out = append(out, v)
			}
		}
		return out
	}
	if v, ok := stringOrSymbol(n); ok {
		return []string{v}
	}
	return nil
}

func actionSet(actions []string) map[string]bool {
	set := make(map[string]bool, len(actions))
	for _, a := range actions {
		set[a] = true
	}
	return set
}

// handleScope handles `scope` blocks. Only `do...end` is modeled; a brace or
// missing block yields "scope requires a single-line block declaration".
func (p *staticParser) handleScope(routesPath string, src []byte, call *parser.CallNode, frame, outerFrame routeFrame, errs []parser.ParseError) StaticResult {
	line := prism.LineAt(src, call.Location.StartOffset)
	if blockKind(src, call.Block) != "do" {
		return StaticResult{Warnings: []StaticWarning{staticWarning(routesPath, line, "scope requires a single-line block declaration: "+firstLine(src, call.Location))}}
	}
	args := prism.ArgNodes(call)
	declaration, ok := parseScopeDeclarationAST(src, call, args, line)
	if !ok {
		return StaticResult{Warnings: []StaticWarning{staticWarning(routesPath, line, declaration.err)}}
	}
	cf := frame
	f := cf
	if declaration.hasPath {
		if cf.resourceController != "" {
			applyResourceScopePath(&f, declaration.path)
		} else {
			f.pathPrefix = joinRoutePath(cf.pathPrefix, declaration.path)
		}
	}
	if declaration.hasModule {
		f.controllerPrefix = cf.controllerPrefix + strings.Trim(declaration.module, "/") + "/"
	}
	if declaration.hasHelper {
		if cf.resourceController != "" {
			applyResourceScopeHelper(&f, declaration.helper)
		} else {
			f.helperPrefix = cf.helperPrefix + declaration.helper + "_"
		}
	}
	if declaration.hasController {
		f.defaultController = qualifyController(f.controllerPrefix, declaration.controller)
	} else if declaration.hasModule && f.resourceController == "" {
		f.defaultController = strings.TrimSuffix(f.controllerPrefix, "/")
	}
	var result StaticResult
	if declaration.unsupportedOptions {
		result.Warnings = append(result.Warnings, staticWarning(routesPath, line, "scope options are only partially modeled"))
	}
	result.append(p.walkBlock(routesPath, src, call.Block, f, frame, errs))
	return result
}

type scopeDecl struct {
	path, module, helper, controller                                 string
	hasPath, hasModule, hasHelper, hasController, unsupportedOptions bool
	err                                                              string
}

// parseScopeDeclarationAST builds a scope declaration from the call args.
// Returns ok=false with decl.err set on the dynamic/conflict errors that the
// old parser surfaced (and which caused the block to be skipped).
func parseScopeDeclarationAST(src []byte, call *parser.CallNode, args []parser.Node, line int) (scopeDecl, bool) {
	var d scopeDecl
	var optionAssocs []*parser.AssocNode
	if len(args) >= 1 {
		if assocs := prism.KeywordAssocs(args[0]); assocs != nil {
			optionAssocs = assocs
		} else {
			v, literal := scopeScalar(src, args[0])
			if !literal {
				d.err = "dynamic scope path is not modeled: " + firstLine(src, call.Location)
				return d, false
			}
			d.path, d.hasPath = v, true
			if len(args) >= 2 {
				optionAssocs = prism.KeywordAssocs(args[1])
			}
		}
	}
	for _, a := range optionAssocs {
		key, _ := prism.SymbolValue(a.Key)
		switch key {
		case "path":
			if d.hasPath {
				d.err = "scope has conflicting positional and path options: " + firstLine(src, call.Location)
				return d, false
			}
			v, literal := scopeScalar(src, a.Value)
			if !literal {
				d.err = "dynamic scope path is not modeled: " + firstLine(src, call.Location)
				return d, false
			}
			d.path, d.hasPath = v, true
		case "module":
			v, literal := scopeScalar(src, a.Value)
			if !literal {
				d.err = "dynamic scope module is not modeled: " + firstLine(src, call.Location)
				return d, false
			}
			d.module, d.hasModule = v, true
		case "as":
			v, literal := scopeScalar(src, a.Value)
			if !literal {
				d.err = "dynamic scope as is not modeled: " + firstLine(src, call.Location)
				return d, false
			}
			d.helper, d.hasHelper = v, true
		case "controller":
			v, literal := scopeScalar(src, a.Value)
			if !literal {
				d.err = "dynamic scope controller is not modeled: " + firstLine(src, call.Location)
				return d, false
			}
			d.controller, d.hasController = v, true
		default:
			d.unsupportedOptions = true
		}
	}
	return d, true
}

// scopeScalar accepts a plain string or symbol scope value; interpolated
// strings and anything else are not literal.
func scopeScalar(src []byte, n parser.Node) (string, bool) {
	if v, ok := prism.StringValue(n); ok {
		return v, true
	}
	if v, ok := prism.SymbolValue(n); ok {
		return v, true
	}
	return "", false
}

// handleController handles `controller :name do ... end`.
func (p *staticParser) handleController(routesPath string, src []byte, call *parser.CallNode, frame, outerFrame routeFrame, errs []parser.ParseError) StaticResult {
	line := prism.LineAt(src, call.Location.StartOffset)
	if blockKind(src, call.Block) != "do" {
		return StaticResult{Warnings: []StaticWarning{staticWarning(routesPath, line, "controller requires a single-line block declaration: "+firstLine(src, call.Location))}}
	}
	args := prism.ArgNodes(call)
	if len(args) != 1 {
		return StaticResult{Warnings: []StaticWarning{staticWarning(routesPath, line, "dynamic controller block is not modeled: "+firstLine(src, call.Location))}}
	}
	controller, literal := scopeScalar(src, args[0])
	if !literal || controller == "" {
		return StaticResult{Warnings: []StaticWarning{staticWarning(routesPath, line, "dynamic controller block is not modeled: "+firstLine(src, call.Location))}}
	}
	cf := frame
	f := cf
	f.defaultController = qualifyController(cf.controllerPrefix, controller)
	if cf.resourceController != "" {
		f.resourceController = f.defaultController
	}
	return p.walkBlock(routesPath, src, call.Block, f, frame, errs)
}

// handleRoot handles `root "c#a"` and `root to: "c#a"`.
func (p *staticParser) handleRoot(routesPath string, src []byte, call *parser.CallNode, frame routeFrame) StaticResult {
	args := prism.ArgNodes(call)
	var action string
	if len(args) >= 1 {
		if assocs := prism.KeywordAssocs(args[0]); assocs != nil {
			if v := assocValue(assocs, "to"); v != nil {
				action, _ = stringOrSymbol(v)
			}
		} else if v, ok := stringOrSymbol(args[0]); ok {
			action = v
		}
	}
	if action == "" {
		return StaticResult{}
	}
	parts := strings.SplitN(action, "#", 2)
	if len(parts) != 2 {
		return StaticResult{}
	}
	cf := frame
	return StaticResult{Entries: []RouteEntry{{
		Prefix:           cf.helperPrefix + "root",
		Verb:             "GET",
		URIPattern:       rootRoutePath(cf.pathPrefix),
		ControllerAction: qualifyController(cf.controllerPrefix, parts[0]) + "#" + parts[1],
	}}}
}

// handleMount handles `mount Target => "/path"`, `mount Target, at: "/path",
// as: :name`, postfix `if`/`unless` modifiers (delegated by
// handleConditional), and the dynamic/malformed cases.
func (p *staticParser) handleMount(routesPath string, src []byte, call *parser.CallNode, frame routeFrame) StaticResult {
	line := prism.LineAt(src, call.Location.StartOffset)
	args := prism.ArgNodes(call)

	var targetNode, pathNode, asNode parser.Node
	if len(args) > 0 {
		if assocs := prism.KeywordAssocs(args[0]); len(assocs) >= 1 {
			// Hash-rocket form: Target => "/path" [, as: :name]
			targetNode = assocs[0].Key
			pathNode = assocs[0].Value
			if len(assocs) > 1 {
				asNode = assocValue(assocs[1:], "as")
			}
		} else {
			targetNode = args[0]
			if len(args) >= 2 {
				assocs := prism.KeywordAssocs(args[1])
				pathNode = assocValue(assocs, "at")
				asNode = assocValue(assocs, "as")
				if len(assocs) > 0 {
					for _, a := range assocs {
						if k, _ := prism.SymbolValue(a.Key); k != "at" && k != "as" {
							return p.mountError(routesPath, line, "mount options are only partially modeled", call)
						}
					}
				}
			}
		}
	}

	target, receiverTarget, dynamic := classifyMountTarget(src, targetNode)
	if dynamic {
		return StaticResult{Warnings: []StaticWarning{staticWarning(routesPath, line, "dynamic mount target is not modeled: "+firstLine(src, call.Location))}}
	}
	// Mount does not span continuation lines: only an `at` value on the same
	// line as the call counts. A path on a later line (e.g. `mount X,\n  at:
	// "/y"`) is treated as absent, matching the old line scanner.
	path, pathOK := mountPath(src, pathNode, line, call)
	if !pathOK {
		return StaticResult{Warnings: []StaticWarning{staticWarning(routesPath, line, "mount route requires a static at option: "+firstLine(src, call.Location))}}
	}

	declaration := mountDeclaration{
		target:         target,
		path:           path,
		receiverTarget: receiverTarget,
	}
	if asNode != nil {
		declaration.hasHelper = true
		if _, ok := asNode.(*parser.NilNode); ok {
			// nil => no helper
		} else if v, ok := prism.SymbolValue(asNode); ok {
			declaration.helper = v
		} else if v, ok := prism.StringValue(asNode); ok {
			declaration.helper = v
		} else {
			return p.mountError(routesPath, line, "mount options are only partially modeled", call)
		}
	}
	entry := resolveMountDeclaration(declaration, frame)
	return StaticResult{Entries: []RouteEntry{entry}}
}

func (p *staticParser) mountError(routesPath string, line int, msg string, call *parser.CallNode) StaticResult {
	// An unsupported-options mount still emits its entry first, then the
	// warning, matching the old ordering. Re-resolve is avoided by returning
	// only the warning here; the entry path is not modeled for these cases.
	return StaticResult{Warnings: []StaticWarning{staticWarning(routesPath, line, msg)}}
}

// mountPath extracts a static mount path on the same line as the call.
func mountPath(src []byte, n parser.Node, callLine int, call *parser.CallNode) (string, bool) {
	if n == nil {
		return "", false
	}
	if prism.LineAt(src, n.GetLocation().StartOffset) != callLine {
		return "", false
	}
	if v, ok := prism.StringValue(n); ok {
		return v, true
	}
	return "", false
}

// classifyMountTarget classifies a mount target node into a constant or
// receiver target (modeled) versus dynamic.
func classifyMountTarget(src []byte, n parser.Node) (target string, receiverTarget bool, dynamic bool) {
	switch v := n.(type) {
	case *parser.ConstantReadNode:
		return v.Name, false, false
	case *parser.ConstantPathNode:
		return prism.Slice(src, v.Location), false, false
	case *parser.CallNode:
		if v.Receiver != nil && len(prism.ArgNodes(v)) == 0 {
			return prism.Slice(src, v.Location), true, false
		}
		return "", false, true
	default:
		return "", false, true
	}
}

// handleDraw handles `draw :name`/`draw "name"` invocations and the outer
// `Rails.application.routes.draw do ... end` wrapper.
func (p *staticParser) handleDraw(routesPath string, src []byte, call *parser.CallNode, frame, outerFrame routeFrame, errs []parser.ParseError) StaticResult {
	args := prism.ArgNodes(call)
	line := prism.LineAt(src, call.Location.StartOffset)
	if len(args) >= 1 {
		name, ok := stringOrSymbol(args[0])
		if !ok {
			return StaticResult{Warnings: []StaticWarning{staticWarning(routesPath, line, "dynamic draw target is not modeled: "+firstLine(src, call.Location))}}
		}
		if !reDrawName.MatchString(name) {
			return StaticResult{Warnings: []StaticWarning{staticWarning(routesPath, line, "unsafe or unsupported draw target: "+firstLine(src, call.Location))}}
		}
		for _, segment := range strings.Split(name, "/") {
			if segment == "" || segment == "." || segment == ".." {
				return StaticResult{Warnings: []StaticWarning{staticWarning(routesPath, line, "unsafe or unsupported draw target: "+firstLine(src, call.Location))}}
			}
		}
		drawPath, err := p.resolveDrawPath(name)
		if err != nil {
			return StaticResult{Warnings: []StaticWarning{staticWarning(routesPath, line, err.Error())}}
		}
		canonicalDrawPath := drawPath
		if resolved, evalErr := filepath.EvalSymlinks(drawPath); evalErr == nil {
			canonicalDrawPath = resolved
		}
		if p.active[canonicalDrawPath] {
			return StaticResult{Warnings: []StaticWarning{staticWarning(routesPath, line, "cyclic draw was skipped: "+name)}}
		}
		drawn, drawErr := p.parseFileAST(drawPath, frame)
		if drawErr != nil {
			return StaticResult{Warnings: []StaticWarning{staticWarning(routesPath, line, "drawn route file could not be read: "+drawErr.Error())}}
		}
		return drawn
	}
	if call.Block != nil {
		return p.walkBlock(routesPath, src, call.Block, frame, outerFrame, errs)
	}
	return StaticResult{}
}

// handleConcern handles `concern :name do ... end` definitions.
func (p *staticParser) handleConcern(routesPath string, src []byte, call *parser.CallNode, frame routeFrame) StaticResult {
	args := prism.ArgNodes(call)
	line := prism.LineAt(src, call.Location.StartOffset)

	if len(args) == 0 {
		return StaticResult{}
	}
	name, ok := prism.SymbolValue(args[0])
	if !ok {
		// reConcernAny path: dynamic concern definition; consume the block.
		return StaticResult{Warnings: []StaticWarning{staticWarning(routesPath, line, "dynamic route concern definition is not modeled: "+firstLine(src, call.Location))}}
	}
	if blockKind(src, call.Block) != "do" {
		// No block (or a brace block, which the old scanner did not treat as a
		// concern body): callable/dynamic concern.
		p.concerns[name] = routeConcern{path: routesPath, supported: false}
		return StaticResult{Warnings: []StaticWarning{staticWarning(routesPath, line, "callable or dynamic route concern is not modeled: "+name)}}
	}
	bn := call.Block.(*parser.BlockNode)
	if bn.ClosingLoc.Length == 0 {
		p.concerns[name] = routeConcern{path: routesPath, supported: false}
		return StaticResult{Warnings: []StaticWarning{staticWarning(routesPath, line, "unterminated route concern: "+name)}}
	}
	if len(args) > 1 || bn.Parameters != nil {
		p.concerns[name] = routeConcern{path: routesPath, supported: false}
		return StaticResult{Warnings: []StaticWarning{staticWarning(routesPath, line, "parameterized route concern is not modeled: "+name)}}
	}
	p.concerns[name] = routeConcern{
		path:      routesPath,
		src:       src,
		nodes:     prism.BlockStatements(bn.Body),
		supported: true,
	}
	return StaticResult{}
}

// handleConcerns handles `concerns :name, :other` and `concerns [:name, :other]`
// invocations, optionally with a trailing options hash.
func (p *staticParser) handleConcerns(routesPath string, src []byte, call *parser.CallNode, frame, outerFrame routeFrame) StaticResult {
	args := prism.ArgNodes(call)
	line := prism.LineAt(src, call.Location.StartOffset)
	var names []string
	hasOptions := false
	for _, a := range args {
		if assocs := prism.KeywordAssocs(a); assocs != nil {
			hasOptions = true
			continue
		}
		if v, ok := prism.SymbolValue(a); ok {
			names = append(names, v)
			continue
		}
		if arr, ok := a.(*parser.ArrayNode); ok {
			for _, e := range arr.Elements {
				if v, ok := prism.SymbolValue(e); ok {
					names = append(names, v)
				} else {
					return StaticResult{Warnings: []StaticWarning{staticWarning(routesPath, line, "dynamic route concern name is not modeled: "+firstLine(src, call.Location))}}
				}
			}
			continue
		}
		return StaticResult{Warnings: []StaticWarning{staticWarning(routesPath, line, "dynamic route concern name is not modeled: "+firstLine(src, call.Location))}}
	}
	if len(names) == 0 {
		return StaticResult{Warnings: []StaticWarning{staticWarning(routesPath, line, "route concern invocation has no names")}}
	}
	var result StaticResult
	if hasOptions {
		result.Warnings = append(result.Warnings, staticWarning(routesPath, line, "route concern invocation options are not modeled"))
	}
	result.append(p.expandConcerns(names, routesPath, line, frame, outerFrame))
	return result
}

// handleVerb handles get/post/put/patch/delete/match route declarations.
func (p *staticParser) handleVerb(routesPath string, src []byte, call *parser.CallNode, frame routeFrame, errs []parser.ParseError) StaticResult {
	line := prism.LineAt(src, call.Location.StartOffset)
	if _, ok := unterminatedVerb(src, call, errs); ok {
		return StaticResult{Warnings: []StaticWarning{staticWarning(routesPath, line, "unterminated multiline route declaration")}}
	}

	args := prism.ArgNodes(call)
	if len(args) == 0 {
		return StaticResult{}
	}
	declaration := verbDeclaration{verb: strings.ToUpper(call.Name)}

	var pathNode, targetNode parser.Node
	var optionAssocs []*parser.AssocNode

	if assocs := prism.KeywordAssocs(args[0]); assocs != nil {
		// Hash-rocket form: "path" => "c#a", :as => "name", ...
		if len(assocs) == 0 {
			return StaticResult{}
		}
		pathNode = assocs[0].Key
		targetNode = assocs[0].Value
		optionAssocs = assocs[1:]
	} else {
		pathNode = args[0]
		if len(args) >= 2 {
			optionAssocs = prism.KeywordAssocs(args[1])
		}
	}

	routePath, symbolic, ok := routePathValue(src, pathNode)
	if !ok {
		return StaticResult{Warnings: []StaticWarning{staticWarning(routesPath, line, "unsupported verb route syntax: "+firstLine(src, call.Location))}}
	}
	declaration.routePath = routePath
	declaration.symbolic = symbolic

	var toNode parser.Node
	for _, a := range optionAssocs {
		key, _ := prism.SymbolValue(a.Key)
		switch key {
		case "to":
			toNode = a.Value
		case "controller":
			declaration.controller = scalar(src, a.Value)
		case "action":
			declaration.action = scalar(src, a.Value)
		case "as":
			declaration.helper = scalar(src, a.Value)
		case "on":
			declaration.on = scalar(src, a.Value)
		case "constraints":
			declaration.constraints, declaration.hasConstraints, declaration.constraintGap = parsePathConstraintsAST(src, a.Value)
		case "via":
			// handled for match below; ignored for other verbs
		}
	}

	if call.Name == "match" {
		viaNode := assocValue(optionAssocs, "via")
		verb, err := parseMatchViaAST(viaNode)
		if err != nil {
			return StaticResult{Warnings: []StaticWarning{staticWarning(routesPath, line, err.Error())}}
		}
		declaration.verb = verb
	}

	// Resolve the target / redirect from the hash-rocket value or the to: option.
	resolved := false
	if targetNode != nil {
		resolved = applyTarget(src, targetNode, &declaration)
	} else if toNode != nil {
		resolved = applyTarget(src, toNode, &declaration)
	}
	if resolved {
		// applyTarget already set declaration.dynamic.
		message := "dynamic route target is not modeled: " + firstLine(src, call.Location)
		if declaration.dynamicRedirect {
			message = "dynamic redirect target is not modeled: " + firstLine(src, call.Location)
		}
		return StaticResult{Warnings: []StaticWarning{staticWarning(routesPath, line, message)}}
	}

	entry, constraintWarning, resolveErr := resolveVerbDeclaration(declaration, frame)
	if resolveErr != nil {
		return StaticResult{Warnings: []StaticWarning{staticWarning(routesPath, line, resolveErr.Error())}}
	}
	var result StaticResult
	result.Entries = append(result.Entries, entry)
	if constraintWarning != "" {
		result.Warnings = append(result.Warnings, staticWarning(routesPath, line, constraintWarning))
	}
	return result
}

// unterminatedVerb detects the `get "/path",` form whose continuation never
// arrives (a parse error sits at the call's end offset).
func unterminatedVerb(src []byte, call *parser.CallNode, errs []parser.ParseError) (int, bool) {
	end := call.Location.StartOffset + call.Location.Length
	for _, e := range errs {
		if e.Type == "expect_argument" && e.Location.StartOffset == end {
			return e.Location.StartOffset, true
		}
	}
	return 0, false
}

// applyTarget sets the declaration target or redirect from a target node. It
// returns true when the target is dynamic (and the caller should warn).
func applyTarget(src []byte, n parser.Node, declaration *verbDeclaration) bool {
	switch v := n.(type) {
	case *parser.StringNode:
		declaration.target = v.Unescaped.Value
	case *parser.SymbolNode:
		declaration.target = v.Unescaped.Value
	case *parser.InterpolatedStringNode:
		declaration.target = normalizeOptionScalar(prism.Slice(src, v.Location))
	case *parser.CallNode:
		if v.Name == "redirect" {
			rd, ok := parseRedirectAST(src, v)
			if !ok {
				declaration.dynamic = true
				declaration.dynamicRedirect = true
				return true
			}
			declaration.redirect = &rd
		} else {
			declaration.dynamic = true
			return true
		}
	default:
		// Lambda, local variable, etc.
		declaration.dynamic = true
		return true
	}
	return false
}

// parseRedirectAST validates a redirect(...) call into a redirectDeclaration.
// Returns false for any dynamic/malformed redirect.
func parseRedirectAST(src []byte, call *parser.CallNode) (redirectDeclaration, bool) {
	if call.Block != nil {
		return redirectDeclaration{}, false
	}
	args := prism.ArgNodes(call)
	if len(args) == 0 {
		return redirectDeclaration{}, false
	}
	dest, ok := prism.StringValue(args[0])
	if !ok {
		return redirectDeclaration{}, false
	}
	redirect := redirectDeclaration{destination: dest, status: 301}
	if len(args) >= 2 {
		assocs := prism.KeywordAssocs(args[1])
		if len(assocs) == 0 {
			return redirectDeclaration{}, false
		}
		for _, a := range assocs {
			key, _ := prism.SymbolValue(a.Key)
			if key != "status" {
				return redirectDeclaration{}, false
			}
			iv, ok := a.Value.(*parser.IntegerNode)
			if !ok || iv.Value < 100 || iv.Value > 999 {
				return redirectDeclaration{}, false
			}
			redirect.status = int(iv.Value)
		}
	}
	return redirect, true
}

// parseMatchViaAST validates a `via:` option node into a verb string (joined
// with "|"), or returns an error matching the old parseMatchVia messages. The
// via value may be a symbol, string, bareword call, or an array of those; a
// nil node is absent (requires a static via), an empty array is an empty via.
func parseMatchViaAST(n parser.Node) (string, error) {
	methods := viaMethodNames(n)
	if methods == nil {
		return "", fmt.Errorf("match route requires a static via option")
	}
	if len(methods) == 0 {
		return "", fmt.Errorf("match route has an empty via option")
	}
	if len(methods) == 1 && methods[0] == "all" {
		return "", nil
	}
	valid := map[string]bool{"get": true, "head": true, "post": true, "patch": true, "put": true, "delete": true, "options": true}
	var verbs []string
	for _, method := range methods {
		method = strings.ToLower(method)
		if !valid[method] {
			return "", fmt.Errorf("match route has unsupported via method: %s", method)
		}
		verbs = append(verbs, strings.ToUpper(method))
	}
	return strings.Join(verbs, "|"), nil
}

// viaMethodNames extracts via method names from a via option node: a single
// symbol/string/bareword, or an array of them. It returns nil for an absent or
// non-static via (so the caller emits the "requires a static via" warning), and
// a non-nil empty slice for an empty array (so the caller emits "empty via").
func viaMethodNames(n parser.Node) []string {
	if n == nil {
		return nil
	}
	if arr, ok := n.(*parser.ArrayNode); ok {
		out := make([]string, 0, len(arr.Elements))
		for _, e := range arr.Elements {
			name, ok := viaScalarName(e)
			if !ok {
				return nil
			}
			out = append(out, name)
		}
		return out
	}
	name, ok := viaScalarName(n)
	if !ok {
		return nil
	}
	return []string{name}
}

// viaScalarName returns the method name carried by a single via item: a
// symbol, a plain string, or a bareword call (e.g. `methods`).
func viaScalarName(n parser.Node) (string, bool) {
	if v, ok := prism.SymbolValue(n); ok {
		return v, true
	}
	if v, ok := prism.StringValue(n); ok {
		return v, true
	}
	if c, ok := n.(*parser.CallNode); ok && c.Receiver == nil && len(prism.ArgNodes(c)) == 0 {
		return c.Name, true
	}
	return "", false
}

// parsePathConstraintsAST extracts static path constraints from a constraints
// option value, returning (constraints, hasConstraints, gap).
func parsePathConstraintsAST(src []byte, n parser.Node) ([]pathConstraint, bool, bool) {
	hash, ok := n.(*parser.HashNode)
	if !ok {
		// constraints: <CallNode|Lambda|...> -> not a hash; gap, not modeled.
		return nil, true, true
	}
	if hash.ClosingLoc.Length == 0 {
		// Unterminated constraint hash.
		return nil, true, true
	}
	var constraints []pathConstraint
	gap := false
	for _, e := range hash.Elements {
		assoc, ok := e.(*parser.AssocNode)
		if !ok {
			gap = true
			continue
		}
		key, ok := prism.SymbolValue(assoc.Key)
		if !ok || !isConstraintKey(key) {
			gap = true
			continue
		}
		value, static := constraintRegexp(src, assoc.Value)
		if !static {
			gap = true
			continue
		}
		constraints = append(constraints, pathConstraint{key: key, value: value})
	}
	if len(constraints) == 0 {
		gap = true
	}
	return constraints, true, gap
}

// constraintRegexp returns the source slice of a static regular expression
// constraint and true; interpolated regexps, strings, and anything else are
// not static.
func constraintRegexp(src []byte, n parser.Node) (string, bool) {
	if _, ok := n.(*parser.RegularExpressionNode); ok {
		value := prism.Slice(src, n.GetLocation())
		if isStaticConstraintRegexp(value) {
			return value, true
		}
		return "", false
	}
	return "", false
}
