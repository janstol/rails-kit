package pluralize_test

import (
	"testing"

	"github.com/janstol/rails-kit/internal/pluralize"
)

func TestPluralize(t *testing.T) {
	p := pluralize.Default()

	cases := []struct {
		in   string
		want string
	}{
		// Built-in irregulars
		{"person", "people"},
		{"child", "children"},
		{"datum", "data"},
		{"analysis", "analyses"},
		{"criterion", "criteria"},
		{"index", "indices"},
		{"matrix", "matrices"},
		{"man", "men"},
		{"woman", "women"},
		{"series", "series"},
		{"sheep", "sheep"},
		{"fish", "fish"},
		// Suffix rules: es
		{"match", "matches"},
		{"box", "boxes"},
		{"bus", "buses"},
		{"buzz", "buzzes"},
		{"wish", "wishes"},
		// Suffix rules: ies
		{"category", "categories"},
		{"city", "cities"},
		{"country", "countries"},
		// Vowel + y -> s
		{"day", "days"},
		{"key", "keys"},
		// Default: s
		{"location", "locations"},
		{"rental", "rentals"},
		{"device", "devices"},
		{"card", "cards"},
		// Idempotency: already-plural inputs should be returned unchanged
		{"users", "users"},
		{"orders", "orders"},
		{"locations", "locations"},
		{"categories", "categories"},
		{"matches", "matches"},
		{"boxes", "boxes"},
		{"wishes", "wishes"},
		{"people", "people"},
		{"children", "children"},
		{"data", "data"},
	}

	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			got := p.Pluralize(tc.in)
			if got != tc.want {
				t.Errorf("Pluralize(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestPluralizeWithExtras(t *testing.T) {
	p := pluralize.New(map[string]string{
		"curriculum": "curricula",
	})
	got := p.Pluralize("curriculum")
	if got != "curricula" {
		t.Errorf("got %q, want curricula", got)
	}
	// Built-ins still work
	got = p.Pluralize("person")
	if got != "people" {
		t.Errorf("got %q, want people", got)
	}
}

func TestSingularize(t *testing.T) {
	p := pluralize.New(map[string]string{
		"curriculum": "curricula",
	})

	cases := []struct {
		in   string
		want string
	}{
		{"users", "user"},
		{"categories", "category"},
		{"matches", "match"},
		{"people", "person"},
		{"curricula", "curriculum"},
		{"user", "user"},
	}

	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			if got := p.Singularize(tc.in); got != tc.want {
				t.Fatalf("Singularize(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
