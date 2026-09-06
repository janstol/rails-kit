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

	"github.com/janstol/rails-kit/internal/astutil"
	"github.com/janstol/rails-kit/internal/reader"
	"github.com/janstol/rails-kit/internal/term"
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
type ParseDiagnostic = astutil.ParseDiagnostic

var errAmbiguousMailerName = errors.New("ambiguous mailer name")

// macroAllowlist holds the mailer macros whose entry line gets a color accent
// in Format. The Default and Layout sections render their own values (the
// macro keyword is not part of the entry), so this is intentionally empty for
// now -- kept as a hook parallel to controllers for any future macro-keyword
// entry.
var macroAllowlist = map[string]bool{
	"default": true,
	"layout":  true,
}

var kind = reader.Kind{
	Noun:         "mailer",
	Plural:       "mailers",
	Suffix:       "_mailer",
	ErrAmbiguous: errAmbiguousMailerName,
	IsMacro:      func(tok string) bool { return macroAllowlist[tok] },
}

// IsAmbiguousError reports whether err indicates multiple mailer matches.
func IsAmbiguousError(err error) bool {
	return errors.Is(err, errAmbiguousMailerName)
}

// Resolve finds the mailer file for the given name or path within railsRoot.
// The name may be a short resource name ("users"), the full file basename
// ("user_mailer"), a CamelCase class name ("UserMailer",
// "Admin::NotificationMailer"), or a file path ending in .rb.
func Resolve(railsRoot, mailersPath, input string) (string, error) {
	return kind.Resolve(railsRoot, mailersPath, input)
}

// ListNames returns sorted snake_case mailer names (relative to the mailers
// directory, without .rb, with the "_mailer" suffix stripped) for every mailer
// file under railsRoot's mailers path. Returns nil, nil if the mailers
// directory does not exist.
func ListNames(railsRoot, mailersPath string) ([]string, error) {
	return kind.ListNames(railsRoot, mailersPath)
}

// Format renders the summary as a human-readable string. st controls
// terminal color accents; the zero value renders identically to the
// uncolored output.
func Format(s *Summary, st term.Styler) string {
	return kind.Format(
		reader.Header{Title: s.ClassName, Parent: s.ParentClass, RelPath: s.RelPath},
		[]reader.Section{
			{Label: "Default", Entries: s.Default},
			{Label: "Layout", Value: s.Layout},
			{Label: "Concerns", Entries: s.Concerns},
			{Label: "Attachments", Entries: s.Attachments},
			{Label: "Mailer Methods", Entries: s.Methods},
		},
		st,
	)
}
