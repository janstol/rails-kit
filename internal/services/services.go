// Package services extracts a structural summary of a Rails service file -- its
// parent class (if any), included concerns, class-level constants, and methods
// (both public instance methods and singleton `def self.x` class methods) --
// without booting Rails.
//
// Services have no universal naming convention (no `_controller`/`_job` suffix)
// and no conventional macros, so this reader is thinner than the others: it
// strips no suffix from file names and collects no macro-specific fields. Only
// the service's own file is parsed; behavior inherited from a superclass is
// not resolved. Summary reports ParentClass so callers know where to look next.
package services

import (
	"errors"
	"strings"

	"github.com/janstol/rails-kit/internal/reader"
	"github.com/janstol/rails-kit/internal/term"
)

// Summary holds the extracted service structure.
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
type ParseDiagnostic struct {
	Line    int
	Message string
}

var errAmbiguousServiceName = errors.New("ambiguous service name")

var kind = reader.Kind{
	Noun:         "service",
	Plural:       "services",
	Suffix:       "",
	ErrAmbiguous: errAmbiguousServiceName,
	IsMacro:      nil,
}

// IsAmbiguousError reports whether err indicates multiple service matches.
func IsAmbiguousError(err error) bool {
	return errors.Is(err, errAmbiguousServiceName)
}

// Resolve finds the service file for the given name or path within railsRoot.
// The name may be a resource name ("user_export_service"), a CamelCase class
// name ("UserExportService", "Admin::BillingService"), or a file path ending
// in .rb. Services have no universal suffix, so the name is matched as-is
// (no `_service` is appended).
func Resolve(railsRoot, servicesPath, input string) (string, error) {
	return kind.Resolve(railsRoot, servicesPath, input)
}

// ListNames returns sorted snake_case service names (relative to the services
// directory, without .rb) for every service file under railsRoot's services
// path. No suffix is stripped -- services have no universal naming convention.
// Returns nil, nil if the services directory does not exist.
func ListNames(railsRoot, servicesPath string) ([]string, error) {
	return kind.ListNames(railsRoot, servicesPath)
}

// Format renders the summary as a human-readable string. st controls terminal
// color accents; the zero value renders identically to the uncolored output.
func Format(s *Summary, st term.Styler) string {
	var sb strings.Builder
	if s.Kind == "module" {
		sb.WriteString(st.Bold("module " + s.ClassName))
	} else {
		sb.WriteString(st.Bold(s.ClassName))
		if s.ParentClass != "" {
			sb.WriteString(" < " + st.Cyan(s.ParentClass))
		}
	}
	sb.WriteString(" " + st.Dim("("+s.RelPath+")") + "\n")
	sb.WriteString(st.Dim(strings.Repeat("=", 40)) + "\n")

	sections := []struct {
		label   string
		entries []string
	}{
		{"Constants", s.Constants},
		{"Concerns", s.Concerns},
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
