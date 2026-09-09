package testutil

import "strings"

// ContainsSubstr reports whether any element of slice contains substr.
func ContainsSubstr(slice []string, substr string) bool {
	for _, s := range slice {
		if strings.Contains(s, substr) {
			return true
		}
	}
	return false
}
