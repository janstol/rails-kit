// Package presenters extracts a structural summary of a Rails presenter file
// -- its parent class, included concerns, class-level constants, attributes,
// other class-level DSL calls (surfaced as macros), and methods -- without
// booting Rails.
//
// Presenters carry more constants than any other reader's domain and lean
// heavily on attr_reader/attr_accessor/attr_writer to expose what they wrap,
// so those get their own Attributes section rather than being folded into
// the Macros catch-all -- the same reasoning as formers' Attributes section.
// delegate and any other class-level call fall through to Macros. Only the
// presenter's own file is parsed; behavior inherited from a superclass is
// not resolved. Summary reports ParentClass so callers know where to look
// next.
package presenters

import (
	"errors"

	"github.com/janstol/rails-kit/internal/astutil"
	"github.com/janstol/rails-kit/internal/reader"
	"github.com/janstol/rails-kit/internal/term"
)

// Summary holds the extracted presenter structure.
type Summary struct {
	ClassName   string
	Kind        string // "class" or "module"
	ParentClass string
	RelPath     string
	Concerns    []string
	Constants   []string
	Attributes  []string
	Macros      []string // catch-all for other class-level calls, e.g. delegate
	Methods     []string
	ParseErrors []ParseDiagnostic
}

// ParseDiagnostic describes a recoverable Ruby syntax error reported by Prism.
type ParseDiagnostic = astutil.ParseDiagnostic

var errAmbiguousPresenterName = errors.New("ambiguous presenter name")

// macroAllowlist holds the presenter DSL keywords whose entry line gets a
// color accent in Format, regardless of which section they land in.
var macroAllowlist = map[string]bool{
	"attr_reader":   true,
	"attr_accessor": true,
	"attr_writer":   true,
	"delegate":      true,
}

func isMacroToken(tok string) bool {
	return macroAllowlist[tok]
}

var kind = reader.Kind{
	Noun:         "presenter",
	Plural:       "presenters",
	Suffix:       "_presenter",
	ErrAmbiguous: errAmbiguousPresenterName,
	IsMacro:      isMacroToken,
}

// IsAmbiguousError reports whether err indicates multiple presenter matches.
func IsAmbiguousError(err error) bool {
	return errors.Is(err, errAmbiguousPresenterName)
}

// Resolve finds the presenter file for the given name or path within
// railsRoot. The name may be a resource name ("users"), the full file
// basename ("users_presenter"), a CamelCase class name ("UserPresenter",
// "Admin::ReportPresenter"), or a file path ending in .rb.
func Resolve(railsRoot, presentersPath, input string) (string, error) {
	return kind.Resolve(railsRoot, presentersPath, input)
}

// ListNames returns sorted snake_case presenter names (relative to the
// presenters directory, without .rb, with the "_presenter" suffix stripped)
// for every presenter file under railsRoot's presenters path, including
// app/presenters/concerns -- nothing owns that directory the way
// app/controllers/concerns is owned by the concerns command, so it stays
// reachable here. Returns nil, nil if the presenters directory does not
// exist.
func ListNames(railsRoot, presentersPath string) ([]string, error) {
	return kind.ListNames(railsRoot, presentersPath)
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
			{Label: "Macros", Entries: s.Macros},
			{Label: "Methods", Entries: s.Methods},
		},
		st,
	)
}
