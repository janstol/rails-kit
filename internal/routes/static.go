package routes

import (
	"bufio"
	"os"
	"regexp"
	"strings"

	"github.com/janstol/rails-kit/internal/pluralize"
)

var (
	reNamespace     = regexp.MustCompile(`^\s*namespace\s+:(\w+)`)
	reResources     = regexp.MustCompile(`^\s*resources?\s+:(\w+)`)
	reOnly          = regexp.MustCompile(`only:\s*\[([^\]]+)\]`)
	reExcept        = regexp.MustCompile(`except:\s*\[([^\]]+)\]`)
	reOnlyScalar    = regexp.MustCompile(`only:\s*(:\w+|['"][^'"]+['"])`)
	reExceptScalar  = regexp.MustCompile(`except:\s*(:\w+|['"][^'"]+['"])`)
	reVerbRoute     = regexp.MustCompile(`^\s*(get|post|put|patch|delete)\s+['"]([^'"]+)['"].*?to:\s*['"]([^'"]+)['"]`)
	reRoot          = regexp.MustCompile(`^\s*root\s+(?:to:\s*)?['"]([^'"]+)['"]`)
	reBlockEnd      = regexp.MustCompile(`^\s*end\b`)
	reBlockOpener   = regexp.MustCompile(`\bdo\b`)
	reMember        = regexp.MustCompile(`^\s*member\s+do\b`)
	reCollection    = regexp.MustCompile(`^\s*collection\s+do\b`)
	reUnsupported   = regexp.MustCompile(`^\s*(scope|constraints?|mount|draw|concerns?|devise_\w+|direct|resolve|match)\b`)
	reVerbStart     = regexp.MustCompile(`^\s*(get|post|put|patch|delete)\b`)
	reIgnoredOption = regexp.MustCompile(`\b(path|controller|as):`)
)

// routeFrame holds path and controller namespace context for a nesting level.
type routeFrame struct {
	depth            int    // block depth when this frame was pushed
	pathPrefix       string // accumulated URL prefix, e.g. "/admin"
	controllerPrefix string // accumulated controller namespace, e.g. "admin/"
	collectionPath   string // current resource collection path, if any
	memberPath       string // current resource member path, if any
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

// ParseStatic parses config/routes.rb and returns route entries.
func ParseStatic(routesPath string, p *pluralize.Pluralizer) ([]RouteEntry, error) {
	result, err := ParseStaticDetailed(routesPath, p)
	if err != nil {
		return nil, err
	}
	return result.Entries, nil
}

// ParseStaticDetailed parses config/routes.rb and returns routes plus approximation warnings.
// It handles resources, namespace, root, and individual verb routes.
func ParseStaticDetailed(routesPath string, p *pluralize.Pluralizer) (StaticResult, error) {
	f, err := os.Open(routesPath)
	if err != nil {
		return StaticResult{}, err
	}
	defer f.Close() //nolint:errcheck

	var (
		result StaticResult
		stack  []routeFrame
		depth  int
	)

	// Start with a root frame.
	stack = append(stack, routeFrame{depth: -1, pathPrefix: "", controllerPrefix: ""})

	currentFrame := func() routeFrame {
		return stack[len(stack)-1]
	}

	scanner := bufio.NewScanner(f)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		// Handle end — decrement depth, pop frames that were entered at this depth.
		if reBlockEnd.MatchString(trimmed) {
			depth--
			if len(stack) > 1 && stack[len(stack)-1].depth == depth {
				stack = stack[:len(stack)-1]
			}
			continue
		}

		hasDo := reBlockOpener.MatchString(trimmed)

		// member/collection alter the path context for custom routes.
		if reMember.MatchString(line) {
			cf := currentFrame()
			if cf.memberPath == "" {
				result.Warnings = append(result.Warnings, StaticWarning{Line: lineNumber, Message: "member block outside a resource was skipped"})
			} else {
				cf.depth = depth
				cf.pathPrefix = cf.memberPath
				stack = append(stack, cf)
			}
			depth++
			continue
		}
		if reCollection.MatchString(line) {
			cf := currentFrame()
			if cf.collectionPath == "" {
				result.Warnings = append(result.Warnings, StaticWarning{Line: lineNumber, Message: "collection block outside a resource was skipped"})
			} else {
				cf.depth = depth
				cf.pathPrefix = cf.collectionPath
				stack = append(stack, cf)
			}
			depth++
			continue
		}

		// namespace :name do
		if m := reNamespace.FindStringSubmatch(line); m != nil && hasDo {
			name := m[1]
			cf := currentFrame()
			stack = append(stack, routeFrame{
				depth:            depth,
				pathPrefix:       cf.pathPrefix + "/" + name,
				controllerPrefix: cf.controllerPrefix + name + "/",
			})
			depth++
			continue
		}

		// resources :name [do] / resource :name [do]
		if m := reResources.FindStringSubmatch(line); m != nil {
			name := m[1]
			singular := strings.HasPrefix(strings.TrimSpace(line), "resource ") ||
				strings.Contains(line, "resource :")
			cf := currentFrame()
			controller := cf.controllerPrefix + name

			actions := resourceActions(line, singular)
			newEntries := expandResources(name, controller, cf.pathPrefix, actions, singular, p)
			result.Entries = append(result.Entries, newEntries...)

			if option := reIgnoredOption.FindStringSubmatch(line); option != nil {
				result.Warnings = append(result.Warnings, StaticWarning{
					Line:    lineNumber,
					Message: option[1] + ": option on resource is not modeled",
				})
			}

			if hasDo {
				singularName := p.Singularize(name)
				collectionPath := cf.pathPrefix + "/" + name
				memberPath := collectionPath
				nestedPath := collectionPath
				if !singular {
					memberPath += "/:id"
					nestedPath += "/:" + singularName + "_id"
				}
				frame := routeFrame{
					depth:            depth,
					pathPrefix:       nestedPath,
					controllerPrefix: cf.controllerPrefix,
					collectionPath:   collectionPath,
					memberPath:       memberPath,
				}
				if singular {
					frame.pathPrefix = collectionPath
				}
				stack = append(stack, frame)
				depth++
			}
			continue
		}

		// root 'controller#action'
		if m := reRoot.FindStringSubmatch(line); m != nil {
			parts := strings.SplitN(m[1], "#", 2)
			if len(parts) == 2 {
				cf := currentFrame()
				controller := cf.controllerPrefix + parts[0]
				result.Entries = append(result.Entries, RouteEntry{
					Prefix:           "root",
					Verb:             "GET",
					URIPattern:       cf.pathPrefix + "/",
					ControllerAction: controller + "#" + parts[1],
				})
			}
			continue
		}

		// get/post/put/patch/delete '/path', to: 'controller#action'
		if m := reVerbRoute.FindStringSubmatch(line); m != nil {
			verb := strings.ToUpper(m[1])
			path := m[2]
			target := m[3]
			parts := strings.SplitN(target, "#", 2)
			if len(parts) == 2 {
				cf := currentFrame()
				controller := cf.controllerPrefix + parts[0]
				fullPath := cf.pathPrefix
				if !strings.HasPrefix(path, "/") {
					fullPath += "/"
				}
				fullPath += path
				result.Entries = append(result.Entries, RouteEntry{
					Prefix:           "",
					Verb:             verb,
					URIPattern:       fullPath,
					ControllerAction: controller + "#" + parts[1],
				})
				if option := reIgnoredOption.FindStringSubmatch(line); option != nil {
					result.Warnings = append(result.Warnings, StaticWarning{
						Line:    lineNumber,
						Message: option[1] + ": option on route is not modeled",
					})
				}
			}
			continue
		}

		if reUnsupported.MatchString(line) {
			result.Warnings = append(result.Warnings, StaticWarning{Line: lineNumber, Message: "unsupported route DSL: " + trimmed})
		} else if reVerbStart.MatchString(line) {
			result.Warnings = append(result.Warnings, StaticWarning{Line: lineNumber, Message: "unsupported verb route syntax: " + trimmed})
		}

		// Track depth for any other block openers we don't handle specially
		// (member, collection, scope, etc.). All do/end pairs must balance.
		if hasDo {
			depth++
		}
	}

	if err := scanner.Err(); err != nil {
		return StaticResult{}, err
	}
	return result, nil
}

// resourceActions returns the set of RESTful actions for a resources/resource line,
// respecting only: and except: options.
func resourceActions(line string, singular bool) map[string]bool {
	all := []string{"index", "new", "create", "show", "edit", "update", "destroy"}
	if singular {
		all = []string{"new", "create", "show", "edit", "update", "destroy"}
	}

	if m := reOnly.FindStringSubmatch(line); m != nil {
		return parseActionList(m[1])
	}
	if m := reOnlyScalar.FindStringSubmatch(line); m != nil {
		return parseActionList(m[1])
	}
	if m := reExcept.FindStringSubmatch(line); m != nil {
		excluded := parseActionList(m[1])
		result := make(map[string]bool, len(all))
		for _, a := range all {
			if !excluded[a] {
				result[a] = true
			}
		}
		return result
	}
	if m := reExceptScalar.FindStringSubmatch(line); m != nil {
		excluded := parseActionList(m[1])
		result := make(map[string]bool, len(all))
		for _, a := range all {
			if !excluded[a] {
				result[a] = true
			}
		}
		return result
	}

	result := make(map[string]bool, len(all))
	for _, a := range all {
		result[a] = true
	}
	return result
}

func parseActionList(s string) map[string]bool {
	result := make(map[string]bool)
	for _, part := range strings.Split(s, ",") {
		action := strings.Trim(strings.TrimSpace(part), ":\"' ")
		if action != "" {
			result[action] = true
		}
	}
	return result
}

// expandResources generates RouteEntry records for a resources/resource declaration.
func expandResources(name, controller, pathPrefix string, actions map[string]bool, singular bool, p *pluralize.Pluralizer) []RouteEntry {
	base := pathPrefix + "/" + name
	var entries []RouteEntry

	type routeDef struct {
		verb   string
		path   string
		action string
		prefix string
	}

	var defs []routeDef
	if singular {
		defs = []routeDef{
			{"GET", base + "/new", "new", "new_" + name},
			{"GET", base + "/edit", "edit", "edit_" + name},
			{"GET", base, "show", name},
			{"POST", base, "create", name},
			{"PATCH", base, "update", name},
			{"PUT", base, "update", name},
			{"DELETE", base, "destroy", name},
		}
	} else {
		// Derive singular prefix for member routes
		singular := p.Singularize(name)
		defs = []routeDef{
			{"GET", base, "index", name},
			{"POST", base, "create", name},
			{"GET", base + "/new", "new", "new_" + singular},
			{"GET", base + "/:" + singular + "_id/edit", "edit", "edit_" + singular},
			{"GET", base + "/:" + singular + "_id", "show", singular},
			{"PATCH", base + "/:" + singular + "_id", "update", singular},
			{"PUT", base + "/:" + singular + "_id", "update", singular},
			{"DELETE", base + "/:" + singular + "_id", "destroy", singular},
		}
	}

	for _, d := range defs {
		if actions[d.action] {
			entries = append(entries, RouteEntry{
				Prefix:           d.prefix,
				Verb:             d.verb,
				URIPattern:       d.path,
				ControllerAction: controller + "#" + d.action,
			})
		}
	}

	return entries
}
