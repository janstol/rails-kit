// Package term provides minimal TTY detection and screen clearing for
// interactive command output.
package term

import (
	"io"
	"os"
)

// IsTTY reports whether f is an interactive terminal that supports ANSI
// escapes. It must be called at render time rather than cached, since tests
// swap the os.Stdout package variable for a pipe.
func IsTTY(f *os.File) bool {
	if os.Getenv("TERM") == "dumb" {
		return false
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

// Clear writes an ANSI sequence that clears the terminal screen, cursor
// position, and scrollback.
func Clear(w io.Writer) {
	_, _ = io.WriteString(w, "\033[H\033[2J\033[3J")
}
