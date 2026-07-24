package routes

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
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
	reUnsupported = regexp.MustCompile(`^\s*(scope|constraints?|mount|draw|concerns?|devise_\w+|direct|resolve|match)\b`)
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
	actions    map[string]bool
}

type verbDeclaration struct {
	verb        string
	routePath   string
	symbolic    bool
	target      string
	controller  string
	action      string
	helper      string
	on          string
	constraints bool
	dynamic     bool
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
	f, err := os.Open(routesPath)
	if err != nil {
		return StaticResult{}, err
	}
	defer f.Close() //nolint:errcheck

	var (
		result StaticResult
		stack  = []routeFrame{{depth: -1}}
		depth  int
	)
	currentFrame := func() routeFrame { return stack[len(stack)-1] }

	scanner := bufio.NewScanner(f)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := scanner.Text()
		trimmed := strings.TrimSpace(stripInlineComment(line))
		if trimmed == "" {
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
				result.Warnings = append(result.Warnings, staticWarning(lineNumber, "member block outside a resource was skipped"))
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
				result.Warnings = append(result.Warnings, staticWarning(lineNumber, "collection block outside a resource was skipped"))
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
				result.Warnings = append(result.Warnings, staticWarning(lineNumber, "unsupported scope module syntax: "+trimmed))
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
			memberHelper := cf.helperPrefix + p.Singularize(helperBase)
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

			if hasDo {
				singularName := p.Singularize(name)
				memberPath := collectionPath
				nestedPath := collectionPath
				if !singularResource {
					memberPath = joinRoutePath(collectionPath, ":"+param)
					nestedPath = joinRoutePath(collectionPath, ":"+singularName+"_"+param)
				}
				frame := cf
				frame.depth = depth
				frame.pathPrefix = nestedPath
				frame.helperPrefix = memberHelper + "_"
				frame.collectionPath = collectionPath
				frame.memberPath = memberPath
				frame.resourcePath = nestedPath
				frame.resourceName = helperBase
				frame.resourceSingular = p.Singularize(helperBase)
				frame.resourceParam = param
				frame.resourceController = resourceController
				frame.collectionHelper = collectionHelper
				frame.memberHelper = memberHelper
				frame.routeMode = "resource"
				frame.defaultController = resourceController
				stack = append(stack, frame)
				depth++
			}
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

		if reVerbStart.MatchString(trimmed) {
			declaration, parseErr := parseVerbDeclaration(trimmed)
			if parseErr != nil {
				result.Warnings = append(result.Warnings, staticWarning(lineNumber, parseErr.Error()))
				continue
			}
			if declaration.dynamic {
				result.Warnings = append(result.Warnings, staticWarning(lineNumber, "dynamic route target is not modeled: "+trimmed))
				continue
			}
			entry, resolveErr := resolveVerbDeclaration(declaration, currentFrame())
			if resolveErr != nil {
				result.Warnings = append(result.Warnings, staticWarning(lineNumber, resolveErr.Error()))
				continue
			}
			result.Entries = append(result.Entries, entry)
			if declaration.constraints {
				result.Warnings = append(result.Warnings, staticWarning(lineNumber, "route constraints are not modeled"))
			}
			continue
		}

		if reUnsupported.MatchString(trimmed) {
			result.Warnings = append(result.Warnings, staticWarning(lineNumber, "unsupported route DSL: "+trimmed))
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
		return map[string]bool{normalizeOptionScalar(raw): true}, true
	}
	result := make(map[string]bool)
	for _, item := range splitOptionItems(raw) {
		if value := normalizeOptionScalar(item); value != "" {
			result[value] = true
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
	pattern := regexp.MustCompile(`(?:\b` + regexp.QuoteMeta(key) + `\s*:|:` + regexp.QuoteMeta(key) + `\s*=>)\s*`)
	location := pattern.FindStringIndex(line)
	if location == nil {
		return "", false
	}
	value := strings.TrimSpace(line[location[1]:])
	if value == "" {
		return "", false
	}
	end := optionValueEnd(value)
	return strings.TrimSpace(value[:end]), true
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
	if strings.HasPrefix(remainder, "=>") {
		targetArg, rest, targetOK := consumeRouteArgument(strings.TrimSpace(strings.TrimPrefix(remainder, "=>")))
		if !targetOK || strings.HasPrefix(targetArg, "redirect") {
			declaration.dynamic = true
			return declaration, nil
		}
		declaration.target = normalizeOptionScalar(targetArg)
		remainder = rest
	}
	if target, ok := optionScalar(remainder, "to"); ok {
		declaration.target = target
	}
	declaration.controller, _ = optionScalar(remainder, "controller")
	declaration.action, _ = optionScalar(remainder, "action")
	declaration.helper, _ = optionScalar(remainder, "as")
	declaration.on, _ = optionScalar(remainder, "on")
	declaration.constraints = hasOption(remainder, "constraints")
	return declaration, nil
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
	controller, action := resolveVerbTarget(declaration, frame)
	if controller == "" || action == "" {
		return RouteEntry{}, fmt.Errorf("could not infer controller/action for route: %s %s", declaration.verb, declaration.routePath)
	}

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

func staticWarning(line int, message string) StaticWarning {
	return StaticWarning{Line: line, Message: message}
}
