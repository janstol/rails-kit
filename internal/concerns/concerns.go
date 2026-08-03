package concerns

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ConcernDetail holds parsed information about a single concern file.
type ConcernDetail struct {
	Name                 string            `json:"name"`
	Path                 string            `json:"path"`
	Type                 string            `json:"type"`
	Methods              []string          `json:"methods,omitempty"`
	ClassMethods         []string          `json:"class_methods,omitempty"`
	HasIncludedBlock     bool              `json:"has_included_block"`
	HasClassMethodsBlock bool              `json:"has_class_methods_block"`
	ParseErrors          []ParseDiagnostic `json:"-"`
}

// ParseDiagnostic describes a recoverable Ruby syntax error reported by Prism.
type ParseDiagnostic struct {
	Line    int
	Message string
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
				names = append(names, strings.TrimSuffix(filepath.ToSlash(rel), ".rb"))
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
		return modelFull, filepath.ToSlash(rel), "model", nil
	}

	if ctrlErr == nil {
		rel, relErr := filepath.Rel(railsRoot, ctrlRel)
		if relErr != nil {
			rel = ctrlRel
		}
		return ctrlFull, filepath.ToSlash(rel), "controller", nil
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
	return fullPath, filepath.ToSlash(rel), concernType, nil
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
