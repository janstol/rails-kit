package model

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/janstol/rails-kit/internal/config"
	"github.com/janstol/rails-kit/internal/term"
)

var (
	// Used in underscore
	reHasUpper        = regexp.MustCompile(`[A-Z]`)
	reAcronymBoundary = regexp.MustCompile(`([A-Z\d]+)([A-Z][a-z])`)
	reWordBoundary    = regexp.MustCompile(`([a-z\d])([A-Z])`)
)

// Summary holds the extracted model structure.
type Summary struct {
	ClassName   string
	ParentClass string
	RelPath     string
	TableName   string
	Concerns    []string
	Assocs      []string
	Valids      []string
	Scopes      []string
	Callbacks   []string
	Enums       []string
	Delegates   []string
	ParseErrors []ParseDiagnostic
}

// ParseDiagnostic describes a recoverable Ruby syntax error reported by Prism.
type ParseDiagnostic struct {
	Line    int
	Message string
}

var errAmbiguousModelName = errors.New("ambiguous model name")

// IsAmbiguousError reports whether err indicates multiple model matches.
func IsAmbiguousError(err error) bool {
	return errors.Is(err, errAmbiguousModelName)
}

// Resolve finds the model file for the given name or path within railsRoot.
func Resolve(railsRoot, modelsPath, input string) (string, error) {
	modelsDir := config.ResolvePath(railsRoot, modelsPath)

	if strings.HasSuffix(input, ".rb") {
		path := input
		if !filepath.IsAbs(path) {
			abs, err := filepath.Abs(path)
			if err == nil {
				path = abs
			}
		}
		if _, err := os.Stat(path); err == nil {
			rel, err := filepath.Rel(modelsDir, path)
			if err != nil || strings.HasPrefix(rel, "..") {
				return "", fmt.Errorf("model file is outside models directory: %s", path)
			}
			return path, nil
		}
		cleanInput := input
		if !filepath.IsAbs(modelsPath) {
			prefix := filepath.ToSlash(filepath.Clean(modelsPath)) + "/"
			cleanInput = strings.TrimPrefix(filepath.ToSlash(cleanInput), prefix)
		}
		path3 := filepath.Join(modelsDir, cleanInput)
		if _, err := os.Stat(path3); err == nil {
			return path3, nil
		}
		path2 := filepath.Join(railsRoot, input)
		if _, err := os.Stat(path2); err == nil {
			rel, err := filepath.Rel(modelsDir, path2)
			if err != nil || strings.HasPrefix(rel, "..") {
				return "", fmt.Errorf("model file is outside models directory: %s", path2)
			}
			return path2, nil
		}
		return "", fmt.Errorf("model file not found: %s", input)
	}

	name := underscore(input)
	normalizedName := normalizeLookupName(name)
	target := normalizedName + ".rb"
	basenameOnly := filepath.Base(normalizedName) == normalizedName
	targetBase := filepath.Base(target)
	info, err := os.Stat(modelsDir)
	if err != nil {
		return "", fmt.Errorf("models path %s: %w", modelsDir, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("models path %s: not a directory", modelsDir)
	}
	var found string
	var matches []string
	err = filepath.WalkDir(modelsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		rel, relErr := filepath.Rel(modelsDir, path)
		if relErr != nil {
			return nil
		}
		if rel == target {
			found = path
			return fs.SkipAll
		}
		if basenameOnly && d.Name() == targetBase {
			matches = append(matches, path)
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("walking models path %s: %w", modelsDir, err)
	}
	if found != "" {
		return found, nil
	}
	if len(matches) > 1 {
		var relNames []string
		for _, m := range matches {
			if r, e := filepath.Rel(modelsDir, m); e == nil {
				relNames = append(relNames, filepath.ToSlash(r))
			} else {
				relNames = append(relNames, m)
			}
		}
		return "", fmt.Errorf("%w '%s', matches: %s", errAmbiguousModelName, input, strings.Join(relNames, ", "))
	}
	if len(matches) == 1 {
		return matches[0], nil
	}
	return "", fmt.Errorf("model file not found for '%s'", input)
}

// ListNames returns sorted snake_case model names (relative to the models
// directory, without .rb) for every model file under railsRoot's models path.
// Returns nil, nil if the models directory does not exist.
func ListNames(railsRoot, modelsPath string) ([]string, error) {
	modelsDir := config.ResolvePath(railsRoot, modelsPath)

	info, err := os.Stat(modelsDir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("models path %s: %w", modelsDir, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("models path %s: not a directory", modelsDir)
	}

	var names []string
	err = filepath.WalkDir(modelsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, ".rb") {
			rel, relErr := filepath.Rel(modelsDir, path)
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

// underscore converts a CamelCase or namespaced class name into its Rails
// autoloading-style snake_case path, mirroring ActiveSupport's `underscore`.
// Inputs without uppercase letters (already snake_case) are returned unchanged.
func underscore(input string) string {
	if !reHasUpper.MatchString(input) {
		return input
	}
	result := strings.ReplaceAll(input, "::", "/")
	result = reAcronymBoundary.ReplaceAllString(result, "${1}_${2}")
	result = reWordBoundary.ReplaceAllString(result, "${1}_${2}")
	return strings.ToLower(result)
}

func normalizeLookupName(name string) string {
	replaced := strings.ReplaceAll(name, "\\", string(filepath.Separator))
	replaced = strings.ReplaceAll(replaced, "/", string(filepath.Separator))
	return filepath.Clean(replaced)
}

// macroAllowlist holds the Rails macros whose entry line gets a color
// accent in Format. Bare-name sections (Concerns, Scopes, Enums) are
// intentionally excluded — their section label already carries the accent.
var macroAllowlist = map[string]bool{
	"has_many":                true,
	"has_one":                 true,
	"belongs_to":              true,
	"has_and_belongs_to_many": true,
	"validates":               true,
	"validate":                true,
	"delegate":                true,
}

func isMacroToken(tok string) bool {
	if macroAllowlist[tok] {
		return true
	}
	return strings.HasPrefix(tok, "before_") || strings.HasPrefix(tok, "after_") || strings.HasPrefix(tok, "around_")
}

// styleEntry colors the leading macro keyword of a "  macro ..." entry line
// produced by Parse, leaving the rest of the line untouched. Lines whose
// first token is not a known macro (bare names like concern or scope
// entries) pass through unchanged.
func styleEntry(entry string, st term.Styler) string {
	const indent = "  "
	if !strings.HasPrefix(entry, indent) {
		return entry
	}
	rest := entry[len(indent):]
	tok := rest
	if idx := strings.IndexByte(rest, ' '); idx >= 0 {
		tok = rest[:idx]
	}
	if !isMacroToken(tok) {
		return entry
	}
	return indent + st.Cyan(tok) + rest[len(tok):]
}

// Format renders the summary as a human-readable string. st controls
// terminal color accents; the zero value renders identically to the
// uncolored output.
func Format(s *Summary, st term.Styler) string {
	var sb strings.Builder
	sb.WriteString(st.Bold(s.ClassName))
	if s.ParentClass != "" {
		sb.WriteString(" < " + st.Cyan(s.ParentClass))
	}
	sb.WriteString(" " + st.Dim("("+s.RelPath+")") + "\n")
	sb.WriteString(st.Dim(strings.Repeat("=", 40)) + "\n")
	if s.TableName != "" {
		sb.WriteString("\n")
		sb.WriteString(st.Bold("Table:") + "\n")
		sb.WriteString("  " + s.TableName + "\n")
	}

	sections := []struct {
		label   string
		entries []string
	}{
		{"Concerns", s.Concerns},
		{"Associations", s.Assocs},
		{"Validations", s.Valids},
		{"Scopes", s.Scopes},
		{"Callbacks", s.Callbacks},
		{"Enums", s.Enums},
		{"Delegates", s.Delegates},
	}
	for _, sec := range sections {
		if len(sec.entries) == 0 {
			continue
		}
		sb.WriteString("\n")
		sb.WriteString(st.Bold(sec.label+":") + "\n")
		for _, e := range sec.entries {
			sb.WriteString(styleEntry(e, st) + "\n")
		}
	}
	sb.WriteString("\n")
	return sb.String()
}
