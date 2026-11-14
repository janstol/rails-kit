package model

import (
	"errors"
	"strings"

	"github.com/janstol/rails-kit/internal/astutil"
	"github.com/janstol/rails-kit/internal/pluralize"
	"github.com/janstol/rails-kit/internal/reader"
	"github.com/janstol/rails-kit/internal/term"
)

// Summary holds the extracted model structure.
type Summary struct {
	ClassName   string
	ParentClass string
	RelPath     string
	TableName   string
	Concerns    []string
	Assocs      []string
	Valids      []string
	Scopes      []string
	Callbacks   []string
	Enums       []string
	Delegates   []string
	ParseErrors []ParseDiagnostic
}

// ParseDiagnostic describes a recoverable Ruby syntax error reported by Prism.
type ParseDiagnostic = astutil.ParseDiagnostic

var errAmbiguousModelName = errors.New("ambiguous model name")

// kind is configured for the model file layout: no universal filename suffix
// (no `_model` is appended), plus entry styling for Format.
var kind = reader.Kind{
	Noun:         "model",
	Plural:       "models",
	Suffix:       "",
	ErrAmbiguous: errAmbiguousModelName,
	IsMacro:      isMacroToken,
}

// IsAmbiguousError reports whether err indicates multiple model matches.
func IsAmbiguousError(err error) bool {
	return errors.Is(err, errAmbiguousModelName)
}

// Resolve finds the model file for the given name or path within railsRoot.
// The name may be a resource name ("user"), a CamelCase class name ("User",
// "Admin::Dashboard"), or a file path ending in .rb. Models have no universal
// suffix, so the name is matched as-is (no `_model` is appended).
func Resolve(railsRoot, modelsPath, input string) (string, error) {
	return kind.Resolve(railsRoot, modelsPath, input)
}

// ResolveName is Resolve with a singular fallback for plural names, since
// people often type the table name ("users", "order_items", "Admin::Users").
// The name as given always wins, so a model whose name is itself plural
// ("news", "settings") still resolves to its own file. Paths ending in .rb
// are never rewritten. If the singular form doesn't resolve either, the
// original error is returned, unless the singular is ambiguous, which is the
// more useful thing to report.
func ResolveName(railsRoot, modelsPath, input string, p *pluralize.Pluralizer) (string, error) {
	path, err := Resolve(railsRoot, modelsPath, input)
	if err == nil || IsAmbiguousError(err) || strings.HasSuffix(input, ".rb") {
		return path, err
	}

	name := astutil.Underscore(input)
	dir, base := "", name
	if i := strings.LastIndex(name, "/"); i >= 0 {
		dir, base = name[:i+1], name[i+1:]
	}
	singular := p.Singularize(base)
	if singular == strings.ToLower(base) {
		return "", err
	}

	path, singularErr := Resolve(railsRoot, modelsPath, dir+singular)
	if singularErr == nil || IsAmbiguousError(singularErr) {
		return path, singularErr
	}
	return "", err
}

// ListNames returns sorted snake_case model names (relative to the models
// directory, without .rb) for every model file under railsRoot's models path.
// Returns nil, nil if the models directory does not exist.
func ListNames(railsRoot, modelsPath string) ([]string, error) {
	return kind.ListNames(railsRoot, modelsPath)
}

// macroAllowlist holds the Rails macros whose entry line gets a color
// accent in Format. Bare-name sections (Concerns, Scopes, Enums) are
// intentionally excluded — their section label already carries the accent.
var macroAllowlist = map[string]bool{
	"has_many":                true,
	"has_one":                 true,
	"belongs_to":              true,
	"has_and_belongs_to_many": true,
	"validates":               true,
	"validate":                true,
	"delegate":                true,
}

func isMacroToken(tok string) bool {
	if macroAllowlist[tok] {
		return true
	}
	return strings.HasPrefix(tok, "before_") || strings.HasPrefix(tok, "after_") || strings.HasPrefix(tok, "around_")
}

// Format renders the summary as a human-readable string. st controls
// terminal color accents; the zero value renders identically to the
// uncolored output.
func Format(s *Summary, st term.Styler) string {
	return kind.Format(
		reader.Header{Title: s.ClassName, Parent: s.ParentClass, RelPath: s.RelPath},
		[]reader.Section{
			{Label: "Table", Value: s.TableName},
			{Label: "Concerns", Entries: s.Concerns},
			{Label: "Associations", Entries: s.Assocs},
			{Label: "Validations", Entries: s.Valids},
			{Label: "Scopes", Entries: s.Scopes},
			{Label: "Callbacks", Entries: s.Callbacks},
			{Label: "Enums", Entries: s.Enums},
			{Label: "Delegates", Entries: s.Delegates},
		},
		st,
	)
}
