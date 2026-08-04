// Package mailers extracts a structural summary of a Rails mailer file -- its
// default headers, layout, included concerns, attachments, and public action
// methods -- without booting Rails.
//
// Only the mailer's own file is parsed; defaults or layout inherited from
// ApplicationMailer (or any other superclass) are not resolved. Summary
// reports ParentClass so callers know where to look next.
package mailers

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

// Summary holds the extracted mailer structure.
type Summary struct {
	ClassName   string
	ParentClass string
	RelPath     string
	Concerns    []string
	Default     []string
	Layout      string
	Attachments []string
	Methods     []string
	ParseErrors []ParseDiagnostic
}

// ParseDiagnostic describes a recoverable Ruby syntax error reported by Prism.
type ParseDiagnostic struct {
	Line    int
	Message string
}

var errAmbiguousMailerName = errors.New("ambiguous mailer name")

// IsAmbiguousError reports whether err indicates multiple mailer matches.
func IsAmbiguousError(err error) bool {
	return errors.Is(err, errAmbiguousMailerName)
}

// Resolve finds the mailer file for the given name or path within railsRoot.
// The name may be a short resource name ("users"), the full file basename
// ("user_mailer"), a CamelCase class name ("UserMailer",
// "Admin::NotificationMailer"), or a file path ending in .rb.
func Resolve(railsRoot, mailersPath, input string) (string, error) {
	mailersDir := config.ResolvePath(railsRoot, mailersPath)

	if strings.HasSuffix(input, ".rb") {
		path := input
		if !filepath.IsAbs(path) {
			abs, err := filepath.Abs(path)
			if err == nil {
				path = abs
			}
		}
		if _, err := os.Stat(path); err == nil {
			rel, err := filepath.Rel(mailersDir, path)
			if err != nil || strings.HasPrefix(rel, "..") {
				return "", fmt.Errorf("mailer file is outside mailers directory: %s", path)
			}
			return path, nil
		}
		cleanInput := input
		if !filepath.IsAbs(mailersPath) {
			prefix := filepath.ToSlash(filepath.Clean(mailersPath)) + "/"
			cleanInput = strings.TrimPrefix(filepath.ToSlash(cleanInput), prefix)
		}
		path3 := filepath.Join(mailersDir, cleanInput)
		if _, err := os.Stat(path3); err == nil {
			return path3, nil
		}
		path2 := filepath.Join(railsRoot, input)
		if _, err := os.Stat(path2); err == nil {
			rel, err := filepath.Rel(mailersDir, path2)
			if err != nil || strings.HasPrefix(rel, "..") {
				return "", fmt.Errorf("mailer file is outside mailers directory: %s", path2)
			}
			return path2, nil
		}
		return "", fmt.Errorf("mailer file not found: %s", input)
	}

	name := underscore(input)
	normalizedName := normalizeLookupName(name)
	info, err := os.Stat(mailersDir)
	if err != nil {
		return "", fmt.Errorf("mailers path %s: %w", mailersPath, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("mailers path %s: not a directory", mailersPath)
	}

	// Prefer the conventional "*_mailer.rb" file. Fall back to the name
	// exactly as given -- a handful of real apps keep .rb files directly
	// under app/mailers that don't follow that convention, and ListNames
	// surfaces those names unmodified.
	suffixed := normalizedName
	if !strings.HasSuffix(suffixed, "_mailer") {
		suffixed += "_mailer"
	}
	candidates := []string{suffixed + ".rb"}
	if suffixed != normalizedName {
		candidates = append(candidates, normalizedName+".rb")
	}

	for _, target := range candidates {
		found, matches, err := findMailerFile(mailersDir, target)
		if err != nil {
			return "", err
		}
		if found != "" {
			return found, nil
		}
		if len(matches) > 1 {
			var relNames []string
			for _, m := range matches {
				if r, e := filepath.Rel(mailersDir, m); e == nil {
					relNames = append(relNames, filepath.ToSlash(r))
				} else {
					relNames = append(relNames, m)
				}
			}
			return "", fmt.Errorf("%w '%s', matches: %s", errAmbiguousMailerName, input, strings.Join(relNames, ", "))
		}
		if len(matches) == 1 {
			return matches[0], nil
		}
	}
	return "", fmt.Errorf("mailer file not found for '%s'", input)
}

// findMailerFile looks for target (a mailers-dir-relative path) under
// mailersDir. If target has no path separators, it also matches by basename
// anywhere in the tree, returning every match so the caller can detect
// ambiguity.
func findMailerFile(mailersDir, target string) (found string, matches []string, err error) {
	basenameOnly := filepath.Base(target) == target
	targetBase := filepath.Base(target)
	err = filepath.WalkDir(mailersDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		rel, relErr := filepath.Rel(mailersDir, path)
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
		return "", nil, fmt.Errorf("walking mailers path %s: %w", mailersDir, err)
	}
	return found, matches, nil
}

// ListNames returns sorted snake_case mailer names (relative to the mailers
// directory, without .rb, with the "_mailer" suffix stripped) for every mailer
// file under railsRoot's mailers path. Returns nil, nil if the mailers
// directory does not exist.
func ListNames(railsRoot, mailersPath string) ([]string, error) {
	mailersDir := config.ResolvePath(railsRoot, mailersPath)

	info, err := os.Stat(mailersDir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("mailers path %s: %w", mailersDir, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("mailers path %s: not a directory", mailersDir)
	}

	var names []string
	err = filepath.WalkDir(mailersDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if strings.HasSuffix(path, ".rb") {
			rel, relErr := filepath.Rel(mailersDir, path)
			if relErr == nil {
				name := strings.TrimSuffix(filepath.ToSlash(rel), ".rb")
				name = strings.TrimSuffix(name, "_mailer")
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

// macroAllowlist holds the mailer macros whose entry line gets a color accent
// in Format. The Default and Layout sections render their own values (the
// macro keyword is not part of the entry), so this is intentionally empty for
// now -- kept as a hook parallel to controllers for any future macro-keyword
// entry.
var macroAllowlist = map[string]bool{
	"default": true,
	"layout":  true,
}

func isMacroToken(tok string) bool {
	return macroAllowlist[tok]
}

// styleEntry colors the leading macro keyword of a "  macro ..." entry line
// produced by Parse, leaving the rest of the line untouched. Lines whose
// first token is not a known macro (bare names like a mailer method) pass
// through unchanged.
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

	if len(s.Default) > 0 {
		sb.WriteString("\n")
		sb.WriteString(st.Bold("Default:") + "\n")
		for _, e := range s.Default {
			sb.WriteString(styleEntry(e, st) + "\n")
		}
	}
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
		{"Attachments", s.Attachments},
		{"Mailer Methods", s.Methods},
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
