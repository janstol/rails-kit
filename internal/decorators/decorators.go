// Package decorators extracts a structural summary of a Rails decorator file
// -- its parent class, included concerns, class-level constants, other
// class-level DSL calls (surfaced as macros), and methods -- without booting
// Rails.
//
// The reader targets the `draper` gem convention (`delegate_all` on an
// `ApplicationDecorator`/`Draper::Decorator` subclass) but never assumes it:
// a custom decorator implementation still resolves and reports a useful
// summary -- parent class, concerns, methods, and the class-level calls it
// does make -- just without the Draper-specific accent. Like helpers, each
// method renders as its full parameter signature rather than a bare name.
// Only the decorator's own file is parsed; behavior inherited from a
// superclass is not resolved. Summary reports ParentClass so callers know
// where to look next.
package decorators

import (
	"errors"

	"github.com/janstol/rails-kit/internal/astutil"
	"github.com/janstol/rails-kit/internal/reader"
	"github.com/janstol/rails-kit/internal/term"
)

// Summary holds the extracted decorator structure.
type Summary struct {
	ClassName   string
	Kind        string // "class" or "module"
	ParentClass string
	RelPath     string
	Concerns    []string
	Constants   []string
	Macros      []string // catch-all for other class-level calls, e.g. delegate_all
	Methods     []string
	ParseErrors []ParseDiagnostic
}

// ParseDiagnostic describes a recoverable Ruby syntax error reported by Prism.
type ParseDiagnostic = astutil.ParseDiagnostic

var errAmbiguousDecoratorName = errors.New("ambiguous decorator name")

// macroAllowlist holds the Draper DSL keywords whose entry line gets a color
// accent in Format.
var macroAllowlist = map[string]bool{
	"delegate_all":          true,
	"delegate":              true,
	"decorates":             true,
	"decorates_association": true,
	"decorates_finders":     true,
}

func isMacroToken(tok string) bool {
	return macroAllowlist[tok]
}

var kind = reader.Kind{
	Noun:         "decorator",
	Plural:       "decorators",
	Suffix:       "_decorator",
	ErrAmbiguous: errAmbiguousDecoratorName,
	IsMacro:      isMacroToken,
}

// IsAmbiguousError reports whether err indicates multiple decorator matches.
func IsAmbiguousError(err error) bool {
	return errors.Is(err, errAmbiguousDecoratorName)
}

// Resolve finds the decorator file for the given name or path within
// railsRoot. The name may be a resource name ("users"), the full file
// basename ("users_decorator"), a CamelCase class name ("UserDecorator",
// "Admin::ReportDecorator"), or a file path ending in .rb.
func Resolve(railsRoot, decoratorsPath, input string) (string, error) {
	return kind.Resolve(railsRoot, decoratorsPath, input)
}

// ListNames returns sorted snake_case decorator names (relative to the
// decorators directory, without .rb, with the "_decorator" suffix stripped)
// for every decorator file under railsRoot's decorators path, including
// app/decorators/concerns -- nothing owns that directory the way
// app/controllers/concerns is owned by the concerns command, so it stays
// reachable here. Returns nil, nil if the decorators directory does not
// exist.
func ListNames(railsRoot, decoratorsPath string) ([]string, error) {
	return kind.ListNames(railsRoot, decoratorsPath)
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
			{Label: "Macros", Entries: s.Macros},
			{Label: "Methods", Entries: s.Methods},
		},
		st,
	)
}
