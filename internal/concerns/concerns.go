package concerns

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var (
	reModule       = regexp.MustCompile(`^\s*module\s+([A-Z]\w*(?:::[A-Z]\w*)*)`)
	reIncludedDo   = regexp.MustCompile(`^\s*included\s+do\b`)
	reClassMethods = regexp.MustCompile(`^\s*class_methods\s+do\b`)
	reDef          = regexp.MustCompile(`^\s*def\s+(\w+[?!=]?)`)
	reEnd          = regexp.MustCompile(`^\s*end\b`)
)

// ConcernDetail holds parsed information about a single concern file.
type ConcernDetail struct {
	Name                 string   `json:"name"`
	Path                 string   `json:"path"`
	Type                 string   `json:"type"`
	Methods              []string `json:"methods,omitempty"`
	ClassMethods         []string `json:"class_methods,omitempty"`
	HasIncludedBlock     bool     `json:"has_included_block"`
	HasClassMethodsBlock bool     `json:"has_class_methods_block"`
}

// ListFiles returns sorted snake_case names (without .rb extension) of all concern
// files in dir, including nested subdirectories.
// Returns nil, nil if dir does not exist (concerns are optional).
func ListFiles(dir string) ([]string, error) {
	info, err := os.Stat(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("concerns path %s: %w", dir, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("concerns path %s: not a directory", dir)
	}

	var names []string
	err = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, ".rb") {
			rel, relErr := filepath.Rel(dir, path)
			if relErr == nil {
				names = append(names, strings.TrimSuffix(rel, ".rb"))
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

// Parse reads a concern .rb file and extracts structural information.
func Parse(filePath, relPath, concernType string) (*ConcernDetail, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("opening concern file: %w", err)
	}
	defer f.Close() //nolint:errcheck

	d := &ConcernDetail{
		Path: relPath,
		Type: concernType,
	}

	// Track nesting depth to detect when we exit class_methods block.
	// depth 0 = top level, we increment on "do"/"begin" blocks, decrement on "end".
	// When inside class_methods block, depth > classMethodsDepth means we're nested further.
	var (
		depth             int
		inClassMethods    bool
		classMethodsDepth int
	)

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		// Skip comments and blank lines
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		if m := reModule.FindStringSubmatch(line); m != nil {
			if d.Name == "" {
				d.Name = m[1]
			}
		}

		if reIncludedDo.MatchString(line) {
			d.HasIncludedBlock = true
		}

		if reClassMethods.MatchString(line) {
			d.HasClassMethodsBlock = true
			inClassMethods = true
			classMethodsDepth = depth
		}

		if m := reDef.FindStringSubmatch(line); m != nil {
			if inClassMethods {
				d.ClassMethods = append(d.ClassMethods, m[1])
			} else {
				d.Methods = append(d.Methods, m[1])
			}
		}

		// Track nesting: count "do" / "begin" on non-def lines (def itself opens a block
		// that ends with end, so we count it), and count "end".
		// Simpler approach: count all block-openers and end keywords.
		if reEnd.MatchString(trimmed) && trimmed == "end" {
			if inClassMethods && depth == classMethodsDepth+1 {
				inClassMethods = false
			}
			if depth > 0 {
				depth--
			}
		} else {
			// Count block openers: do, begin, def, class, module, if/unless/until/while/for/case
			opens := countBlockOpeners(trimmed)
			depth += opens
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading concern file: %w", err)
	}

	return d, nil
}

// countBlockOpeners counts how many block-opening keywords are on a line.
// This is an approximation sufficient for typical Rails concern files.
func countBlockOpeners(line string) int {
	// Lines ending with "do" open a block
	count := 0
	if strings.HasSuffix(line, " do") || strings.HasSuffix(line, "\tdo") ||
		line == "do" || strings.Contains(line, " do |") || strings.Contains(line, " do\t") {
		count++
	}
	// def, class, module each open a block
	if strings.HasPrefix(line, "def ") || strings.HasPrefix(line, "def\t") {
		count++
	}
	if strings.HasPrefix(line, "class ") || strings.HasPrefix(line, "module ") {
		count++
	}
	// begin
	if line == "begin" || strings.HasPrefix(line, "begin ") {
		count++
	}
	// if/unless/until/while/for/case (single-line ternary if doesn't open a block)
	for _, kw := range []string{"if ", "unless ", "until ", "while ", "for ", "case "} {
		if strings.HasPrefix(line, kw) {
			count++
			break
		}
	}
	return count
}

// FindConcern locates a concern file by snake_case name.
// Supports qualified names like "model/searchable" or "controller/searchable".
// Checks modelDir first, then controllerDir.
// Returns fullPath, relPath (from railsRoot), concernType, and error.
func FindConcern(modelDir, controllerDir, railsRoot, name string) (string, string, string, error) {
	// Handle qualified names
	if strings.HasPrefix(name, "model/") {
		bareName := strings.TrimPrefix(name, "model/")
		return findInDir(modelDir, railsRoot, bareName, "model")
	}
	if strings.HasPrefix(name, "controller/") {
		bareName := strings.TrimPrefix(name, "controller/")
		return findInDir(controllerDir, railsRoot, bareName, "controller")
	}

	// Unqualified: check both dirs first to detect ambiguity.
	modelFull, modelRel, modelErr := findFile(modelDir, name)
	ctrlFull, ctrlRel, ctrlErr := findFile(controllerDir, name)

	if modelErr == nil && ctrlErr == nil {
		return "", "", "", fmt.Errorf("concern %q exists in both model and controller concerns — use 'model/%s' or 'controller/%s' to disambiguate", name, name, name)
	}

	if modelErr == nil {
		rel, relErr := filepath.Rel(railsRoot, modelRel)
		if relErr != nil {
			rel = modelRel
		}
		return modelFull, rel, "model", nil
	}

	if ctrlErr == nil {
		rel, relErr := filepath.Rel(railsRoot, ctrlRel)
		if relErr != nil {
			rel = ctrlRel
		}
		return ctrlFull, rel, "controller", nil
	}

	return "", "", "", fmt.Errorf("concern %q not found", name)
}

func findInDir(dir, railsRoot, name, concernType string) (string, string, string, error) {
	fullPath, relPath, err := findFile(dir, name)
	if err != nil {
		return "", "", "", fmt.Errorf("concern %q not found in %s concerns", name, concernType)
	}
	rel, relErr := filepath.Rel(railsRoot, relPath)
	if relErr != nil {
		rel = relPath
	}
	return fullPath, rel, concernType, nil
}

// findFile looks for name.rb in dir, supporting namespaced names like "admin/searchable".
func findFile(dir, name string) (fullPath, absPath string, err error) {
	candidate := filepath.Join(dir, name+".rb")
	if _, statErr := os.Stat(candidate); statErr == nil {
		return candidate, candidate, nil
	}
	return "", "", fmt.Errorf("not found")
}

// Format returns a human-readable text representation of a ConcernDetail.
func Format(d *ConcernDetail) string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "%s (%s)\n", d.Name, d.Path)
	sb.WriteString(strings.Repeat("=", 40) + "\n")
	fmt.Fprintf(&sb, "Type: %s\n", d.Type)
	fmt.Fprintf(&sb, "Included block: %s\n", yesNo(d.HasIncludedBlock))

	if len(d.Methods) > 0 {
		sb.WriteString("\nMethods:\n")
		for _, m := range d.Methods {
			fmt.Fprintf(&sb, "  %s\n", m)
		}
	}

	if len(d.ClassMethods) > 0 {
		sb.WriteString("\nClass methods:\n")
		for _, m := range d.ClassMethods {
			fmt.Fprintf(&sb, "  %s\n", m)
		}
	}

	return sb.String()
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}
