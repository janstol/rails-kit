package routes

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"
	"github.com/janstol/rails-kit/internal/pluralize"
	"github.com/janstol/rails-kit/internal/prism"
)

// Regular expressions retained by the AST driver: reDrawName validates draw
// targets, reAcronymWord/reWordCase drive underscoreMountTarget's camelCase
// splitting.
var (
	reDrawName    = regexp.MustCompile(`^[A-Za-z0-9_/-]+$`)
	reAcronymWord = regexp.MustCompile(`([A-Z]+)([A-Z][a-z])`)
	reWordCase    = regexp.MustCompile(`([a-z0-9])([A-Z])`)
)

// routeFrame holds URL, controller, helper, and resource context for a block.
type routeFrame struct {
	depth            int
	pathPrefix       string
	controllerPrefix string
	helperPrefix     string

	collectionPath     string
	memberPath         string
	resourcePath       string
	resourceName       string
	resourceSingular   string
	resourceParam      string
	resourceController string
	collectionHelper   string
	memberHelper       string
	routeMode          string
	defaultController  string
}

// StaticWarning describes route syntax that the approximate parser skipped or
// only partially modeled.
type StaticWarning struct {
	Path    string
	Line    int
	Message string
}

// StaticResult contains parsed routes and non-fatal approximation warnings.
type StaticResult struct {
	Entries  []RouteEntry
	Warnings []StaticWarning
	// corrupt signals that this result's source statement corrupted the
	// enclosing block's frame, so the block's subsequent siblings must be
	// walked at the outer (parent) frame. It mirrors a frame-stack bug in the
	// old regex scanner: a `namespace :x, defaults: {...} do` line was mistaken
	// for a malformed inline brace block, and its `end` popped the enclosing
	// namespace too. It is not serialized and does not affect equality of the
	// visible fields.
	corrupt bool
}

type resourceOptions struct {
	path       string
	hasPath    bool
	controller string
	helper     string
	param      string
	concerns   []string
	actions    map[string]bool
}

type verbDeclaration struct {
	verb            string
	routePath       string
	symbolic        bool
	target          string
	redirect        *redirectDeclaration
	controller      string
	action          string
	helper          string
	on              string
	constraints     []pathConstraint
	hasConstraints  bool
	constraintGap   bool
	dynamic         bool
	dynamicRedirect bool
}

type pathConstraint struct {
	key   string
	value string
}

type redirectDeclaration struct {
	destination string
	status      int
}

type mountDeclaration struct {
	target         string
	path           string
	helper         string
	hasHelper      bool
	receiverTarget bool
}

type staticParser struct {
	pluralizer    *pluralize.Pluralizer
	drawRoot      string
	active        map[string]bool
	concerns      map[string]routeConcern
	activeConcern map[string]bool
	prismParser   *prism.Parser
	ctx           context.Context
}
type routeConcern struct {
	path      string
	src       []byte
	nodes     []parser.Node
	supported bool
}

// ParseStatic parses config/routes.rb and returns route entries.
func ParseStatic(routesPath string, p *pluralize.Pluralizer) ([]RouteEntry, error) {
	result, err := ParseStaticDetailed(routesPath, p)
	if err != nil {
		return nil, err
	}
	return result.Entries, nil
}

// ParseStaticDetailed parses config/routes.rb and returns routes plus approximation warnings.
func ParseStaticDetailed(routesPath string, p *pluralize.Pluralizer) (StaticResult, error) {
	drawRoot, err := filepath.Abs(filepath.Join(filepath.Dir(routesPath), "routes"))
	if err != nil {
		return StaticResult{}, fmt.Errorf("resolving routes directory: %w", err)
	}
	ctx := context.Background()
	prismParser, err := prism.NewParser(ctx)
	if err != nil {
		return StaticResult{}, fmt.Errorf("creating prism parser: %w", err)
	}
	defer prismParser.Close(ctx) //nolint:errcheck
	sp := staticParser{
		pluralizer:    p,
		drawRoot:      drawRoot,
		active:        make(map[string]bool),
		concerns:      make(map[string]routeConcern),
		activeConcern: make(map[string]bool),
		prismParser:   prismParser,
		ctx:           ctx,
	}
	return sp.parseFileAST(routesPath, routeFrame{depth: -1})
}
func namespaceRouteFrame(frame routeFrame, name string) routeFrame {
	frame.pathPrefix = joinRoutePath(frame.pathPrefix, name)
	frame.controllerPrefix += name + "/"
	frame.helperPrefix += name + "_"
	frame.defaultController = strings.TrimSuffix(frame.controllerPrefix, "/")
	frame.routeMode = ""
	return frame
}

func (p *staticParser) expandConcerns(names []string, routesPath string, lineNumber int, frame, outerFrame routeFrame) StaticResult {
	var result StaticResult
	for _, name := range names {
		concern, ok := p.concerns[name]
		if !ok {
			result.Warnings = append(result.Warnings, staticWarning(routesPath, lineNumber, "route concern was not found: "+name))
			continue
		}
		if !concern.supported {
			continue
		}
		if p.activeConcern[name] {
			result.Warnings = append(result.Warnings, staticWarning(routesPath, lineNumber, "cyclic route concern was skipped: "+name))
			continue
		}

		p.activeConcern[name] = true
		expanded := p.walkStatements(concern.path, concern.src, concern.nodes, frame, outerFrame, nil)
		delete(p.activeConcern, name)
		result.Entries = append(result.Entries, expanded.Entries...)
		result.Warnings = append(result.Warnings, expanded.Warnings...)
	}
	return result
}
func (p *staticParser) resolveDrawPath(name string) (string, error) {
	candidate := filepath.Clean(filepath.Join(p.drawRoot, filepath.FromSlash(name)+".rb"))
	if !pathWithin(p.drawRoot, candidate) {
		return "", fmt.Errorf("draw target escapes config/routes: %s", name)
	}

	resolved, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		return candidate, nil
	}
	resolvedRoot := p.drawRoot
	if root, rootErr := filepath.EvalSymlinks(p.drawRoot); rootErr == nil {
		resolvedRoot = root
	}
	if !pathWithin(resolvedRoot, resolved) {
		return "", fmt.Errorf("draw target escapes config/routes through a symlink: %s", name)
	}
	return candidate, nil
}

func pathWithin(root, candidate string) bool {
	relative, err := filepath.Rel(root, candidate)
	if err != nil {
		return false
	}
	return relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func allResourceActions(singular bool) map[string]bool {
	actions := []string{"index", "new", "create", "show", "edit", "update", "destroy"}
	if singular {
		actions = actions[1:]
	}
	result := make(map[string]bool, len(actions))
	for _, action := range actions {
		result[action] = true
	}
	return result
}
func normalizeOptionScalar(raw string) string {
	return strings.Trim(strings.TrimSpace(raw), ":\"' ")
}

func applyResourceScopePath(frame *routeFrame, scopePath string) {
	collectionPath := frame.collectionPath
	separator := strings.LastIndex(strings.TrimSuffix(collectionPath, "/"), "/")
	parentPath := ""
	resourceSegment := strings.Trim(collectionPath, "/")
	if separator >= 0 {
		parentPath = collectionPath[:separator]
		resourceSegment = collectionPath[separator+1:]
	}
	scopedCollection := joinRoutePath(joinRoutePath(parentPath, scopePath), resourceSegment)
	replacePrefix := func(path string) string {
		if path == collectionPath {
			return scopedCollection
		}
		if strings.HasPrefix(path, collectionPath+"/") {
			return scopedCollection + strings.TrimPrefix(path, collectionPath)
		}
		return path
	}
	frame.pathPrefix = replacePrefix(frame.pathPrefix)
	frame.collectionPath = scopedCollection
	frame.memberPath = replacePrefix(frame.memberPath)
	frame.resourcePath = replacePrefix(frame.resourcePath)
}

func applyResourceScopeHelper(frame *routeFrame, helper string) {
	parentPrefix := strings.TrimSuffix(frame.memberHelper, frame.resourceSingular)
	scopedBase := parentPrefix + helper + "_" + frame.resourceSingular
	frame.memberHelper = scopedBase
	frame.collectionHelper = parentPrefix + helper + "_" + frame.resourceName
	frame.helperPrefix = scopedBase + "_"
}
func resolveMountDeclaration(declaration mountDeclaration, frame routeFrame) RouteEntry {
	helper := declaration.helper
	if !declaration.hasHelper && !declaration.receiverTarget {
		helper = underscoreMountTarget(declaration.target)
	}
	if helper != "" {
		helper = frame.helperPrefix + helper
	}
	return RouteEntry{
		Prefix:           helper,
		Verb:             "",
		URIPattern:       joinRoutePath(frame.pathPrefix, declaration.path),
		ControllerAction: declaration.target,
	}
}
func underscoreMountTarget(target string) string {
	target = strings.ReplaceAll(target, "::", "/")
	target = reAcronymWord.ReplaceAllString(target, "${1}_${2}")
	target = reWordCase.ReplaceAllString(target, "${1}_${2}")
	target = strings.ReplaceAll(target, "-", "_")
	return strings.ReplaceAll(strings.ToLower(target), "/", "_")
}
func isConstraintKey(key string) bool {
	for position := range len(key) {
		char := key[position]
		validStart := (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || char == '_'
		if position == 0 && !validStart {
			return false
		}
		if position > 0 && !isRouteWordByte(char) {
			return false
		}
	}
	return true
}

func isStaticConstraintRegexp(value string) bool {
	if len(value) < 2 || value[0] != '/' || strings.Contains(value, "#{") {
		return false
	}
	escaped := false
	for position := 1; position < len(value); position++ {
		char := value[position]
		if escaped {
			escaped = false
			continue
		}
		if char == '\\' {
			escaped = true
			continue
		}
		if char != '/' {
			continue
		}
		for _, flag := range value[position+1:] {
			if flag != 'i' && flag != 'm' && flag != 'x' {
				return false
			}
		}
		return true
	}
	return false
}
func isRouteWordByte(value byte) bool {
	return value == '_' || value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z' || value >= '0' && value <= '9'
}

func resolveVerbDeclaration(declaration verbDeclaration, frame routeFrame) (RouteEntry, string, error) {
	mode := declaration.on
	if mode == "" {
		mode = frame.routeMode
	}
	basePath := frame.pathPrefix
	switch mode {
	case "member":
		if frame.memberPath == "" {
			return RouteEntry{}, "", fmt.Errorf("member route outside a resource was skipped")
		}
		basePath = frame.memberPath
	case "collection":
		if frame.collectionPath == "" {
			return RouteEntry{}, "", fmt.Errorf("collection route outside a resource was skipped")
		}
		basePath = frame.collectionPath
	case "", "resource":
	default:
		return RouteEntry{}, "", fmt.Errorf("unsupported route location: %s", mode)
	}
	routePath := joinRoutePath(basePath, declaration.routePath)
	constraintSuffix, constraintWarning := resolvePathConstraints(declaration, routePath)

	if declaration.redirect != nil {
		helper := declaration.helper
		if helper != "" {
			helper = frame.helperPrefix + helper
		}
		return RouteEntry{
			Prefix:           helper,
			Verb:             declaration.verb,
			URIPattern:       routePath,
			ControllerAction: fmt.Sprintf("redirect(%d, %s)%s", declaration.redirect.status, declaration.redirect.destination, constraintSuffix),
		}, constraintWarning, nil
	}

	controller, action := resolveVerbTarget(declaration, frame)
	if controller == "" || action == "" {
		return RouteEntry{}, "", fmt.Errorf("could not infer controller/action for route: %s %s", declaration.verb, declaration.routePath)
	}

	helper := declaration.helper
	if helper != "" {
		helper = frame.helperPrefix + helper
	} else {
		helper = inferredVerbHelper(action, mode, frame)
	}
	return RouteEntry{
		Prefix:           helper,
		Verb:             declaration.verb,
		URIPattern:       routePath,
		ControllerAction: controller + "#" + action + constraintSuffix,
	}, constraintWarning, nil
}

func resolvePathConstraints(declaration verbDeclaration, routePath string) (string, string) {
	if !declaration.hasConstraints {
		return "", ""
	}
	var modeled []string
	gap := declaration.constraintGap
	for _, constraint := range declaration.constraints {
		if !routeHasParameter(routePath, constraint.key) {
			gap = true
			continue
		}
		modeled = append(modeled, constraint.key+": "+constraint.value)
	}

	suffix := ""
	if len(modeled) > 0 {
		suffix = " {" + strings.Join(modeled, ", ") + "}"
	}
	switch {
	case !gap:
		return suffix, ""
	case len(modeled) > 0:
		return suffix, "route constraints are only partially modeled"
	default:
		return "", "route constraints are not modeled"
	}
}

func routeHasParameter(routePath, key string) bool {
	needle := ":" + key
	for start := 0; start < len(routePath); {
		position := strings.Index(routePath[start:], needle)
		if position < 0 {
			return false
		}
		position += start
		beforeOK := position == 0 || routePath[position-1] == '/' || routePath[position-1] == '('
		end := position + len(needle)
		afterOK := end == len(routePath) || !isRouteWordByte(routePath[end])
		if beforeOK && afterOK {
			return true
		}
		start = position + len(needle)
	}
	return false
}

func resolveVerbTarget(declaration verbDeclaration, frame routeFrame) (string, string) {
	if declaration.target != "" {
		parts := strings.SplitN(declaration.target, "#", 2)
		if len(parts) == 2 {
			return qualifyController(frame.controllerPrefix, parts[0]), parts[1]
		}
	}
	controller := declaration.controller
	action := declaration.action
	if action == "" {
		action = routeActionFromPath(declaration.routePath)
	}
	if controller != "" {
		return qualifyController(frame.controllerPrefix, controller), action
	}
	if frame.resourceController != "" {
		return frame.resourceController, action
	}
	if frame.defaultController != "" {
		return frame.defaultController, action
	}
	if !declaration.symbolic && strings.Contains(declaration.routePath, "/") {
		parts := strings.Split(strings.Trim(declaration.routePath, "/"), "/")
		if len(parts) > 1 {
			return qualifyController(frame.controllerPrefix, strings.Join(parts[:len(parts)-1], "/")), action
		}
	}
	return "", action
}

func routeActionFromPath(routePath string) string {
	parts := strings.Split(strings.Trim(routePath, "/"), "/")
	if len(parts) == 0 {
		return ""
	}
	action := parts[len(parts)-1]
	if strings.HasPrefix(action, ":") && len(parts) > 1 {
		action = parts[len(parts)-2]
	}
	return strings.TrimPrefix(action, ":")
}

func inferredVerbHelper(action, mode string, frame routeFrame) string {
	switch mode {
	case "member":
		if frame.memberHelper != "" {
			return action + "_" + frame.memberHelper
		}
	case "collection":
		if frame.collectionHelper != "" {
			return action + "_" + frame.collectionHelper
		}
	case "resource":
		if frame.memberHelper != "" {
			return frame.memberHelper + "_" + action
		}
	}
	return frame.helperPrefix + action
}

func expandResources(controller, collectionPath, collectionHelper, memberHelper, param string, actions map[string]bool, singular bool) []RouteEntry {
	type routeDef struct {
		verb, path, action, helper string
	}
	var defs []routeDef
	if singular {
		defs = []routeDef{
			{"GET", joinRoutePath(collectionPath, "new"), "new", "new_" + memberHelper},
			{"GET", joinRoutePath(collectionPath, "edit"), "edit", "edit_" + memberHelper},
			{"GET", collectionPath, "show", memberHelper},
			{"POST", collectionPath, "create", memberHelper},
			{"PATCH", collectionPath, "update", memberHelper},
			{"PUT", collectionPath, "update", memberHelper},
			{"DELETE", collectionPath, "destroy", memberHelper},
		}
	} else {
		memberPath := joinRoutePath(collectionPath, ":"+param)
		defs = []routeDef{
			{"GET", collectionPath, "index", collectionHelper},
			{"POST", collectionPath, "create", collectionHelper},
			{"GET", joinRoutePath(collectionPath, "new"), "new", "new_" + memberHelper},
			{"GET", joinRoutePath(memberPath, "edit"), "edit", "edit_" + memberHelper},
			{"GET", memberPath, "show", memberHelper},
			{"PATCH", memberPath, "update", memberHelper},
			{"PUT", memberPath, "update", memberHelper},
			{"DELETE", memberPath, "destroy", memberHelper},
		}
	}
	var entries []RouteEntry
	emittedHelpers := make(map[string]bool)
	for _, definition := range defs {
		if actions[definition.action] {
			helper := definition.helper
			if emittedHelpers[helper] {
				helper = ""
			} else {
				emittedHelpers[helper] = true
			}
			entries = append(entries, RouteEntry{
				Prefix:           helper,
				Verb:             definition.verb,
				URIPattern:       definition.path,
				ControllerAction: controller + "#" + definition.action,
			})
		}
	}
	return entries
}

func qualifyController(prefix, controller string) string {
	if strings.HasPrefix(controller, "/") {
		return strings.TrimPrefix(controller, "/")
	}
	return prefix + strings.Trim(controller, "/")
}

func joinRoutePath(base, segment string) string {
	base = strings.TrimSuffix(base, "/")
	segment = strings.Trim(segment, "/")
	switch {
	case base == "" && segment == "":
		return "/"
	case base == "":
		return "/" + segment
	case segment == "":
		return base
	default:
		return base + "/" + segment
	}
}

func rootRoutePath(prefix string) string {
	if prefix == "" {
		return "/"
	}
	return strings.TrimSuffix(prefix, "/") + "/"
}

func staticWarning(path string, line int, message string) StaticWarning {
	return StaticWarning{Path: path, Line: line, Message: message}
}
