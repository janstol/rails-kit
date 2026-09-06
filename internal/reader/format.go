package reader

import (
	"strings"

	"github.com/janstol/rails-kit/internal/term"
)

// Header describes the title line shared by every detail reader.
type Header struct {
	Title   string // "User" for a class, "module Foo" for services' module form
	Parent  string // rendered as " < <Parent>" in cyan; empty renders nothing
	RelPath string
}

// Section is one rendered block. Exactly one of Value/Entries is set; a
// Section with neither renders nothing. Value is rendered verbatim (indented,
// unstyled); Entries each go through Kind.StyleEntry. If both are set, Value
// wins.
type Section struct {
	Label   string
	Value   string
	Entries []string
}

// Format renders h and sections as the human-readable summary shared by every
// detail reader: a bold title (with an optional " < Parent" in cyan) followed
// by a dimmed "(RelPath)" and a dimmed rule, then each non-empty section in
// order, then a trailing newline. st controls terminal color accents; the
// zero value renders identically to the uncolored output.
func (k Kind) Format(h Header, sections []Section, st term.Styler) string {
	var sb strings.Builder
	sb.WriteString(st.Bold(h.Title))
	if h.Parent != "" {
		sb.WriteString(" < " + st.Cyan(h.Parent))
	}
	sb.WriteString(" " + st.Dim("("+h.RelPath+")") + "\n")
	sb.WriteString(st.Dim(strings.Repeat("=", 40)) + "\n")

	for _, sec := range sections {
		if sec.Value != "" {
			sb.WriteString("\n")
			sb.WriteString(st.Bold(sec.Label+":") + "\n")
			sb.WriteString("  " + sec.Value + "\n")
			continue
		}
		if len(sec.Entries) == 0 {
			continue
		}
		sb.WriteString("\n")
		sb.WriteString(st.Bold(sec.Label+":") + "\n")
		for _, e := range sec.Entries {
			sb.WriteString(k.StyleEntry(e, st) + "\n")
		}
	}
	sb.WriteString("\n")
	return sb.String()
}
