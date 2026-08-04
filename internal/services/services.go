// Package services extracts a structural summary of a Rails service file -- its
// parent class (if any), included concerns, class-level constants, and methods
// (both public instance methods and singleton `def self.x` class methods) --
// without booting Rails.
//
// Services have no universal naming convention (no `_controller`/`_job` suffix)
// and no conventional macros, so this reader is thinner than the others: it
// strips no suffix from file names and collects no macro-specific fields. Only
// the service's own file is parsed; behavior inherited from a superclass is
// not resolved. Summary reports ParentClass so callers know where to look next.
package services

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/janstol/rails-kit/internal/astutil"
	"github.com/janstol/rails-kit/internal/config"
	"github.com/janstol/rails-kit/internal/term"
)

// Summary holds the extracted service structure.
type Summary struct {
	ClassName   string
	Kind        string // "class" or "module"
	ParentClass string
	RelPath     string
	Concerns    []string
	Constants   []string
	Methods     []string
	ParseErrors []ParseDiagnostic
}

// ParseDiagnostic describes a recoverable Ruby syntax error reported by Prism.
type ParseDiagnostic struct {
	Line    int
	Message string
}

var errAmbiguousServiceName = errors.New("ambiguous service name")

// IsAmbiguousError reports whether err indicates multiple service matches.
func IsAmbiguousError(err error) bool {
	return errors.Is(err, errAmbiguousServiceName)
}

// Resolve finds the service file for the given name or path within railsRoot.
// The name may be a resource name ("user_export_service"), a CamelCase class
// name ("UserExportService", "Admin::BillingService"), or a file path ending
// in .rb. Services have no universal suffix, so the name is matched as-is
// (no `_service` is appended).
func Resolve(railsRoot, servicesPath, input string) (string, error) {
	servicesDir := config.ResolvePath(railsRoot, servicesPath)

	if strings.HasSuffix(input, ".rb") {
		path := input
		if !filepath.IsAbs(path) {
			abs, err := filepath.Abs(path)
			if err == nil {
				path = abs
			}
		}
		if _, err := os.Stat(path); err == nil {
			rel, err := filepath.Rel(servicesDir, path)
			if err != nil || strings.HasPrefix(rel, "..") {
				return "", fmt.Errorf("service file is outside services directory: %s", path)
			}
			return path, nil
		}
		cleanInput := input
		if !filepath.IsAbs(servicesPath) {
			prefix := filepath.ToSlash(filepath.Clean(servicesPath)) + "/"
			cleanInput = strings.TrimPrefix(filepath.ToSlash(cleanInput), prefix)
		}
		path3 := filepath.Join(servicesDir, cleanInput)
		if _, err := os.Stat(path3); err == nil {
			return path3, nil
		}
		path2 := filepath.Join(railsRoot, input)
		if _, err := os.Stat(path2); err == nil {
			rel, err := filepath.Rel(servicesDir, path2)
			if err != nil || strings.HasPrefix(rel, "..") {
				return "", fmt.Errorf("service file is outside services directory: %s", path2)
			}
			return path2, nil
		}
		return "", fmt.Errorf("service file not found: %s", input)
	}

	name := astutil.Underscore(input)
	normalizedName := astutil.NormalizeLookupName(name)
	info, err := os.Stat(servicesDir)
	if err != nil {
		return "", fmt.Errorf("services path %s: %w", servicesPath, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("services path %s: not a directory", servicesPath)
	}

	// Services have no universal suffix -- match the name exactly as given.
	candidates := []string{normalizedName + ".rb"}

	for _, target := range candidates {
		found, matches, err := findServiceFile(servicesDir, target)
		if err != nil {
			return "", err
		}
		if found != "" {
			return found, nil
		}
		if len(matches) > 1 {
			var relNames []string
			for _, m := range matches {
				if r, e := filepath.Rel(servicesDir, m); e == nil {
					relNames = append(relNames, filepath.ToSlash(r))
				} else {
					relNames = append(relNames, m)
				}
			}
			return "", fmt.Errorf("%w '%s', matches: %s", errAmbiguousServiceName, input, strings.Join(relNames, ", "))
		}
		if len(matches) == 1 {
			return matches[0], nil
		}
	}
	return "", fmt.Errorf("service file not found for '%s'", input)
}

// findServiceFile looks for target (a services-dir-relative path) under
// servicesDir. If target has no path separators, it also matches by basename
// anywhere in the tree, returning every match so the caller can detect
// ambiguity.
func findServiceFile(servicesDir, target string) (found string, matches []string, err error) {
	basenameOnly := filepath.Base(target) == target
	targetBase := filepath.Base(target)
	err = filepath.WalkDir(servicesDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		rel, relErr := filepath.Rel(servicesDir, path)
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
		return "", nil, fmt.Errorf("walking services path %s: %w", servicesDir, err)
	}
	return found, matches, nil
}

// ListNames returns sorted snake_case service names (relative to the services
// directory, without .rb) for every service file under railsRoot's services
// path. No suffix is stripped -- services have no universal naming convention.
// Returns nil, nil if the services directory does not exist.
func ListNames(railsRoot, servicesPath string) ([]string, error) {
	servicesDir := config.ResolvePath(railsRoot, servicesPath)

	info, err := os.Stat(servicesDir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("services path %s: %w", servicesDir, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("services path %s: not a directory", servicesDir)
	}

	var names []string
	err = filepath.WalkDir(servicesDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if strings.HasSuffix(path, ".rb") {
			rel, relErr := filepath.Rel(servicesDir, path)
			if relErr == nil {
				name := strings.TrimSuffix(filepath.ToSlash(rel), ".rb")
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

// macroAllowlist holds the macros whose entry line gets a color accent in
// Format. Services have no conventional macros, so this is empty and styleEntry
// passes every entry through unchanged. Kept as a hook parallel to the other
// readers for any future macro-keyword entry.
var macroAllowlist = map[string]bool{}

func isMacroToken(tok string) bool {
	return macroAllowlist[tok]
}

// styleEntry colors the leading macro keyword of a "  macro ..." entry line
// produced by Parse, leaving the rest of the line untouched. Services have no
// known macros, so every line passes through unchanged.
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

// Format renders the summary as a human-readable string. st controls terminal
// color accents; the zero value renders identically to the uncolored output.
func Format(s *Summary, st term.Styler) string {
	var sb strings.Builder
	if s.Kind == "module" {
		sb.WriteString(st.Bold("module " + s.ClassName))
	} else {
		sb.WriteString(st.Bold(s.ClassName))
		if s.ParentClass != "" {
			sb.WriteString(" < " + st.Cyan(s.ParentClass))
		}
	}
	sb.WriteString(" " + st.Dim("("+s.RelPath+")") + "\n")
	sb.WriteString(st.Dim(strings.Repeat("=", 40)) + "\n")

	sections := []struct {
		label   string
		entries []string
	}{
		{"Constants", s.Constants},
		{"Concerns", s.Concerns},
		{"Methods", s.Methods},
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
