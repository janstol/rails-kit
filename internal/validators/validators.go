// Package validators extracts a structural summary of a Rails validator
// file -- its parent class, included concerns, class-level constants, other
// class-level DSL calls (surfaced as macros), and methods -- without booting
// Rails.
//
// Three shapes exist in the wild, distinguished by the header line alone:
// an ActiveModel::EachValidator subclass overriding validate_each, an
// ActiveModel::Validator subclass overriding validate, and a plain class
// (or module) that includes ActiveModel::Validations and drives itself with
// validates. Every method is rendered as its full parameter signature, same
// as decorators/formers/presenters/helpers -- for a validator that
// signature is what tells an EachValidator apart from a Validator at a
// glance, not boilerplate to hide. Constants get their own section because
// that is where the actual rule usually lives: a format regexp or an
// allowed-value list. Only the validator's own file is parsed; behavior
// inherited from a superclass is not resolved. Summary reports ParentClass
// so callers know where to look next.
//
// Unlike most readers, validators are deliberately not wired into
// internal/related: validator files are named after the rule they enforce
// (phone_validator, email_format_validator), not after the model they run
// against, so there is no name-based link worth drawing.
package validators

import (
	"errors"

	"github.com/janstol/rails-kit/internal/astutil"
	"github.com/janstol/rails-kit/internal/reader"
	"github.com/janstol/rails-kit/internal/term"
)

// Summary holds the extracted validator structure.
type Summary struct {
	ClassName   string
	Kind        string // "class" or "module"
	ParentClass string
	RelPath     string
	Concerns    []string
	Constants   []string
	Macros      []string // catch-all for other class-level calls, e.g. validates
	Methods     []string
	ParseErrors []ParseDiagnostic
}

// ParseDiagnostic describes a recoverable Ruby syntax error reported by Prism.
type ParseDiagnostic = astutil.ParseDiagnostic

var errAmbiguousValidatorName = errors.New("ambiguous validator name")

// macroAllowlist holds the validator DSL keywords whose entry line gets a
// color accent in Format.
var macroAllowlist = map[string]bool{
	"validates":      true,
	"validates_each": true,
	"validate":       true,
	"attr_accessor":  true,
	"attr_reader":    true,
	"attr_writer":    true,
	"delegate":       true,
}

func isMacroToken(tok string) bool {
	return macroAllowlist[tok]
}

var kind = reader.Kind{
	Noun:         "validator",
	Plural:       "validators",
	Suffix:       "_validator",
	ErrAmbiguous: errAmbiguousValidatorName,
	IsMacro:      isMacroToken,
}

// IsAmbiguousError reports whether err indicates multiple validator matches.
func IsAmbiguousError(err error) bool {
	return errors.Is(err, errAmbiguousValidatorName)
}

// Resolve finds the validator file for the given name or path within
// railsRoot. The name may be a resource name ("phone"), the full file
// basename ("phone_validator"), a CamelCase class name ("PhoneValidator",
// "Admin::AccessValidator"), or a file path ending in .rb.
func Resolve(railsRoot, validatorsPath, input string) (string, error) {
	return kind.Resolve(railsRoot, validatorsPath, input)
}

// ListNames returns sorted snake_case validator names (relative to the
// validators directory, without .rb, with the "_validator" suffix stripped)
// for every validator file under railsRoot's validators path, including
// app/validators/concerns -- nothing owns that directory the way
// app/controllers/concerns is owned by the concerns command, so it stays
// reachable here. Returns nil, nil if the validators directory does not
// exist.
func ListNames(railsRoot, validatorsPath string) ([]string, error) {
	return kind.ListNames(railsRoot, validatorsPath)
}

// Format renders the summary as a human-readable string. st controls terminal
// color accents; the zero value renders identically to the uncolored output.
func Format(s *Summary, st term.Styler) string {
	return kind.Format(
		reader.ClassOrModuleHeader(s.Kind, s.ClassName, s.ParentClass, s.RelPath),
		[]reader.Section{
			{Label: "Constants", Entries: s.Constants},
			{Label: "Concerns", Entries: s.Concerns},
			{Label: "Macros", Entries: s.Macros},
			{Label: "Methods", Entries: s.Methods},
		},
		st,
	)
}
