// Package suggest finds near matches for a name that was not found, so
// not-found errors can end with a "did you mean ...?" hint.
package suggest

import (
	"sort"
	"strings"
)

const maxSuggestions = 3

// Closest returns up to three candidates close to input, nearest first (ties
// in name order). A candidate matches when the edit distance from input to
// the candidate, or to the candidate's last path segment (so "dashbord" finds
// "admin/dashboard"), is within a budget that grows with the input's length:
// one edit up to four characters, two up to eight, three beyond. Adjacent
// transpositions count as one edit. Comparison is case-insensitive.
func Closest(input string, candidates []string) []string {
	needle := strings.ToLower(input)
	if needle == "" {
		return nil
	}
	budget := budgetFor(len([]rune(needle)))

	type match struct {
		name string
		dist int
	}
	var matches []match
	seen := make(map[string]bool, len(candidates))
	for _, c := range candidates {
		if seen[c] {
			continue
		}
		seen[c] = true
		lower := strings.ToLower(c)
		if lower == needle {
			continue
		}
		d := distance(needle, lower)
		if i := strings.LastIndex(lower, "/"); i >= 0 {
			d = min(d, distance(needle, lower[i+1:]))
		}
		if d <= budget {
			matches = append(matches, match{c, d})
		}
	}

	sort.Slice(matches, func(i, j int) bool {
		if matches[i].dist != matches[j].dist {
			return matches[i].dist < matches[j].dist
		}
		return matches[i].name < matches[j].name
	})
	if len(matches) > maxSuggestions {
		matches = matches[:maxSuggestions]
	}
	names := make([]string, len(matches))
	for i, m := range matches {
		names[i] = m.name
	}
	return names
}

// Hint formats suggestions for the end of an error message:
// "; did you mean 'a'?", "; did you mean 'a' or 'b'?",
// "; did you mean 'a', 'b' or 'c'?", or "" when there are none.
func Hint(suggestions []string) string {
	if len(suggestions) == 0 {
		return ""
	}
	quoted := make([]string, len(suggestions))
	for i, s := range suggestions {
		quoted[i] = "'" + s + "'"
	}
	list := quoted[0]
	if n := len(quoted); n > 1 {
		list = strings.Join(quoted[:n-1], ", ") + " or " + quoted[n-1]
	}
	return "; did you mean " + list + "?"
}

func budgetFor(n int) int {
	switch {
	case n <= 4:
		return 1
	case n <= 8:
		return 2
	default:
		return 3
	}
}

// distance is the optimal string alignment distance between a and b:
// insertions, deletions, substitutions and adjacent transpositions each
// cost one.
func distance(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	prev2 := make([]int, len(rb)+1)
	prev := make([]int, len(rb)+1)
	cur := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
			if i > 1 && j > 1 && ra[i-1] == rb[j-2] && ra[i-2] == rb[j-1] {
				cur[j] = min(cur[j], prev2[j-2]+1)
			}
		}
		prev2, prev, cur = prev, cur, prev2
	}
	return prev[len(rb)]
}
