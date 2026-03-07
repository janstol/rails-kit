package fixtures

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

var interestingKeys = []string{"id", "name", "title", "email", "type", "status", "kind", "role"}

var yamlExts = []string{".yml", ".yaml"}

func isYAMLFile(path string) bool {
	return strings.HasSuffix(path, ".yml") || strings.HasSuffix(path, ".yaml")
}

func trimYAMLExt(name string) string {
	name = strings.TrimSuffix(name, ".yml")
	name = strings.TrimSuffix(name, ".yaml")
	return name
}

const maxValueLength = 60

// ListFiles returns relative paths (without .yml) of all fixture files in the directory,
// including nested subdirectories (e.g., "admin/dashboards").
func ListFiles(fixturesDir string) ([]string, error) {
	if err := validateFixturesDir(fixturesDir); err != nil {
		return nil, err
	}
	var names []string
	err := filepath.WalkDir(fixturesDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && isYAMLFile(path) {
			rel, relErr := filepath.Rel(fixturesDir, path)
			if relErr == nil {
				names = append(names, trimYAMLExt(rel))
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(names)
	return names, nil
}

// Load finds and parses the fixture file for the given name (tries plural.yml then name.yml).
// Returns the filename and parsed data.
func Load(fixturesDir, name, plural string) (string, map[string]interface{}, error) {
	if err := validateFixturesDir(fixturesDir); err != nil {
		return "", nil, err
	}
	var candidates []string
	for _, ext := range yamlExts {
		candidates = append(candidates, filepath.Join(fixturesDir, plural+ext))
		candidates = append(candidates, filepath.Join(fixturesDir, name+ext))
	}
	var path string
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			path = c
			break
		}
	}
	if path == "" {
		// Fallback: walk subdirectories to find a file matching plural.yml or name.yml by basename.
		var matches []string
		err := filepath.WalkDir(fixturesDir, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			base := filepath.Base(p)
			if base == plural+".yml" || base == name+".yml" || base == plural+".yaml" || base == name+".yaml" {
				matches = append(matches, p)
			}
			return nil
		})
		if err != nil {
			return "", nil, fmt.Errorf("walking fixtures path %s: %w", fixturesDir, err)
		}
		switch len(matches) {
		case 0:
			// handled below
		case 1:
			path = matches[0]
		default:
			var rels []string
			for _, m := range matches {
				if rel, err := filepath.Rel(fixturesDir, m); err == nil {
					rels = append(rels, rel)
				} else {
					rels = append(rels, m)
				}
			}
			return "", nil, fmt.Errorf("ambiguous fixture '%s', matches: %s", name, strings.Join(rels, ", "))
		}
	}
	if path == "" {
		return "", nil, fmt.Errorf("fixture not found for '%s' (tried %s.yml/.yaml and %s.yml/.yaml)", name, plural, name)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return "", nil, fmt.Errorf("reading fixture: %w", err)
	}
	if err := ValidateERBUsage(string(content)); err != nil {
		rel, relErr := filepath.Rel(fixturesDir, path)
		if relErr != nil {
			rel = filepath.Base(path)
		}
		return "", nil, fmt.Errorf("fixture %s uses ERB that changes structure and cannot be summarized safely: %w", rel, err)
	}

	normalized := []byte(StripERB(string(content)))

	// Try parsing normalized content first so valid YAML with ERB placeholders
	// still follows the documented "__ERB__" behavior.
	data, err := parseYAML(normalized)
	if err != nil {
		data, err = parseYAML(content)
		if err != nil {
			return "", nil, fmt.Errorf("parsing fixture: %w", err)
		}
		sanitized, ok := sanitizeRawERB(data).(map[string]interface{})
		if !ok {
			return "", nil, fmt.Errorf("fixture file is empty or has unexpected structure")
		}
		data = sanitized
	}
	if data == nil {
		return "", nil, fmt.Errorf("fixture file is empty")
	}
	normalized2, ok := normalizeERBPlaceholders(data).(map[string]interface{})
	if !ok {
		return "", nil, fmt.Errorf("fixture has unexpected structure after ERB normalization")
	}
	data = normalized2

	rel, err := filepath.Rel(fixturesDir, path)
	if err != nil {
		rel = filepath.Base(path)
	}
	return rel, data, nil
}

func validateFixturesDir(fixturesDir string) error {
	info, err := os.Stat(fixturesDir)
	if err != nil {
		return fmt.Errorf("fixtures path %s: %w", fixturesDir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("fixtures path %s: not a directory", fixturesDir)
	}
	return nil
}

// Summarize formats a fixture entry's attributes into a compact string.
func Summarize(attrs interface{}) string {
	m, ok := attrs.(map[string]interface{})
	if !ok {
		return "(empty)"
	}
	if len(m) == 0 {
		return "(empty)"
	}

	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}

	// Priority keys first
	var selected []string
	for _, k := range interestingKeys {
		if _, ok := m[k]; ok {
			selected = append(selected, k)
		}
	}
	// Fill remaining slots from non-priority keys
	prioritySet := make(map[string]bool)
	for _, k := range selected {
		prioritySet[k] = true
	}
	var rest []string
	for _, k := range keys {
		if !prioritySet[k] {
			rest = append(rest, k)
		}
	}
	sort.Strings(rest)
	for _, k := range rest {
		if len(selected) >= 5 {
			break
		}
		selected = append(selected, k)
	}

	parts := make([]string, 0, len(selected))
	for _, k := range selected {
		v := fmt.Sprintf("%v", m[k])
		v = strings.Join(strings.Fields(v), " ")
		if runes := []rune(v); len(runes) > maxValueLength {
			v = string(runes[:maxValueLength])
		}
		parts = append(parts, fmt.Sprintf("%s: %s", k, v))
	}
	return strings.Join(parts, ", ")
}

// VisibleEntries returns fixture entries excluding Rails metadata keys.
func VisibleEntries(data map[string]interface{}) map[string]interface{} {
	filtered := make(map[string]interface{}, len(data))
	for k, v := range data {
		if isFixtureMetadataKey(k) {
			continue
		}
		filtered[k] = v
	}
	return filtered
}

func isFixtureMetadataKey(key string) bool {
	return key == "_fixture"
}

// sanitizeRawERB walks parsed YAML data and replaces any string containing
// raw ERB output tags with the __ERB__ placeholder. Used as defense-in-depth
// when the fallback raw-content parse path is taken.
func sanitizeRawERB(value interface{}) interface{} {
	switch v := value.(type) {
	case map[string]interface{}:
		sanitized := make(map[string]interface{}, len(v))
		for k, child := range v {
			sanitized[k] = sanitizeRawERB(child)
		}
		return sanitized
	case []interface{}:
		sanitized := make([]interface{}, len(v))
		for i, child := range v {
			sanitized[i] = sanitizeRawERB(child)
		}
		return sanitized
	case string:
		if strings.Contains(v, "<%=") {
			return "__ERB__"
		}
		return v
	default:
		return v
	}
}

func normalizeERBPlaceholders(value interface{}) interface{} {
	switch v := value.(type) {
	case map[string]interface{}:
		normalized := make(map[string]interface{}, len(v))
		for k, child := range v {
			normalized[k] = normalizeERBPlaceholders(child)
		}
		return normalized
	case []interface{}:
		normalized := make([]interface{}, len(v))
		for i, child := range v {
			normalized[i] = normalizeERBPlaceholders(child)
		}
		return normalized
	case string:
		v = strings.ReplaceAll(v, `"__ERB__"`, "__ERB__")
		v = strings.ReplaceAll(v, `'__ERB__'`, "__ERB__")
		return v
	default:
		return v
	}
}

func parseYAML(data []byte) (map[string]interface{}, error) {
	var result map[string]interface{}
	if err := yaml.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return result, nil
}
