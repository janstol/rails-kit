package locales

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const maxLeafLength = 120

// Load deep-merges all YAML files in the locales directory into a single map.
// Returns an error if any file cannot be read or parsed.
func Load(localesDir string) (map[string]interface{}, error) {
	info, err := os.Stat(localesDir)
	if err != nil {
		return nil, fmt.Errorf("locales path %s: %w", localesDir, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("locales path %s: not a directory", localesDir)
	}

	var allFiles []string
	if err := filepath.WalkDir(localesDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && (strings.HasSuffix(path, ".yml") || strings.HasSuffix(path, ".yaml")) {
			allFiles = append(allFiles, path)
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("walking locales dir: %w", err)
	}
	sort.Strings(allFiles)

	merged := make(map[string]interface{})
	for _, path := range allFiles {
		rel, relErr := filepath.Rel(localesDir, path)
		if relErr != nil {
			rel = filepath.Base(path)
		}
		rel = filepath.ToSlash(rel)
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", rel, err)
		}
		var parsed map[string]interface{}
		if err := yaml.Unmarshal(data, &parsed); err != nil {
			return nil, fmt.Errorf("parsing %s: %w", rel, err)
		}
		merged = deepMerge(merged, parsed)
	}
	return merged, nil
}

// ListScopes prints nested scopes from the merged locale map (e.g., en.views.users, en.time.formats).
func ListScopes(merged map[string]interface{}) []string {
	var scopes []string
	collectScopes(merged, "", 1, &scopes)
	return scopes
}

// Navigate walks the merged map along the dot-separated scope path.
func Navigate(merged map[string]interface{}, scope string) (interface{}, error) {
	keys := strings.Split(scope, ".")
	var node interface{} = merged
	for _, key := range keys {
		m, ok := node.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("scope '%s' not found (key '%s' is not a map)", scope, key)
		}
		v, ok := m[key]
		if !ok {
			return nil, fmt.Errorf("scope '%s' not found (missing key '%s')", scope, key)
		}
		node = v
	}
	return node, nil
}

// PrintTree writes the value tree to a strings.Builder with 2-space indentation.
func PrintTree(sb *strings.Builder, value interface{}, indent int) {
	printNode(sb, "", value, indent)
}

func printNode(sb *strings.Builder, key string, value interface{}, indent int) {
	prefix := strings.Repeat("  ", indent)
	switch v := value.(type) {
	case map[string]interface{}:
		if key != "" {
			sb.WriteString(prefix + key + ":\n")
			indent++
		}
		for _, childKey := range sortedKeys(v) {
			printNode(sb, childKey, v[childKey], indent)
		}
	case []interface{}:
		if key != "" {
			sb.WriteString(prefix + key + ":\n")
		}
		listIndent := indent
		if key != "" {
			listIndent++
		}
		listPrefix := strings.Repeat("  ", listIndent)
		for _, item := range v {
			switch child := item.(type) {
			case map[string]interface{}:
				sb.WriteString(listPrefix + "-\n")
				printNode(sb, "", child, listIndent+1)
			case []interface{}:
				sb.WriteString(listPrefix + "-\n")
				printNode(sb, "", child, listIndent+1)
			default:
				sb.WriteString(listPrefix + "- " + formatLeaf(child) + "\n")
			}
		}
	default:
		leaf := formatLeaf(v)
		if key != "" {
			sb.WriteString(prefix + key + ": " + leaf + "\n")
			return
		}
		sb.WriteString(prefix + leaf + "\n")
	}
}

func formatLeaf(value interface{}) string {
	leaf := fmt.Sprintf("%v", value)
	if runes := []rune(leaf); len(runes) > maxLeafLength {
		leaf = string(runes[:maxLeafLength]) + "..."
	}
	return leaf
}

func deepMerge(base, other map[string]interface{}) map[string]interface{} {
	if base == nil {
		base = make(map[string]interface{})
	}
	result := make(map[string]interface{}, len(base))
	for k, v := range base {
		result[k] = v
	}
	for k, v := range other {
		if bv, ok := result[k]; ok {
			bMap, bIsMap := bv.(map[string]interface{})
			vMap, vIsMap := v.(map[string]interface{})
			if bIsMap && vIsMap {
				result[k] = deepMerge(bMap, vMap)
				continue
			}
		}
		result[k] = v
	}
	return result
}

func sortedKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func collectScopes(node map[string]interface{}, prefix string, depth int, scopes *[]string) {
	for _, key := range sortedKeys(node) {
		child, ok := node[key].(map[string]interface{})
		if !ok {
			continue
		}
		path := key
		if prefix != "" {
			path = prefix + "." + key
		}
		if depth >= 2 {
			*scopes = append(*scopes, path)
		}
		collectScopes(child, path, depth+1, scopes)
	}
}
