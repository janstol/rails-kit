// Package helpers extracts a structural summary of a Rails view helper file --
// its included concerns, class-level constants, and methods -- without
// booting Rails.
//
// A helper is an API surface consumed from views, so unlike the other
// readers, Methods here renders each method's parameter signature rather than
// just its name. Only the helper's own file is parsed.
package helpers

import (
	"errors"

	"github.com/janstol/rails-kit/internal/astutil"
	"github.com/janstol/rails-kit/internal/reader"
	"github.com/janstol/rails-kit/internal/term"
)

// Summary holds the extracted helper structure.
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
type ParseDiagnostic = astutil.ParseDiagnostic

var errAmbiguousHelperName = errors.New("ambiguous helper name")

var kind = reader.Kind{
	Noun:         "helper",
	Plural:       "helpers",
	Suffix:       "_helper",
	ErrAmbiguous: errAmbiguousHelperName,
	IsMacro:      nil,
}

// IsAmbiguousError reports whether err indicates multiple helper matches.
func IsAmbiguousError(err error) bool {
	return errors.Is(err, errAmbiguousHelperName)
}

// Resolve finds the helper file for the given name or path within railsRoot.
// The name may be a resource name ("users"), the full file basename
// ("users_helper"), a CamelCase module name ("UsersHelper",
// "Admin::ReportsHelper"), or a file path ending in .rb.
func Resolve(railsRoot, helpersPath, input string) (string, error) {
	return kind.Resolve(railsRoot, helpersPath, input)
}

// ListNames returns sorted snake_case helper names (relative to the helpers
// directory, without .rb, with the _helper suffix stripped) for every helper
// file under railsRoot's helpers path. Returns nil, nil if the helpers
// directory does not exist.
func ListNames(railsRoot, helpersPath string) ([]string, error) {
	return kind.ListNames(railsRoot, helpersPath)
}

// Format renders the summary as a human-readable string. st controls terminal
// color accents; the zero value renders identically to the uncolored output.
func Format(s *Summary, st term.Styler) string {
	return kind.Format(
		reader.ClassOrModuleHeader(s.Kind, s.ClassName, s.ParentClass, s.RelPath),
		[]reader.Section{
			{Label: "Constants", Entries: s.Constants},
			{Label: "Concerns", Entries: s.Concerns},
			{Label: "Methods", Entries: s.Methods},
		},
		st,
	)
}
