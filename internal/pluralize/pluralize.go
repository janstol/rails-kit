package pluralize

import "strings"

// builtinIrregulars covers common Rails model name irregulars.
var builtinIrregulars = map[string]string{
	"person":    "people",
	"child":     "children",
	"datum":     "data",
	"analysis":  "analyses",
	"criterion": "criteria",
	"index":     "indices",
	"matrix":    "matrices",
	"man":       "men",
	"woman":     "women",
	"series":    "series",
	"sheep":     "sheep",
	"fish":      "fish",
}

// Pluralizer holds the irregular map merged with any project-specific overrides.
type Pluralizer struct {
	irregulars       map[string]string // singular → plural
	irregularPlurals map[string]bool   // plural → true (reverse lookup)
	singulars        map[string]string // plural -> singular
}

// New creates a Pluralizer with the built-in irregular map plus any extras.
func New(extras map[string]string) *Pluralizer {
	m := make(map[string]string, len(builtinIrregulars)+len(extras))
	rev := make(map[string]bool, len(builtinIrregulars)+len(extras))
	singulars := make(map[string]string, len(builtinIrregulars)+len(extras))
	for k, v := range builtinIrregulars {
		m[k] = v
		rev[v] = true
		singulars[v] = k
	}
	for k, v := range extras {
		singular := strings.ToLower(k)
		plural := strings.ToLower(v)
		m[singular] = plural
		rev[plural] = true
		singulars[plural] = singular
	}
	return &Pluralizer{irregulars: m, irregularPlurals: rev, singulars: singulars}
}

// Default returns a Pluralizer with only the built-in irregulars.
func Default() *Pluralizer {
	return New(nil)
}

// Pluralize returns the plural form of word. Calling Pluralize on an already-plural
// word returns it unchanged (idempotent for common regular and irregular plurals).
func (p *Pluralizer) Pluralize(word string) string {
	lower := strings.ToLower(word)

	// Direct singular→plural irregular lookup.
	if plural, ok := p.irregulars[lower]; ok {
		return plural
	}

	// Reverse irregular lookup: already a known plural.
	if p.irregularPlurals[lower] {
		return lower
	}

	// Idempotency check for regular plurals: detect already-plural forms by
	// trying to reverse the suffix rules and re-pluralizing.
	if p.seemsAlreadyPlural(lower) {
		return lower
	}

	return p.pluralizeRegular(lower)
}

// Singularize returns the singular form of a plural word when it matches a
// supported irregular or suffix rule. Non-plural words are returned unchanged.
func (p *Pluralizer) Singularize(word string) string {
	lower := strings.ToLower(word)

	if singular, ok := p.singulars[lower]; ok {
		return singular
	}

	switch {
	case strings.HasSuffix(lower, "ies") && len(lower) > 3:
		candidate := lower[:len(lower)-3] + "y"
		if p.Pluralize(candidate) == lower {
			return candidate
		}
	case (strings.HasSuffix(lower, "ses") || strings.HasSuffix(lower, "xes") ||
		strings.HasSuffix(lower, "zes") || strings.HasSuffix(lower, "ches") ||
		strings.HasSuffix(lower, "shes")) && len(lower) > 2:
		candidate := lower[:len(lower)-2]
		if p.Pluralize(candidate) == lower {
			return candidate
		}
	case strings.HasSuffix(lower, "s") && len(lower) > 1:
		candidate := lower[:len(lower)-1]
		if p.Pluralize(candidate) == lower {
			return candidate
		}
	}

	return lower
}

// seemsAlreadyPlural returns true if lower appears to already be in plural form.
func (p *Pluralizer) seemsAlreadyPlural(lower string) bool {
	switch {
	case strings.HasSuffix(lower, "ies") && len(lower) > 3:
		// "parties" → base "party" → pluralize → "parties" ✓
		base := lower[:len(lower)-3] + "y"
		return p.pluralizeRegular(base) == lower

	case (strings.HasSuffix(lower, "ses") || strings.HasSuffix(lower, "xes") ||
		strings.HasSuffix(lower, "zes") || strings.HasSuffix(lower, "ches") ||
		strings.HasSuffix(lower, "shes")) && len(lower) > 3:
		// "classes" → base "class" → pluralize → "classes" ✓
		base := lower[:len(lower)-2]
		return p.pluralizeRegular(base) == lower

	case strings.HasSuffix(lower, "s") && len(lower) > 2:
		// "users" → base "user" → base ends in consonant, not special suffix
		// → pluralize → "users" ✓
		// "bus" → base "bu" → ends in vowel → skip (keeps "bus" → "buses" working)
		// "status" → base "statu" → ends in vowel → skip (keeps "status" → "statuses")
		base := lower[:len(lower)-1]
		if len(base) >= 2 && !isVowel(base[len(base)-1]) && !endsInSpecialPluralSuffix(base) {
			return p.pluralizeRegular(base) == lower
		}
	}
	return false
}

// pluralizeRegular applies suffix rules to produce a plural form.
func (p *Pluralizer) pluralizeRegular(lower string) string {
	switch {
	case strings.HasSuffix(lower, "s"),
		strings.HasSuffix(lower, "x"),
		strings.HasSuffix(lower, "z"),
		strings.HasSuffix(lower, "ch"),
		strings.HasSuffix(lower, "sh"):
		return lower + "es"
	case len(lower) > 1 && strings.HasSuffix(lower, "y") && !isVowel(lower[len(lower)-2]):
		return lower[:len(lower)-1] + "ies"
	default:
		return lower + "s"
	}
}

// endsInSpecialPluralSuffix returns true if the word ends in a suffix that
// would trigger the "add es" or "ies" plural rule.
func endsInSpecialPluralSuffix(s string) bool {
	return strings.HasSuffix(s, "s") ||
		strings.HasSuffix(s, "x") ||
		strings.HasSuffix(s, "z") ||
		strings.HasSuffix(s, "ch") ||
		strings.HasSuffix(s, "sh") ||
		(len(s) > 1 && strings.HasSuffix(s, "y") && !isVowel(s[len(s)-2]))
}

func isVowel(c byte) bool {
	return c == 'a' || c == 'e' || c == 'i' || c == 'o' || c == 'u'
}
