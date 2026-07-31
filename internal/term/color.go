package term

import (
	"fmt"
	"os"
)

// Mode selects when a Styler emits ANSI escapes.
type Mode int

const (
	// ModeAuto enables color only when the target is an interactive terminal.
	ModeAuto Mode = iota
	// ModeAlways enables color unconditionally, except when NO_COLOR is set.
	ModeAlways
	// ModeNever disables color unconditionally.
	ModeNever
)

// ParseMode parses a --color flag value ("auto", "always", "never").
func ParseMode(s string) (Mode, error) {
	switch s {
	case "auto":
		return ModeAuto, nil
	case "always":
		return ModeAlways, nil
	case "never":
		return ModeNever, nil
	default:
		return ModeAuto, fmt.Errorf("invalid --color value %q: must be one of auto, always, never", s)
	}
}

// Styler renders styled text for terminal output. The zero value is
// disabled, so callers that do not opt in stay byte-identical.
type Styler struct {
	enabled bool
}

// NewStyler resolves m against f and the environment into a Styler.
//
// Resolution order: ModeNever disables unconditionally. NO_COLOR set to any
// non-empty value (per no-color.org) disables next, even under ModeAlways —
// NO_COLOR is a user-level opt-out, and --color=always is an escape hatch
// for pipes, not an override of the environment. ModeAlways then enables
// unconditionally. ModeAuto enables only when f is an interactive terminal
// (see IsTTY, which also treats TERM=dumb as non-interactive).
func NewStyler(m Mode, f *os.File) Styler {
	if m == ModeNever {
		return Styler{}
	}
	if os.Getenv("NO_COLOR") != "" {
		return Styler{}
	}
	if m == ModeAlways {
		return Styler{enabled: true}
	}
	return Styler{enabled: IsTTY(f)}
}

// Enabled reports whether s renders ANSI escapes.
func (s Styler) Enabled() bool {
	return s.enabled
}

func (s Styler) wrap(code, v string) string {
	if !s.enabled {
		return v
	}
	return "\033[" + code + "m" + v + "\033[0m"
}

// Bold renders v in bold when enabled.
func (s Styler) Bold(v string) string {
	return s.wrap("1", v)
}

// Cyan renders v in cyan when enabled.
func (s Styler) Cyan(v string) string {
	return s.wrap("36", v)
}

// Dim renders v dimmed when enabled.
func (s Styler) Dim(v string) string {
	return s.wrap("2", v)
}
