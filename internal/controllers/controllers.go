// Package controllers extracts a structural summary of a Rails controller
// file -- filters, rescue_from handlers, helper methods, layout, respond_to
// formats, strong params, and action methods -- without booting Rails.
//
// Only the controller's own file is parsed; filters inherited from
// ApplicationController (or any other superclass) are not resolved. Summary
// reports ParentClass so callers know where to look next.
package controllers

import (
	"errors"
	"strings"

	"github.com/janstol/rails-kit/internal/astutil"
	"github.com/janstol/rails-kit/internal/config"
	"github.com/janstol/rails-kit/internal/reader"
	"github.com/janstol/rails-kit/internal/term"
)

// Summary holds the extracted controller structure.
type Summary struct {
	ClassName     string
	ParentClass   string
	RelPath       string
	Concerns      []string
	Filters       []string
	RescueFrom    []string
	HelperMethods []string
	Layout        string
	RespondTo     []string
	StrongParams  []string
	Actions       []string
	ParseErrors   []ParseDiagnostic
}

// ParseDiagnostic describes a recoverable Ruby syntax error reported by Prism.
type ParseDiagnostic = astutil.ParseDiagnostic

var errAmbiguousControllerName = errors.New("ambiguous controller name")

// macroAllowlist holds the controller macros whose entry line gets a color
// accent in Format. Bare-name sections (Concerns, HelperMethods, RespondTo,
// Actions) are intentionally excluded -- their section label already carries
// the accent.
var macroAllowlist = map[string]bool{
	"rescue_from": true,
}

func isMacroToken(tok string) bool {
	if macroAllowlist[tok] {
		return true
	}
	return strings.HasPrefix(tok, "before_") || strings.HasPrefix(tok, "after_") || strings.HasPrefix(tok, "around_") ||
		strings.HasPrefix(tok, "skip_before_") || strings.HasPrefix(tok, "skip_after_") || strings.HasPrefix(tok, "skip_around_")
}

var kind = reader.Kind{
	Noun:         "controller",
	Plural:       "controllers",
	Suffix:       "_controller",
	ErrAmbiguous: errAmbiguousControllerName,
	IsMacro:      isMacroToken,
}

// IsAmbiguousError reports whether err indicates multiple controller matches.
func IsAmbiguousError(err error) bool {
	return errors.Is(err, errAmbiguousControllerName)
}

// Resolve finds the controller file for the given name or path within railsRoot.
// The name may be a short resource name ("users"), the full file basename
// ("users_controller"), a CamelCase class name ("UsersController",
// "Admin::ReportsController"), or a file path ending in .rb.
func Resolve(railsRoot, controllersPath, input string) (string, error) {
	return kind.Resolve(railsRoot, controllersPath, input)
}

// ListNames returns sorted snake_case controller names (relative to the
// controllers directory, without .rb, with the "_controller" suffix
// stripped) for every controller file under railsRoot's controllers path.
// Files under controllerConcernsPath are excluded -- they hold concern
// modules, not controllers. Returns nil, nil if the controllers directory
// does not exist.
func ListNames(railsRoot, controllersPath, controllerConcernsPath string) ([]string, error) {
	concernsDir := config.ResolvePath(railsRoot, controllerConcernsPath)
	return kind.ListNames(railsRoot, controllersPath, concernsDir)
}

// Format renders the summary as a human-readable string. st controls
// terminal color accents; the zero value renders identically to the
// uncolored output.
func Format(s *Summary, st term.Styler) string {
	return kind.Format(
		reader.Header{Title: s.ClassName, Parent: s.ParentClass, RelPath: s.RelPath},
		[]reader.Section{
			{Label: "Layout", Value: s.Layout},
			{Label: "Concerns", Entries: s.Concerns},
			{Label: "Filters", Entries: s.Filters},
			{Label: "Rescue From", Entries: s.RescueFrom},
			{Label: "Helper Methods", Entries: s.HelperMethods},
			{Label: "Respond To", Entries: s.RespondTo},
			{Label: "Strong Params", Entries: s.StrongParams},
			{Label: "Actions", Entries: s.Actions},
		},
		st,
	)
}
