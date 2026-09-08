// Package formers extracts a structural summary of a Rails form object --
// its parent class, included concerns, class-level constants, attributes,
// validations, other class-level DSL calls (surfaced as macros), and methods
// -- without booting Rails.
//
// The reader targets the common `ActiveModel::Model`-based form object
// convention, but never assumes it: a custom former implementation still
// resolves and reports a useful summary -- parent class, concerns, methods,
// and the class-level calls it does make -- just without any convention-
// specific accent. Formers are only lightly parameterized (most public
// methods take no arguments), but the dominant signal in a former is its
// validations and attributes, so each gets its own section rather than being
// folded into a catch-all. Only the former's own file is parsed; behavior
// inherited from a superclass is not resolved. Summary reports ParentClass so
// callers know where to look next.
//
// Filenames follow two conventions across real apps -- "_former" and
// "_form" -- plus a handful of bare-named files (mostly under
// formers/concerns/), so Resolve and ListNames try both suffixes.
package formers

import (
	"errors"

	"github.com/janstol/rails-kit/internal/astutil"
	"github.com/janstol/rails-kit/internal/reader"
	"github.com/janstol/rails-kit/internal/term"
)

// Summary holds the extracted former structure.
type Summary struct {
	ClassName   string
	Kind        string // "class" or "module"
	ParentClass string
	RelPath     string
	Concerns    []string
	Constants   []string
	Attributes  []string
	Validations []string
	Macros      []string // catch-all for other class-level calls, e.g. delegate
	Methods     []string
	ParseErrors []ParseDiagnostic
}

// ParseDiagnostic describes a recoverable Ruby syntax error reported by Prism.
type ParseDiagnostic = astutil.ParseDiagnostic

var errAmbiguousFormerName = errors.New("ambiguous former name")

// macroAllowlist holds the form-object DSL keywords whose entry line gets a
// color accent in Format, regardless of which section they land in.
var macroAllowlist = map[string]bool{
	"validates":      true,
	"validate":       true,
	"validates_each": true,
	"attr_accessor":  true,
	"attr_reader":    true,
	"attr_writer":    true,
	"delegate":       true,
	"with_options":   true,
}

func isMacroToken(tok string) bool {
	return macroAllowlist[tok]
}

var kind = reader.Kind{
	Noun:         "former",
	Plural:       "formers",
	Suffix:       "_former",
	AltSuffixes:  []string{"_form"},
	ErrAmbiguous: errAmbiguousFormerName,
	IsMacro:      isMacroToken,
}

// IsAmbiguousError reports whether err indicates multiple former matches.
func IsAmbiguousError(err error) bool {
	return errors.Is(err, errAmbiguousFormerName)
}

// Resolve finds the former file for the given name or path within railsRoot.
// The name may be a resource name ("users"), the full file basename
// ("users_former", "session_form"), a CamelCase class name ("UserFormer",
// "Admin::ReportFormer"), or a file path ending in .rb.
func Resolve(railsRoot, formersPath, input string) (string, error) {
	return kind.Resolve(railsRoot, formersPath, input)
}

// ListNames returns sorted snake_case former names (relative to the formers
// directory, without .rb, with the "_former"/"_form" suffix stripped) for
// every former file under railsRoot's formers path, including
// formers/concerns -- nothing owns that directory the way
// app/controllers/concerns is owned by the concerns command, so it stays
// reachable here. Returns nil, nil if the formers directory does not exist.
func ListNames(railsRoot, formersPath string) ([]string, error) {
	return kind.ListNames(railsRoot, formersPath)
}

// Format renders the summary as a human-readable string. st controls terminal
// color accents; the zero value renders identically to the uncolored output.
func Format(s *Summary, st term.Styler) string {
	h := reader.Header{Title: s.ClassName, Parent: s.ParentClass, RelPath: s.RelPath}
	if s.Kind == "module" {
		h = reader.Header{Title: "module " + s.ClassName, RelPath: s.RelPath}
	}
	return kind.Format(
		h,
		[]reader.Section{
			{Label: "Constants", Entries: s.Constants},
			{Label: "Concerns", Entries: s.Concerns},
			{Label: "Attributes", Entries: s.Attributes},
			{Label: "Validations", Entries: s.Validations},
			{Label: "Macros", Entries: s.Macros},
			{Label: "Methods", Entries: s.Methods},
		},
		st,
	)
}
