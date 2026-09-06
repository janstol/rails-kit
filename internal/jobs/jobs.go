// Package jobs extracts a structural summary of a Rails ActiveJob file -- its
// queue, retry_on/discard_on handlers, included concerns, and public methods
// (notably `perform`) -- without booting Rails.
//
// Only the job's own file is parsed; queue, retry, or discard behavior
// inherited from ApplicationJob (or any other superclass) is not resolved.
// Summary reports ParentClass so callers know where to look next.
package jobs

import (
	"errors"

	"github.com/janstol/rails-kit/internal/astutil"
	"github.com/janstol/rails-kit/internal/reader"
	"github.com/janstol/rails-kit/internal/term"
)

// Summary holds the extracted job structure.
type Summary struct {
	ClassName   string
	ParentClass string
	RelPath     string
	Concerns    []string
	Queue       string
	RetryOn     []string
	DiscardOn   []string
	Methods     []string
	ParseErrors []ParseDiagnostic
}

// ParseDiagnostic describes a recoverable Ruby syntax error reported by Prism.
type ParseDiagnostic = astutil.ParseDiagnostic

var errAmbiguousJobName = errors.New("ambiguous job name")

// macroAllowlist holds the job macros whose entry line gets a color accent in
// Format. queue_as renders its own value (the macro keyword is not part of the
// entry when it is a bare symbol/string), but retry_on and discard_on entries
// carry their keyword, so they are accent targets.
var macroAllowlist = map[string]bool{
	"queue_as":   true,
	"retry_on":   true,
	"discard_on": true,
}

var kind = reader.Kind{
	Noun:         "job",
	Plural:       "jobs",
	Suffix:       "_job",
	ErrAmbiguous: errAmbiguousJobName,
	IsMacro:      func(tok string) bool { return macroAllowlist[tok] },
}

// IsAmbiguousError reports whether err indicates multiple job matches.
func IsAmbiguousError(err error) bool {
	return errors.Is(err, errAmbiguousJobName)
}

// Resolve finds the job file for the given name or path within railsRoot.
// The name may be a short resource name ("sync_user"), the full file basename
// ("sync_user_job"), a CamelCase class name ("SyncUserJob",
// "Admin::ExportJob"), or a file path ending in .rb.
func Resolve(railsRoot, jobsPath, input string) (string, error) {
	return kind.Resolve(railsRoot, jobsPath, input)
}

// ListNames returns sorted snake_case job names (relative to the jobs
// directory, without .rb, with the "_job" suffix stripped) for every job file
// under railsRoot's jobs path. Returns nil, nil if the jobs directory does not
// exist.
func ListNames(railsRoot, jobsPath string) ([]string, error) {
	return kind.ListNames(railsRoot, jobsPath)
}

// Format renders the summary as a human-readable string. st controls terminal
// color accents; the zero value renders identically to the uncolored output.
func Format(s *Summary, st term.Styler) string {
	return kind.Format(
		reader.Header{Title: s.ClassName, Parent: s.ParentClass, RelPath: s.RelPath},
		[]reader.Section{
			{Label: "Queue", Value: s.Queue},
			{Label: "Concerns", Entries: s.Concerns},
			{Label: "Retry On", Entries: s.RetryOn},
			{Label: "Discard On", Entries: s.DiscardOn},
			{Label: "Job Methods", Entries: s.Methods},
		},
		st,
	)
}
