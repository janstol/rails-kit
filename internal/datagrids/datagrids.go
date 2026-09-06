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
	"strings"

	"github.com/janstol/rails-kit/internal/astutil"
	"github.com/janstol/rails-kit/internal/reader"
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
type ParseDiagnostic = astutil.ParseDiagnostic

var errAmbiguousDatagridName = errors.New("ambiguous datagrid name")

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

var kind = reader.Kind{
	Noun:         "datagrid",
	Plural:       "datagrids",
	Suffix:       "_datagrid",
	ErrAmbiguous: errAmbiguousDatagridName,
	IsMacro:      isMacroToken,
}

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
	return kind.Resolve(railsRoot, datagridsPath, input)
}

// ListNames returns sorted snake_case datagrid names (relative to the
// datagrids directory, without .rb, with the "_datagrid" suffix stripped) for
// every datagrid file under railsRoot's datagrids path. A non-suffixed file
// (a custom grid) keeps its name verbatim -- TrimSuffix is a no-op for it.
// Returns nil, nil if the datagrids directory does not exist.
func ListNames(railsRoot, datagridsPath string) ([]string, error) {
	return kind.ListNames(railsRoot, datagridsPath)
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
			sb.WriteString(kind.StyleEntry(e, st) + "\n")
		}
	}
	sb.WriteString("\n")
	return sb.String()
}
