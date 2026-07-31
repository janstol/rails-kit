package term

import (
	"os"
	"strings"
	"testing"
)

func TestParseMode(t *testing.T) {
	cases := []struct {
		in   string
		want Mode
	}{
		{"auto", ModeAuto},
		{"always", ModeAlways},
		{"never", ModeNever},
	}
	for _, tc := range cases {
		got, err := ParseMode(tc.in)
		if err != nil {
			t.Errorf("ParseMode(%q): unexpected error: %v", tc.in, err)
		}
		if got != tc.want {
			t.Errorf("ParseMode(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestParseMode_Invalid(t *testing.T) {
	_, err := ParseMode("bogus")
	if err == nil {
		t.Fatal("expected error for invalid mode")
	}
	got := err.Error()
	for _, sub := range []string{"auto", "always", "never"} {
		if !strings.Contains(got, sub) {
			t.Errorf("error %q does not name valid value %q", got, sub)
		}
	}
}

// nonTTYFile returns an *os.File that is guaranteed not to be a character
// device, standing in for a piped stdout.
func nonTTYFile(t *testing.T) *os.File {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = r.Close()
		_ = w.Close()
	})
	return w
}

func TestNewStyler(t *testing.T) {
	pipe := nonTTYFile(t)

	cases := []struct {
		name    string
		mode    Mode
		noColor string // "" = unset
		want    bool
	}{
		{"never/no NO_COLOR", ModeNever, "", false},
		{"never/NO_COLOR set", ModeNever, "1", false},
		{"always/no NO_COLOR", ModeAlways, "", true},
		{"always/NO_COLOR set", ModeAlways, "1", false},
		{"auto/no NO_COLOR/pipe", ModeAuto, "", false},
		{"auto/NO_COLOR set/pipe", ModeAuto, "1", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("NO_COLOR", tc.noColor)
			st := NewStyler(tc.mode, pipe)
			if st.Enabled() != tc.want {
				t.Errorf("NewStyler(%v, pipe).Enabled() = %v, want %v", tc.mode, st.Enabled(), tc.want)
			}
		})
	}
}

func TestStyler_ZeroValueDisabled(t *testing.T) {
	var st Styler
	if st.Enabled() {
		t.Fatal("zero-value Styler must be disabled")
	}
	if got := st.Bold("x"); got != "x" {
		t.Errorf("Bold on disabled styler = %q, want unchanged", got)
	}
	if got := st.Cyan("x"); got != "x" {
		t.Errorf("Cyan on disabled styler = %q, want unchanged", got)
	}
	if got := st.Dim("x"); got != "x" {
		t.Errorf("Dim on disabled styler = %q, want unchanged", got)
	}
}

func TestStyler_EnabledWraps(t *testing.T) {
	st := NewStyler(ModeAlways, nonTTYFile(t))
	if !st.Enabled() {
		t.Fatal("expected enabled styler")
	}
	if got, want := st.Bold("x"), "\033[1mx\033[0m"; got != want {
		t.Errorf("Bold(%q) = %q, want %q", "x", got, want)
	}
	if got, want := st.Cyan("x"), "\033[36mx\033[0m"; got != want {
		t.Errorf("Cyan(%q) = %q, want %q", "x", got, want)
	}
	if got, want := st.Dim("x"), "\033[2mx\033[0m"; got != want {
		t.Errorf("Dim(%q) = %q, want %q", "x", got, want)
	}
}
