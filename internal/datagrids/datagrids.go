// Package datagrids extracts a structural summary of a Rails datagrid file --
// its parent class, included concerns, decorator, scope, filters, columns,
// other class-level DSL calls, and methods -- without booting Rails.
//
// The reader targets the `datagrid` gem DSL (`filter`/`column`/`scope`/`decorate`
// on a `BaseDatagrid` subclass, files named `*_datagrid.rb`) but never assumes
// it: a custom grid implementation or a different grid library in
// `app/datagrids/` still resolves and reports a useful services-shaped summary
// (parent class, concerns, methods, and the class-level calls it does make,
// surfaced as macros) -- just without the datagrid-gem-specific structure. Only
// the datagrid's own file is parsed; behavior inherited from a superclass is
// not resolved. Summary reports ParentClass so callers know where to look next.
package datagrids

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

// Summary holds the extracted datagrid structure.
type Summary struct {
	ClassName   string
	ParentClass string // omitempty: absent for custom-impl files with no superclass
	RelPath     string
	Concerns    []string
	Decorate    string // decorator class from `decorate { X }`, or "(block)"
	Scope       string // "(block)" when a `scope do…end` block is present
	Filters     []string
	Columns     []string
	Macros      []string // catch-all for other class-level calls
	Methods     []string // public instance + singleton `def self.x`
	ParseErrors []ParseDiagnostic
}

// ParseDiagnostic describes a recoverable Ruby syntax error reported by Prism.
type ParseDiagnostic struct {
	Line    int
	Message string
}

var errAmbiguousDatagridName = errors.New("ambiguous datagrid name")

// IsAmbiguousError reports whether err indicates multiple datagrid matches.
func IsAmbiguousError(err error) bool {
	return errors.Is(err, errAmbiguousDatagridName)
}

// Resolve finds the datagrid file for the given name or path within railsRoot.
// The name may be a short resource name ("example"), the full file basename
// ("example_datagrid"), a CamelCase class name ("ExampleDatagrid",
// "Admin::ReportDatagrid"), or a file path ending in .rb.
//
// The conventional `datagrid`-gem file is `*_datagrid.rb`; Resolve tries that
// first and falls back to the name exactly as given, so a non-suffixed custom
// grid (`custom_grid.rb`) still resolves.
func Resolve(railsRoot, datagridsPath, input string) (string, error) {
	datagridsDir := config.ResolvePath(railsRoot, datagridsPath)

	if strings.HasSuffix(input, ".rb") {
		path := input
		if !filepath.IsAbs(path) {
			abs, err := filepath.Abs(path)
			if err == nil {
				path = abs
			}
		}
		if _, err := os.Stat(path); err == nil {
			rel, err := filepath.Rel(datagridsDir, path)
			if err != nil || strings.HasPrefix(rel, "..") {
				return "", fmt.Errorf("datagrid file is outside datagrids directory: %s", path)
			}
			return path, nil
		}
		cleanInput := input
		if !filepath.IsAbs(datagridsPath) {
			prefix := filepath.ToSlash(filepath.Clean(datagridsPath)) + "/"
			cleanInput = strings.TrimPrefix(filepath.ToSlash(cleanInput), prefix)
		}
		path3 := filepath.Join(datagridsDir, cleanInput)
		if _, err := os.Stat(path3); err == nil {
			return path3, nil
		}
		path2 := filepath.Join(railsRoot, input)
		if _, err := os.Stat(path2); err == nil {
			rel, err := filepath.Rel(datagridsDir, path2)
			if err != nil || strings.HasPrefix(rel, "..") {
				return "", fmt.Errorf("datagrid file is outside datagrids directory: %s", path2)
			}
			return path2, nil
		}
		return "", fmt.Errorf("datagrid file not found: %s", input)
	}

	name := astutil.Underscore(input)
	normalizedName := astutil.NormalizeLookupName(name)
	info, err := os.Stat(datagridsDir)
	if err != nil {
		return "", fmt.Errorf("datagrids path %s: %w", datagridsDir, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("datagrids path %s: not a directory", datagridsDir)
	}

	// Prefer the conventional "*_datagrid.rb" file. Fall back to the name
	// exactly as given -- a custom grid implementation that doesn't follow the
	// `_datagrid` suffix convention still resolves, and ListNames surfaces
	// those names unmodified.
	suffixed := normalizedName
	if !strings.HasSuffix(suffixed, "_datagrid") {
		suffixed += "_datagrid"
	}
	candidates := []string{suffixed + ".rb"}
	if suffixed != normalizedName {
		candidates = append(candidates, normalizedName+".rb")
	}

	for _, target := range candidates {
		found, matches, err := findDatagridFile(datagridsDir, target)
		if err != nil {
			return "", err
		}
		if found != "" {
			return found, nil
		}
		if len(matches) > 1 {
			var relNames []string
			for _, m := range matches {
				if r, e := filepath.Rel(datagridsDir, m); e == nil {
					relNames = append(relNames, filepath.ToSlash(r))
				} else {
					relNames = append(relNames, m)
				}
			}
			return "", fmt.Errorf("%w '%s', matches: %s", errAmbiguousDatagridName, input, strings.Join(relNames, ", "))
		}
		if len(matches) == 1 {
			return matches[0], nil
		}
	}
	return "", fmt.Errorf("datagrid file not found for '%s'", input)
}

// findDatagridFile looks for target (a datagrids-dir-relative path) under
// datagridsDir. If target has no path separators, it also matches by basename
// anywhere in the tree, returning every match so the caller can detect
// ambiguity.
func findDatagridFile(datagridsDir, target string) (found string, matches []string, err error) {
	basenameOnly := filepath.Base(target) == target
	targetBase := filepath.Base(target)
	err = filepath.WalkDir(datagridsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		rel, relErr := filepath.Rel(datagridsDir, path)
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
		return "", nil, fmt.Errorf("walking datagrids path %s: %w", datagridsDir, err)
	}
	return found, matches, nil
}

// ListNames returns sorted snake_case datagrid names (relative to the
// datagrids directory, without .rb, with the "_datagrid" suffix stripped) for
// every datagrid file under railsRoot's datagrids path. A non-suffixed file
// (a custom grid) keeps its name verbatim -- TrimSuffix is a no-op for it.
// Returns nil, nil if the datagrids directory does not exist.
func ListNames(railsRoot, datagridsPath string) ([]string, error) {
	datagridsDir := config.ResolvePath(railsRoot, datagridsPath)

	info, err := os.Stat(datagridsDir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("datagrids path %s: %w", datagridsDir, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("datagrids path %s: not a directory", datagridsDir)
	}

	var names []string
	err = filepath.WalkDir(datagridsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if strings.HasSuffix(path, ".rb") {
			rel, relErr := filepath.Rel(datagridsDir, path)
			if relErr == nil {
				name := strings.TrimSuffix(filepath.ToSlash(rel), ".rb")
				name = strings.TrimSuffix(name, "_datagrid")
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

// macroAllowlist holds the datagrid DSL keywords whose entry line gets a color
// accent in Format. The catch-all Macros section also surfaces `filter_*`/
// `column_*` helpers (e.g. `filter_per_page`, `column_actions`, `column_id`),
// so those prefixes are accented too.
var macroAllowlist = map[string]bool{
	"filter":   true,
	"column":   true,
	"scope":    true,
	"decorate": true,
}

func isMacroToken(tok string) bool {
	if macroAllowlist[tok] {
		return true
	}
	return strings.HasPrefix(tok, "filter_") || strings.HasPrefix(tok, "column_")
}

// styleEntry colors the leading DSL keyword of a "  keyword ..." entry line
// produced by Parse, leaving the rest of the line untouched. Lines whose first
// token is not a known datagrid macro (a bare method or concern name) pass
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

// Format renders the summary as a human-readable string. st controls terminal
// color accents; the zero value renders identically to the uncolored output.
func Format(s *Summary, st term.Styler) string {
	var sb strings.Builder
	sb.WriteString(st.Bold(s.ClassName))
	if s.ParentClass != "" {
		sb.WriteString(" < " + st.Cyan(s.ParentClass))
	}
	sb.WriteString(" " + st.Dim("("+s.RelPath+")") + "\n")
	sb.WriteString(st.Dim(strings.Repeat("=", 40)) + "\n")

	if s.Decorate != "" {
		sb.WriteString("\n")
		sb.WriteString(st.Bold("Decorate:") + "\n")
		sb.WriteString("  " + s.Decorate + "\n")
	}
	if s.Scope != "" {
		sb.WriteString("\n")
		sb.WriteString(st.Bold("Scope:") + "\n")
		sb.WriteString("  " + s.Scope + "\n")
	}

	sections := []struct {
		label   string
		entries []string
	}{
		{"Concerns", s.Concerns},
		{"Filters", s.Filters},
		{"Columns", s.Columns},
		{"Macros", s.Macros},
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
