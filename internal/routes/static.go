package routes

import (
	"bufio"
	"os"
	"regexp"
	"strings"

	"github.com/janstol/rails-kit/internal/pluralize"
)

var (
	reNamespace   = regexp.MustCompile(`^\s*namespace\s+:(\w+)`)
	reResources   = regexp.MustCompile(`^\s*resources?\s+:(\w+)`)
	reOnly        = regexp.MustCompile(`only:\s*\[([^\]]+)\]`)
	reExcept      = regexp.MustCompile(`except:\s*\[([^\]]+)\]`)
	reVerbRoute   = regexp.MustCompile(`^\s*(get|post|put|patch|delete)\s+['"]([^'"]+)['"].*?to:\s*['"]([^'"]+)['"]`)
	reRoot        = regexp.MustCompile(`^\s*root\s+(?:to:\s*)?['"]([^'"]+)['"]`)
	reBlockEnd    = regexp.MustCompile(`^\s*end\b`)
	reBlockOpener = regexp.MustCompile(`\bdo\b`)
)

// routeFrame holds path and controller namespace context for a nesting level.
type routeFrame struct {
	depth            int    // block depth when this frame was pushed
	pathPrefix       string // accumulated URL prefix, e.g. "/admin"
	controllerPrefix string // accumulated controller namespace, e.g. "admin/"
}

// ParseStatic parses config/routes.rb and returns a slice of RouteEntry.
// It handles resources, namespace, root, and individual verb routes.
// Blocks not explicitly handled (scope, member, collection, etc.) are
// depth-tracked so that nested resources inside them resolve correctly.
func ParseStatic(routesPath string, p *pluralize.Pluralizer) ([]RouteEntry, error) {
	f, err := os.Open(routesPath)
	if err != nil {
		return nil, err
	}
	defer f.Close() //nolint:errcheck

	var (
		entries []RouteEntry
		stack   []routeFrame
		depth   int
	)

	// Start with a root frame.
	stack = append(stack, routeFrame{depth: -1, pathPrefix: "", controllerPrefix: ""})

	currentFrame := func() routeFrame {
		return stack[len(stack)-1]
	}

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
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
			entries = append(entries, newEntries...)

			if hasDo {
				// Nested resources: path prefix gains "/:name_singular_id"
				singularName := p.Singularize(name)
				if singular {
					stack = append(stack, routeFrame{
						depth:            depth,
						pathPrefix:       cf.pathPrefix + "/" + name,
						controllerPrefix: cf.controllerPrefix,
					})
				} else {
					stack = append(stack, routeFrame{
						depth:            depth,
						pathPrefix:       cf.pathPrefix + "/" + name + "/:" + singularName + "_id",
						controllerPrefix: cf.controllerPrefix,
					})
				}
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
				entries = append(entries, RouteEntry{
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
				entries = append(entries, RouteEntry{
					Prefix:           "",
					Verb:             verb,
					URIPattern:       fullPath,
					ControllerAction: controller + "#" + parts[1],
				})
			}
			continue
		}

		// Track depth for any other block openers we don't handle specially
		// (member, collection, scope, etc.). All do/end pairs must balance.
		if hasDo {
			depth++
		}
	}

	return entries, scanner.Err()
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
