package suggest_test

import (
	"reflect"
	"testing"

	"github.com/janstol/rails-kit/internal/suggest"
)

func TestClosest(t *testing.T) {
	names := []string{"user", "users", "post", "comment", "admin/dashboard", "order_item", "user_export", "category"}

	tests := []struct {
		input string
		want  []string
	}{
		{"usr", []string{"user"}},                 // deletion, short budget of 1
		{"uesr", []string{"user"}},                // transposition counts as one edit; "users" is two away
		{"dashbord", []string{"admin/dashboard"}}, // matched against the last segment
		{"Comment", nil},                          // exact match (case-insensitive) is not a suggestion
		{"comments", []string{"comment"}},         // plural typed for a singular name
		{"ordr_item", []string{"order_item"}},     // two-edit budget for mid-length names
		{"categroy", []string{"category"}},        // transposition in a longer name
		{"zzz", nil},                              // nothing close
		{"", nil},                                 // empty input
		{"pots", []string{"post"}},                // only names in the list are suggested
		{"usera", []string{"user", "users"}},      // equal distance: ordered by name
	}
	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got := suggest.Closest(tc.input, names)
			if len(got) == 0 && len(tc.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("Closest(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestClosestLimitsToThree(t *testing.T) {
	got := suggest.Closest("cat", []string{"bat", "cab", "car", "cut", "hat"})
	if want := []string{"bat", "cab", "car"}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestHint(t *testing.T) {
	tests := []struct {
		in   []string
		want string
	}{
		{nil, ""},
		{[]string{"a"}, "; did you mean 'a'?"},
		{[]string{"a", "b"}, "; did you mean 'a' or 'b'?"},
		{[]string{"a", "b", "c"}, "; did you mean 'a', 'b' or 'c'?"},
	}
	for _, tc := range tests {
		if got := suggest.Hint(tc.in); got != tc.want {
			t.Errorf("Hint(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
