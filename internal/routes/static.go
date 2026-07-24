package routes

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/janstol/rails-kit/internal/pluralize"
)

var (
	reNamespace   = regexp.MustCompile(`^\s*namespace\s+:(\w+)`)
	reResources   = regexp.MustCompile(`^\s*(resources?)\s+:(\w+)`)
	reVerbStart   = regexp.MustCompile(`^\s*(get|post|put|patch|delete)\s+(.+)$`)
	reRoot        = regexp.MustCompile(`^\s*root\s+(?:to:\s*)?['"]([^'"]+)['"]`)
	reBlockEnd    = regexp.MustCompile(`^\s*end\b`)
	reBlockOpener = regexp.MustCompile(`\bdo\b`)
	reMember      = regexp.MustCompile(`^\s*member\s+do\b`)
	reCollection  = regexp.MustCompile(`^\s*collection\s+do\b`)
	reScopeModule = regexp.MustCompile(`^\s*scope\s+module:\s*`)
	reDraw        = regexp.MustCompile(`^\s*draw\b`)
	reDrawName    = regexp.MustCompile(`^[A-Za-z0-9_/-]+$`)
	reConcern     = regexp.MustCompile(`^\s*concern\s+:([A-Za-z_]\w*)\b(.*)$`)
	reConcernArgs = regexp.MustCompile(`\bdo\s*\|`)
	reConcernAny  = regexp.MustCompile(`^\s*concern\b`)
	reConcerns    = regexp.MustCompile(`^\s*concerns\b`)
	reConcernName = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
	reOptionStart = regexp.MustCompile(`^(?:[A-Za-z_]\w*\s*:|:[A-Za-z_]\w*\s*=>)`)
	reRubyBlock   = regexp.MustCompile(`^\s*(if|unless|case|begin|while|until|for)\b`)
	reUnsupported = regexp.MustCompile(`^\s*(scope|constraints?|mount|devise_\w+|direct|resolve|match)\b`)
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
	constraints     bool
	dynamic         bool
	dynamicRedirect bool
}

type redirectDeclaration struct {
	destination string
	status      int
}

type staticParser struct {
	pluralizer    *pluralize.Pluralizer
	drawRoot      string
	active        map[string]bool
	concerns      map[string]routeConcern
	activeConcern map[string]bool
}

type routeConcern struct {
	path       string
	content    string
	lineOffset int
	supported  bool
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
	parser := staticParser{
		pluralizer:    p,
		drawRoot:      drawRoot,
		active:        make(map[string]bool),
		concerns:      make(map[string]routeConcern),
		activeConcern: make(map[string]bool),
	}
	return parser.parseFile(routesPath, routeFrame{depth: -1})
}

func (p *staticParser) parseFile(routesPath string, initialFrame routeFrame) (StaticResult, error) {
	f, err := os.Open(routesPath)
	if err != nil {
		return StaticResult{}, err
	}
	defer f.Close() //nolint:errcheck

	canonicalPath, err := filepath.Abs(routesPath)
	if err != nil {
		return StaticResult{}, fmt.Errorf("resolving route file: %w", err)
	}
	if resolved, resolveErr := filepath.EvalSymlinks(canonicalPath); resolveErr == nil {
		canonicalPath = resolved
	}
	p.active[canonicalPath] = true
	defer delete(p.active, canonicalPath)

	return p.parseScanner(routesPath, bufio.NewScanner(f), initialFrame, 0)
}

func (p *staticParser) parseScanner(routesPath string, scanner *bufio.Scanner, initialFrame routeFrame, lineOffset int) (StaticResult, error) {
	initialFrame.depth = -1
	var (
		result StaticResult
		stack  = []routeFrame{initialFrame}
		depth  int
	)
	currentFrame := func() routeFrame { return stack[len(stack)-1] }

	lineNumber := lineOffset
	for scanner.Scan() {
		lineNumber++
		line := scanner.Text()
		trimmed := strings.TrimSpace(stripInlineComment(line))
		if trimmed == "" {
			continue
		}

		if match := reConcern.FindStringSubmatch(trimmed); match != nil {
			name := match[1]
			definitionLine := lineNumber
			remainder := strings.TrimSpace(match[2])
			hasBlock := reBlockOpener.MatchString(remainder)
			supported := hasBlock &&
				!strings.Contains(remainder, ",") &&
				!reConcernArgs.MatchString(remainder)
			if !hasBlock {
				p.concerns[name] = routeConcern{path: routesPath, supported: false}
				result.Warnings = append(result.Warnings, staticWarning(routesPath, lineNumber, "callable or dynamic route concern is not modeled: "+name))
				continue
			}

			body, bodyOffset, complete := captureConcernBody(scanner, &lineNumber)
			if !complete {
				p.concerns[name] = routeConcern{path: routesPath, supported: false}
				result.Warnings = append(result.Warnings, staticWarning(routesPath, definitionLine, "unterminated route concern: "+name))
				break
			}
			if !supported {
				p.concerns[name] = routeConcern{path: routesPath, supported: false}
				result.Warnings = append(result.Warnings, staticWarning(routesPath, bodyOffset, "parameterized route concern is not modeled: "+name))
				continue
			}
			p.concerns[name] = routeConcern{
				path:       routesPath,
				content:    body,
				lineOffset: bodyOffset,
				supported:  true,
			}
			continue
		}
		if reConcernAny.MatchString(trimmed) {
			definitionLine := lineNumber
			if reBlockOpener.MatchString(trimmed) {
				_, _, _ = captureConcernBody(scanner, &lineNumber)
			}
			result.Warnings = append(result.Warnings, staticWarning(routesPath, definitionLine, "dynamic route concern definition is not modeled: "+trimmed))
			continue
		}

		if reBlockEnd.MatchString(trimmed) {
			depth--
			if len(stack) > 1 && stack[len(stack)-1].depth == depth {
				stack = stack[:len(stack)-1]
			}
			continue
		}

		hasDo := reBlockOpener.MatchString(trimmed)
		if reMember.MatchString(trimmed) {
			cf := currentFrame()
			if cf.memberPath == "" {
				result.Warnings = append(result.Warnings, staticWarning(routesPath, lineNumber, "member block outside a resource was skipped"))
			} else {
				cf.depth = depth
				cf.pathPrefix = cf.memberPath
				cf.routeMode = "member"
				stack = append(stack, cf)
			}
			depth++
			continue
		}
		if reCollection.MatchString(trimmed) {
			cf := currentFrame()
			if cf.collectionPath == "" {
				result.Warnings = append(result.Warnings, staticWarning(routesPath, lineNumber, "collection block outside a resource was skipped"))
			} else {
				cf.depth = depth
				cf.pathPrefix = cf.collectionPath
				cf.routeMode = "collection"
				stack = append(stack, cf)
			}
			depth++
			continue
		}

		if m := reNamespace.FindStringSubmatch(trimmed); m != nil && hasDo {
			name := m[1]
			cf := currentFrame()
			frame := cf
			frame.depth = depth
			frame.pathPrefix = joinRoutePath(cf.pathPrefix, name)
			frame.controllerPrefix = cf.controllerPrefix + name + "/"
			frame.helperPrefix = cf.helperPrefix + name + "_"
			frame.defaultController = strings.TrimSuffix(frame.controllerPrefix, "/")
			frame.routeMode = ""
			stack = append(stack, frame)
			depth++
			continue
		}

		if reScopeModule.MatchString(trimmed) && hasDo {
			moduleName, ok := optionScalar(trimmed, "module")
			if !ok {
				result.Warnings = append(result.Warnings, staticWarning(routesPath, lineNumber, "unsupported scope module syntax: "+trimmed))
				depth++
				continue
			}
			cf := currentFrame()
			frame := cf
			frame.depth = depth
			frame.controllerPrefix = cf.controllerPrefix + strings.Trim(moduleName, "/") + "/"
			if helper, hasHelper := optionScalar(trimmed, "as"); hasHelper {
				frame.helperPrefix = cf.helperPrefix + helper + "_"
			}
			if frame.resourceController == "" {
				frame.defaultController = strings.TrimSuffix(frame.controllerPrefix, "/")
			}
			stack = append(stack, frame)
			depth++
			continue
		}

		if m := reResources.FindStringSubmatch(trimmed); m != nil {
			singularResource := m[1] == "resource"
			name := m[2]
			cf := currentFrame()
			options := parseResourceOptions(trimmed, singularResource)

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
			frame := cf
			frame.pathPrefix = nestedPath
			frame.helperPrefix = memberHelper + "_"
			frame.collectionPath = collectionPath
			frame.memberPath = memberPath
			frame.resourcePath = nestedPath
			frame.resourceName = helperBase
			frame.resourceSingular = p.pluralizer.Singularize(helperBase)
			frame.resourceParam = param
			frame.resourceController = resourceController
			frame.collectionHelper = collectionHelper
			frame.memberHelper = memberHelper
			frame.routeMode = "resource"
			frame.defaultController = resourceController

			if len(options.concerns) > 0 {
				expanded := p.expandConcerns(options.concerns, routesPath, lineNumber, frame)
				result.Entries = append(result.Entries, expanded.Entries...)
				result.Warnings = append(result.Warnings, expanded.Warnings...)
			}

			if hasDo {
				frame.depth = depth
				stack = append(stack, frame)
				depth++
			}
			continue
		}

		if reConcerns.MatchString(trimmed) {
			names, hasOptions, parseErr := parseConcernInvocation(trimmed)
			if parseErr != nil {
				result.Warnings = append(result.Warnings, staticWarning(routesPath, lineNumber, parseErr.Error()))
				continue
			}
			if hasOptions {
				result.Warnings = append(result.Warnings, staticWarning(routesPath, lineNumber, "route concern invocation options are not modeled"))
			}
			expanded := p.expandConcerns(names, routesPath, lineNumber, currentFrame())
			result.Entries = append(result.Entries, expanded.Entries...)
			result.Warnings = append(result.Warnings, expanded.Warnings...)
			continue
		}

		if m := reRoot.FindStringSubmatch(trimmed); m != nil {
			parts := strings.SplitN(m[1], "#", 2)
			if len(parts) == 2 {
				cf := currentFrame()
				result.Entries = append(result.Entries, RouteEntry{
					Prefix:           cf.helperPrefix + "root",
					Verb:             "GET",
					URIPattern:       rootRoutePath(cf.pathPrefix),
					ControllerAction: qualifyController(cf.controllerPrefix, parts[0]) + "#" + parts[1],
				})
			}
			continue
		}

		if reDraw.MatchString(trimmed) {
			name, parseErr := parseDrawName(trimmed)
			if parseErr != nil {
				result.Warnings = append(result.Warnings, staticWarning(routesPath, lineNumber, parseErr.Error()))
				continue
			}
			drawPath, resolveErr := p.resolveDrawPath(name)
			if resolveErr != nil {
				result.Warnings = append(result.Warnings, staticWarning(routesPath, lineNumber, resolveErr.Error()))
				continue
			}
			canonicalDrawPath := drawPath
			if resolved, evalErr := filepath.EvalSymlinks(drawPath); evalErr == nil {
				canonicalDrawPath = resolved
			}
			if p.active[canonicalDrawPath] {
				result.Warnings = append(result.Warnings, staticWarning(routesPath, lineNumber, "cyclic draw was skipped: "+name))
				continue
			}
			drawn, drawErr := p.parseFile(drawPath, currentFrame())
			if drawErr != nil {
				result.Warnings = append(result.Warnings, staticWarning(routesPath, lineNumber, "drawn route file could not be read: "+drawErr.Error()))
				continue
			}
			result.Entries = append(result.Entries, drawn.Entries...)
			result.Warnings = append(result.Warnings, drawn.Warnings...)
			continue
		}

		if reVerbStart.MatchString(trimmed) {
			declaration, parseErr := parseVerbDeclaration(trimmed)
			if parseErr != nil {
				result.Warnings = append(result.Warnings, staticWarning(routesPath, lineNumber, parseErr.Error()))
				continue
			}
			if declaration.dynamic {
				message := "dynamic route target is not modeled: " + trimmed
				if declaration.dynamicRedirect {
					message = "dynamic redirect target is not modeled: " + trimmed
				}
				result.Warnings = append(result.Warnings, staticWarning(routesPath, lineNumber, message))
				continue
			}
			entry, resolveErr := resolveVerbDeclaration(declaration, currentFrame())
			if resolveErr != nil {
				result.Warnings = append(result.Warnings, staticWarning(routesPath, lineNumber, resolveErr.Error()))
				continue
			}
			result.Entries = append(result.Entries, entry)
			if declaration.constraints {
				result.Warnings = append(result.Warnings, staticWarning(routesPath, lineNumber, "route constraints are not modeled"))
			}
			continue
		}

		if reUnsupported.MatchString(trimmed) {
			result.Warnings = append(result.Warnings, staticWarning(routesPath, lineNumber, "unsupported route DSL: "+trimmed))
		}
		if hasDo {
			depth++
		}
	}

	if err := scanner.Err(); err != nil {
		return StaticResult{}, err
	}
	return result, nil
}

func captureConcernBody(scanner *bufio.Scanner, lineNumber *int) (string, int, bool) {
	lineOffset := *lineNumber
	depth := 1
	var lines []string
	for scanner.Scan() {
		(*lineNumber)++
		line := scanner.Text()
		trimmed := strings.TrimSpace(stripInlineComment(line))
		if reBlockEnd.MatchString(trimmed) {
			depth--
			if depth == 0 {
				return strings.Join(lines, "\n"), lineOffset, true
			}
			lines = append(lines, line)
			continue
		}
		lines = append(lines, line)
		if reBlockOpener.MatchString(trimmed) || reRubyBlock.MatchString(trimmed) {
			depth++
		}
	}
	return strings.Join(lines, "\n"), lineOffset, false
}

func parseConcernInvocation(line string) ([]string, bool, error) {
	value := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "concerns"))
	if strings.HasPrefix(value, "(") {
		if !strings.HasSuffix(value, ")") {
			return nil, false, fmt.Errorf("unsupported route concern syntax: %s", line)
		}
		value = strings.TrimSpace(value[1 : len(value)-1])
	}
	if value == "" {
		return nil, false, fmt.Errorf("route concern invocation has no names")
	}

	var (
		nameValues []string
		remainder  string
	)
	switch {
	case strings.HasPrefix(value, "["):
		end := strings.IndexByte(value, ']')
		if end < 0 {
			return nil, false, fmt.Errorf("unsupported route concern syntax: %s", line)
		}
		nameValues = splitOptionItems(value[1:end])
		remainder = strings.TrimSpace(strings.TrimPrefix(value[end+1:], ","))
	case strings.HasPrefix(value, "%i[") || strings.HasPrefix(value, "%w["):
		end := strings.IndexByte(value, ']')
		if end < 0 {
			return nil, false, fmt.Errorf("unsupported route concern syntax: %s", line)
		}
		nameValues = splitOptionItems(value[3:end])
		remainder = strings.TrimSpace(strings.TrimPrefix(value[end+1:], ","))
	default:
		for _, item := range strings.Split(value, ",") {
			item = strings.TrimSpace(item)
			if reOptionStart.MatchString(item) {
				remainder = item
				break
			}
			nameValues = append(nameValues, item)
		}
	}

	names := make([]string, 0, len(nameValues))
	for _, value := range nameValues {
		name := normalizeOptionScalar(value)
		if !reConcernName.MatchString(name) {
			return nil, false, fmt.Errorf("dynamic route concern name is not modeled: %s", line)
		}
		names = append(names, name)
	}
	if len(names) == 0 {
		return nil, false, fmt.Errorf("route concern invocation has no names")
	}
	return names, remainder != "", nil
}

func (p *staticParser) expandConcerns(names []string, routesPath string, lineNumber int, frame routeFrame) StaticResult {
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
		expanded, err := p.parseScanner(
			concern.path,
			bufio.NewScanner(strings.NewReader(concern.content)),
			frame,
			concern.lineOffset,
		)
		delete(p.activeConcern, name)
		if err != nil {
			result.Warnings = append(result.Warnings, staticWarning(routesPath, lineNumber, "route concern could not be parsed: "+err.Error()))
			continue
		}
		result.Entries = append(result.Entries, expanded.Entries...)
		result.Warnings = append(result.Warnings, expanded.Warnings...)
	}
	return result
}

func parseDrawName(line string) (string, error) {
	value := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "draw"))
	if strings.HasPrefix(value, "(") {
		if !strings.HasSuffix(value, ")") {
			return "", fmt.Errorf("unsupported draw syntax: %s", line)
		}
		value = strings.TrimSpace(value[1 : len(value)-1])
	}

	var name string
	switch {
	case strings.HasPrefix(value, ":"):
		name = strings.TrimSpace(strings.TrimPrefix(value, ":"))
	case len(value) >= 2 && (value[0] == '\'' || value[0] == '"') && value[len(value)-1] == value[0]:
		name = value[1 : len(value)-1]
	default:
		return "", fmt.Errorf("dynamic draw target is not modeled: %s", line)
	}
	if !reDrawName.MatchString(name) {
		return "", fmt.Errorf("unsafe or unsupported draw target: %s", line)
	}
	for _, segment := range strings.Split(name, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return "", fmt.Errorf("unsafe or unsupported draw target: %s", line)
		}
	}
	return name, nil
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

func parseResourceOptions(line string, singular bool) resourceOptions {
	options := resourceOptions{
		actions: allResourceActions(singular),
	}
	if actions, ok := optionList(line, "only"); ok {
		options.actions = actions
	} else if excluded, ok := optionList(line, "except"); ok {
		for action := range excluded {
			delete(options.actions, action)
		}
	}
	if value, ok := optionScalar(line, "path"); ok {
		options.path, options.hasPath = value, true
	}
	options.controller, _ = optionScalar(line, "controller")
	options.helper, _ = optionScalar(line, "as")
	options.param, _ = optionScalar(line, "param")
	options.concerns, _ = optionValues(line, "concerns")
	return options
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

func optionList(line, key string) (map[string]bool, bool) {
	values, ok := optionValues(line, key)
	if !ok {
		return nil, false
	}
	result := make(map[string]bool, len(values))
	for _, value := range values {
		result[value] = true
	}
	return result, true
}

func optionValues(line, key string) ([]string, bool) {
	raw, ok := optionRawValue(line, key)
	if !ok {
		return nil, false
	}
	raw = strings.TrimSpace(raw)
	switch {
	case strings.HasPrefix(raw, "%i[") || strings.HasPrefix(raw, "%w["):
		raw = strings.TrimSuffix(raw[3:], "]")
	case strings.HasPrefix(raw, "["):
		raw = strings.TrimSuffix(strings.TrimPrefix(raw, "["), "]")
	default:
		value := normalizeOptionScalar(raw)
		if value == "" {
			return []string{}, true
		}
		return []string{value}, true
	}
	var result []string
	for _, item := range splitOptionItems(raw) {
		if value := normalizeOptionScalar(item); value != "" {
			result = append(result, value)
		}
	}
	return result, true
}

func optionScalar(line, key string) (string, bool) {
	raw, ok := optionRawValue(line, key)
	if !ok {
		return "", false
	}
	return normalizeOptionScalar(raw), true
}

func optionRawValue(line, key string) (string, bool) {
	value, ok := optionTail(line, key)
	if !ok {
		return "", false
	}
	end := optionValueEnd(value)
	return strings.TrimSpace(value[:end]), true
}

func optionTail(line, key string) (string, bool) {
	pattern := regexp.MustCompile(`(?:\b` + regexp.QuoteMeta(key) + `\s*:|:` + regexp.QuoteMeta(key) + `\s*=>)\s*`)
	location := pattern.FindStringIndex(line)
	if location == nil {
		return "", false
	}
	value := strings.TrimSpace(line[location[1]:])
	if value == "" {
		return "", false
	}
	return value, true
}

func optionValueEnd(value string) int {
	if len(value) == 0 {
		return 0
	}
	if value[0] == '\'' || value[0] == '"' {
		quote := value[0]
		for i := 1; i < len(value); i++ {
			if value[i] == quote && value[i-1] != '\\' {
				return i + 1
			}
		}
		return len(value)
	}
	if strings.HasPrefix(value, "%i[") || strings.HasPrefix(value, "%w[") || value[0] == '[' {
		if end := strings.IndexByte(value, ']'); end >= 0 {
			return end + 1
		}
	}
	for i, char := range value {
		if char == ',' || char == ' ' || char == '\t' {
			return i
		}
	}
	return len(value)
}

func splitOptionItems(raw string) []string {
	if strings.Contains(raw, ",") {
		return strings.Split(raw, ",")
	}
	return strings.Fields(raw)
}

func normalizeOptionScalar(raw string) string {
	return strings.Trim(strings.TrimSpace(raw), ":\"' ")
}

func parseVerbDeclaration(line string) (verbDeclaration, error) {
	m := reVerbStart.FindStringSubmatch(line)
	if m == nil {
		return verbDeclaration{}, fmt.Errorf("unsupported verb route syntax: %s", line)
	}
	declaration := verbDeclaration{verb: strings.ToUpper(m[1])}
	argument, remainder, ok := consumeRouteArgument(strings.TrimSpace(m[2]))
	if !ok {
		return verbDeclaration{}, fmt.Errorf("unsupported verb route syntax: %s", line)
	}
	if strings.HasPrefix(argument, ":") {
		declaration.symbolic = true
		declaration.routePath = normalizeOptionScalar(argument)
	} else {
		declaration.routePath = normalizeOptionScalar(argument)
	}

	remainder = strings.TrimSpace(remainder)
	optionsText := remainder
	declaration.controller, _ = optionScalar(optionsText, "controller")
	declaration.action, _ = optionScalar(optionsText, "action")
	declaration.helper, _ = optionScalar(optionsText, "as")
	declaration.on, _ = optionScalar(optionsText, "on")
	declaration.constraints = hasOption(optionsText, "constraints")

	if strings.HasPrefix(remainder, "=>") {
		targetInput := strings.TrimSpace(strings.TrimPrefix(remainder, "=>"))
		if strings.HasPrefix(targetInput, "redirect") {
			redirect, _, redirectErr := parseRedirectCall(targetInput)
			if redirectErr != nil {
				declaration.dynamic = true
				declaration.dynamicRedirect = true
				return declaration, nil
			}
			declaration.redirect = &redirect
		} else {
			targetArg, _, targetOK := consumeRouteArgument(targetInput)
			if !targetOK {
				declaration.dynamic = true
				return declaration, nil
			}
			declaration.target = normalizeOptionScalar(targetArg)
		}
	}
	if toTail, ok := optionTail(optionsText, "to"); ok {
		toTail = strings.TrimSpace(toTail)
		if strings.HasPrefix(toTail, "redirect") {
			redirect, _, redirectErr := parseRedirectCall(toTail)
			if redirectErr != nil {
				declaration.dynamic = true
				declaration.dynamicRedirect = true
				return declaration, nil
			}
			declaration.redirect = &redirect
		} else if target, targetOK := optionScalar(optionsText, "to"); targetOK {
			declaration.target = target
		}
	}
	if declaration.redirect == nil && strings.Contains(optionsText, "redirect") {
		declaration.dynamic = true
		declaration.dynamicRedirect = true
		return declaration, nil
	}
	return declaration, nil
}

func parseRedirectCall(input string) (redirectDeclaration, string, error) {
	body, remainder, ok := consumeFunctionCall(input, "redirect")
	if !ok {
		return redirectDeclaration{}, "", fmt.Errorf("dynamic redirect target")
	}
	arguments := splitTopLevelArguments(body)
	if len(arguments) == 0 {
		return redirectDeclaration{}, "", fmt.Errorf("redirect has no destination")
	}

	destinationArg := strings.TrimSpace(arguments[0])
	destination, rest, literal := consumeRouteArgument(destinationArg)
	if !literal || rest != "" || len(destination) < 2 ||
		(destination[0] != '\'' && destination[0] != '"') ||
		strings.Contains(destination, "#{") {
		return redirectDeclaration{}, "", fmt.Errorf("dynamic redirect target")
	}

	redirect := redirectDeclaration{
		destination: destination[1 : len(destination)-1],
		status:      301,
	}
	for _, argument := range arguments[1:] {
		argument = strings.TrimSpace(argument)
		if !strings.HasPrefix(argument, "status:") {
			return redirectDeclaration{}, "", fmt.Errorf("dynamic redirect options")
		}
		statusText := strings.TrimSpace(strings.TrimPrefix(argument, "status:"))
		status, err := strconv.Atoi(statusText)
		if err != nil || status < 100 || status > 999 {
			return redirectDeclaration{}, "", fmt.Errorf("invalid redirect status")
		}
		redirect.status = status
	}
	return redirect, remainder, nil
}

func consumeFunctionCall(input, name string) (string, string, bool) {
	input = strings.TrimSpace(input)
	if !strings.HasPrefix(input, name) {
		return "", "", false
	}
	position := len(name)
	for position < len(input) && (input[position] == ' ' || input[position] == '\t') {
		position++
	}
	if position >= len(input) || input[position] != '(' {
		return "", "", false
	}

	start := position + 1
	depth := 1
	var quote byte
	escaped := false
	for position = start; position < len(input); position++ {
		char := input[position]
		if quote != 0 {
			if escaped {
				escaped = false
				continue
			}
			if char == '\\' {
				escaped = true
				continue
			}
			if char == quote {
				quote = 0
			}
			continue
		}
		switch char {
		case '\'', '"':
			quote = char
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				remainder := strings.TrimSpace(strings.TrimPrefix(input[position+1:], ","))
				return input[start:position], remainder, true
			}
		}
	}
	return "", "", false
}

func splitTopLevelArguments(input string) []string {
	var (
		result  []string
		start   int
		depth   int
		quote   byte
		escaped bool
	)
	for position := 0; position < len(input); position++ {
		char := input[position]
		if quote != 0 {
			if escaped {
				escaped = false
				continue
			}
			if char == '\\' {
				escaped = true
				continue
			}
			if char == quote {
				quote = 0
			}
			continue
		}
		switch char {
		case '\'', '"':
			quote = char
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
		case ',':
			if depth == 0 {
				result = append(result, strings.TrimSpace(input[start:position]))
				start = position + 1
			}
		}
	}
	result = append(result, strings.TrimSpace(input[start:]))
	return result
}

func consumeRouteArgument(input string) (string, string, bool) {
	if input == "" {
		return "", "", false
	}
	if input[0] == '\'' || input[0] == '"' {
		end := optionValueEnd(input)
		if end == len(input) && input[len(input)-1] != input[0] {
			return "", "", false
		}
		return input[:end], strings.TrimSpace(strings.TrimPrefix(input[end:], ",")), true
	}
	if input[0] == ':' {
		end := 1
		for end < len(input) && (isRouteWordByte(input[end]) || input[end] == '!' || input[end] == '?') {
			end++
		}
		return input[:end], strings.TrimSpace(strings.TrimPrefix(input[end:], ",")), end > 1
	}
	if strings.HasPrefix(input, "redirect(") {
		return input, "", true
	}
	return "", "", false
}

func isRouteWordByte(value byte) bool {
	return value == '_' || value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z' || value >= '0' && value <= '9'
}

func hasOption(line, key string) bool {
	_, ok := optionRawValue(line, key)
	return ok
}

func resolveVerbDeclaration(declaration verbDeclaration, frame routeFrame) (RouteEntry, error) {
	mode := declaration.on
	if mode == "" {
		mode = frame.routeMode
	}
	basePath := frame.pathPrefix
	switch mode {
	case "member":
		if frame.memberPath == "" {
			return RouteEntry{}, fmt.Errorf("member route outside a resource was skipped")
		}
		basePath = frame.memberPath
	case "collection":
		if frame.collectionPath == "" {
			return RouteEntry{}, fmt.Errorf("collection route outside a resource was skipped")
		}
		basePath = frame.collectionPath
	case "", "resource":
	default:
		return RouteEntry{}, fmt.Errorf("unsupported route location: %s", mode)
	}
	routePath := joinRoutePath(basePath, declaration.routePath)

	if declaration.redirect != nil {
		helper := declaration.helper
		if helper != "" {
			helper = frame.helperPrefix + helper
		}
		return RouteEntry{
			Prefix:           helper,
			Verb:             declaration.verb,
			URIPattern:       routePath,
			ControllerAction: fmt.Sprintf("redirect(%d, %s)", declaration.redirect.status, declaration.redirect.destination),
		}, nil
	}

	controller, action := resolveVerbTarget(declaration, frame)
	if controller == "" || action == "" {
		return RouteEntry{}, fmt.Errorf("could not infer controller/action for route: %s %s", declaration.verb, declaration.routePath)
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
		ControllerAction: controller + "#" + action,
	}, nil
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
	for _, definition := range defs {
		if actions[definition.action] {
			entries = append(entries, RouteEntry{
				Prefix:           definition.helper,
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

func stripInlineComment(line string) string {
	var quote rune
	escaped := false
	for index, char := range line {
		if escaped {
			escaped = false
			continue
		}
		if char == '\\' && quote != 0 {
			escaped = true
			continue
		}
		if quote != 0 {
			if char == quote {
				quote = 0
			}
			continue
		}
		if char == '\'' || char == '"' {
			quote = char
			continue
		}
		if char == '#' {
			return line[:index]
		}
	}
	return line
}

func staticWarning(path string, line int, message string) StaticWarning {
	return StaticWarning{Path: path, Line: line, Message: message}
}
