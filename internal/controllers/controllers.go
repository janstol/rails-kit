// Package controllers extracts a structural summary of a Rails controller
// file -- filters, rescue_from handlers, helper methods, layout, respond_to
// formats, strong params, and action methods -- without booting Rails.
//
// Only the controller's own file is parsed; filters inherited from
// ApplicationController (or any other superclass) are not resolved. Summary
// reports ParentClass so callers know where to look next.
package controllers

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
	reHasUpper        = regexp.MustCompile(`[A-Z]`)
	reAcronymBoundary = regexp.MustCompile(`([A-Z\d]+)([A-Z][a-z])`)
	reWordBoundary    = regexp.MustCompile(`([a-z\d])([A-Z])`)
)

// Summary holds the extracted controller structure.
type Summary struct {
	ClassName     string
	ParentClass   string
	RelPath       string
	Concerns      []string
	Filters       []string
	RescueFrom    []string
	HelperMethods []string
	Layout        string
	RespondTo     []string
	StrongParams  []string
	Actions       []string
	ParseErrors   []ParseDiagnostic
}

// ParseDiagnostic describes a recoverable Ruby syntax error reported by Prism.
type ParseDiagnostic struct {
	Line    int
	Message string
}

var errAmbiguousControllerName = errors.New("ambiguous controller name")

// IsAmbiguousError reports whether err indicates multiple controller matches.
func IsAmbiguousError(err error) bool {
	return errors.Is(err, errAmbiguousControllerName)
}

// Resolve finds the controller file for the given name or path within railsRoot.
// The name may be a short resource name ("users"), the full file basename
// ("users_controller"), a CamelCase class name ("UsersController",
// "Admin::ReportsController"), or a file path ending in .rb.
func Resolve(railsRoot, controllersPath, input string) (string, error) {
	controllersDir := config.ResolvePath(railsRoot, controllersPath)

	if strings.HasSuffix(input, ".rb") {
		path := input
		if !filepath.IsAbs(path) {
			abs, err := filepath.Abs(path)
			if err == nil {
				path = abs
			}
		}
		if _, err := os.Stat(path); err == nil {
			rel, err := filepath.Rel(controllersDir, path)
			if err != nil || strings.HasPrefix(rel, "..") {
				return "", fmt.Errorf("controller file is outside controllers directory: %s", path)
			}
			return path, nil
		}
		cleanInput := input
		if !filepath.IsAbs(controllersPath) {
			prefix := filepath.ToSlash(filepath.Clean(controllersPath)) + "/"
			cleanInput = strings.TrimPrefix(filepath.ToSlash(cleanInput), prefix)
		}
		path3 := filepath.Join(controllersDir, cleanInput)
		if _, err := os.Stat(path3); err == nil {
			return path3, nil
		}
		path2 := filepath.Join(railsRoot, input)
		if _, err := os.Stat(path2); err == nil {
			rel, err := filepath.Rel(controllersDir, path2)
			if err != nil || strings.HasPrefix(rel, "..") {
				return "", fmt.Errorf("controller file is outside controllers directory: %s", path2)
			}
			return path2, nil
		}
		return "", fmt.Errorf("controller file not found: %s", input)
	}

	name := underscore(input)
	normalizedName := normalizeLookupName(name)
	info, err := os.Stat(controllersDir)
	if err != nil {
		return "", fmt.Errorf("controllers path %s: %w", controllersDir, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("controllers path %s: not a directory", controllersDir)
	}

	// Prefer the conventional "*_controller.rb" file. Fall back to the name
	// exactly as given -- a handful of real apps keep .rb files directly
	// under app/controllers (outside concerns/) that don't follow that
	// convention, and ListNames surfaces those names unmodified.
	suffixed := normalizedName
	if !strings.HasSuffix(suffixed, "_controller") {
		suffixed += "_controller"
	}
	candidates := []string{suffixed + ".rb"}
	if suffixed != normalizedName {
		candidates = append(candidates, normalizedName+".rb")
	}

	for _, target := range candidates {
		found, matches, err := findControllerFile(controllersDir, target)
		if err != nil {
			return "", err
		}
		if found != "" {
			return found, nil
		}
		if len(matches) > 1 {
			var relNames []string
			for _, m := range matches {
				if r, e := filepath.Rel(controllersDir, m); e == nil {
					relNames = append(relNames, filepath.ToSlash(r))
				} else {
					relNames = append(relNames, m)
				}
			}
			return "", fmt.Errorf("%w '%s', matches: %s", errAmbiguousControllerName, input, strings.Join(relNames, ", "))
		}
		if len(matches) == 1 {
			return matches[0], nil
		}
	}
	return "", fmt.Errorf("controller file not found for '%s'", input)
}

// findControllerFile looks for target (a controllers-dir-relative path) under
// controllersDir. If target has no path separators, it also matches by
// basename anywhere in the tree, returning every match so the caller can
// detect ambiguity.
func findControllerFile(controllersDir, target string) (found string, matches []string, err error) {
	basenameOnly := filepath.Base(target) == target
	targetBase := filepath.Base(target)
	err = filepath.WalkDir(controllersDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		rel, relErr := filepath.Rel(controllersDir, path)
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
		return "", nil, fmt.Errorf("walking controllers path %s: %w", controllersDir, err)
	}
	return found, matches, nil
}

// ListNames returns sorted snake_case controller names (relative to the
// controllers directory, without .rb, with the "_controller" suffix
// stripped) for every controller file under railsRoot's controllers path.
// Files under controllerConcernsPath are excluded -- they hold concern
// modules, not controllers. Returns nil, nil if the controllers directory
// does not exist.
func ListNames(railsRoot, controllersPath, controllerConcernsPath string) ([]string, error) {
	controllersDir := config.ResolvePath(railsRoot, controllersPath)
	concernsDir := config.ResolvePath(railsRoot, controllerConcernsPath)

	info, err := os.Stat(controllersDir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("controllers path %s: %w", controllersDir, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("controllers path %s: not a directory", controllersDir)
	}

	var names []string
	err = filepath.WalkDir(controllersDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path == concernsDir {
				return fs.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, ".rb") {
			rel, relErr := filepath.Rel(controllersDir, path)
			if relErr == nil {
				name := strings.TrimSuffix(filepath.ToSlash(rel), ".rb")
				name = strings.TrimSuffix(name, "_controller")
				names = append(names, name)
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

// macroAllowlist holds the controller macros whose entry line gets a color
// accent in Format. Bare-name sections (Concerns, HelperMethods, RespondTo,
// Actions) are intentionally excluded -- their section label already carries
// the accent.
var macroAllowlist = map[string]bool{
	"rescue_from": true,
}

func isMacroToken(tok string) bool {
	if macroAllowlist[tok] {
		return true
	}
	return strings.HasPrefix(tok, "before_") || strings.HasPrefix(tok, "after_") || strings.HasPrefix(tok, "around_") ||
		strings.HasPrefix(tok, "skip_before_") || strings.HasPrefix(tok, "skip_after_") || strings.HasPrefix(tok, "skip_around_")
}

// styleEntry colors the leading macro keyword of a "  macro ..." entry line
// produced by Parse, leaving the rest of the line untouched. Lines whose
// first token is not a known macro (bare names like a helper method or
// action) pass through unchanged.
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
	if s.Layout != "" {
		sb.WriteString("\n")
		sb.WriteString(st.Bold("Layout:") + "\n")
		sb.WriteString("  " + s.Layout + "\n")
	}

	sections := []struct {
		label   string
		entries []string
	}{
		{"Concerns", s.Concerns},
		{"Filters", s.Filters},
		{"Rescue From", s.RescueFrom},
		{"Helper Methods", s.HelperMethods},
		{"Respond To", s.RespondTo},
		{"Strong Params", s.StrongParams},
		{"Actions", s.Actions},
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
