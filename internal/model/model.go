package model

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/janstol/rails-kit/internal/config"
	"github.com/janstol/rails-kit/internal/term"
)

var (
	reClassDecl   = regexp.MustCompile(`^\s*class\s+([A-Z]\w*(?:::[A-Z]\w*)*)(?:\s*<\s*([A-Z]\w*(?:::[A-Z]\w*)*))?`)
	reConcern     = regexp.MustCompile(`^\s*include\s+(\S+)`)
	reSkipModules = regexp.MustCompile(`^(ActiveModel|ActiveRecord|ActiveSupport|Devise|Comparable|Enumerable)`)
	reAssoc       = regexp.MustCompile(`^\s*(has_many|has_one|belongs_to|has_and_belongs_to_many)\s+:(\w+)`)
	reScope       = regexp.MustCompile(`^\s*scope\s+:(\w+)`)
	reValidate    = regexp.MustCompile(`^\s*validate\s+:(\w+)`)
	reValidates   = regexp.MustCompile(`^\s*(validates_?\w*)\s+:(\w+)`)
	reCallback    = regexp.MustCompile(`^\s*(before_validation|after_validation|before_create|after_create_commit|after_create|before_update|after_update_commit|after_update|before_save|after_save_commit|after_save|before_destroy|after_destroy_commit|after_destroy|after_initialize|before_commit|after_commit|after_rollback|after_touch|after_find|around_\w+)\b`)
	reEnum        = regexp.MustCompile(`^\s*enum\s+:?(\w+)`)
	reDelegate    = regexp.MustCompile(`^\s*delegate\s+(.+)`)

	// Association option extractors
	reThrough     = regexp.MustCompile(`through:\s*:?"?(\w+)"?`)
	reClassName   = regexp.MustCompile(`class_name:\s*["'](\w[^"']+)["']`)
	rePolymorphic = regexp.MustCompile(`polymorphic:\s*true`)
	reDependent   = regexp.MustCompile(`dependent:\s*:(\w+)`)
	reOptional    = regexp.MustCompile(`optional:\s*true`)
	reInverseOf   = regexp.MustCompile(`inverse_of:\s*:?"?(\w+)"?`)
	reSource      = regexp.MustCompile(`source:\s*:?"?(\w+)"?`)

	// Validation type extractors
	reCallbackTarget = regexp.MustCompile(`:\s*(\w+)(?:\s*,|\s*$)`)
	reScopeArgs      = regexp.MustCompile(`->\s*\(\s*\w|->\s*\|\w|lambda\s*\{\s*\|`)
	reLambdaArgs     = regexp.MustCompile(`->\s*\(([^)]+)\)`)
	reTableName      = regexp.MustCompile(`^\s*self\.table_name\s*=\s*["']([^"']+)["']`)

	// Used in joinContinuations
	reInlineComment = regexp.MustCompile(`#[^"']*$`)

	// Used in the reValidates case for extracting extra fields
	reFieldOptionSuffix = regexp.MustCompile(`\s+[a-z_]\w+:.*$`)
	reConditionalSuffix = regexp.MustCompile(`,?\s*(if|unless):.*$`)
	reFieldSymbol       = regexp.MustCompile(`:([a-z_]\w*)`)

	// Used in extractValidationTypes
	rePresence     = regexp.MustCompile(`presence:\s*true`)
	reUniqueness   = regexp.MustCompile(`uniqueness`)
	reLength       = regexp.MustCompile(`length:`)
	reFormat       = regexp.MustCompile(`format:`)
	reNumericality = regexp.MustCompile(`numericality`)
	reInclusion    = regexp.MustCompile(`inclusion:`)
	reExclusion    = regexp.MustCompile(`exclusion:`)
	reConfirmation = regexp.MustCompile(`confirmation:\s*true`)
	reEmailFormat  = regexp.MustCompile(`email_format:`)
	reAllowNil     = regexp.MustCompile(`allow_nil:\s*true`)
	reAllowBlank   = regexp.MustCompile(`allow_blank:\s*true`)
	reOn           = regexp.MustCompile(`on:\s*:(\w+)`)

	// Used in underscore
	reHasUpper        = regexp.MustCompile(`[A-Z]`)
	reAcronymBoundary = regexp.MustCompile(`([A-Z\d]+)([A-Z][a-z])`)
	reWordBoundary    = regexp.MustCompile(`([a-z\d])([A-Z])`)
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
}

var errAmbiguousModelName = errors.New("ambiguous model name")

// IsAmbiguousError reports whether err indicates multiple model matches.
func IsAmbiguousError(err error) bool {
	return errors.Is(err, errAmbiguousModelName)
}

// Resolve finds the model file for the given name or path within railsRoot.
func Resolve(railsRoot, modelsPath, input string) (string, error) {
	modelsDir := config.ResolvePath(railsRoot, modelsPath)

	if strings.HasSuffix(input, ".rb") {
		path := input
		if !filepath.IsAbs(path) {
			abs, err := filepath.Abs(path)
			if err == nil {
				path = abs
			}
		}
		if _, err := os.Stat(path); err == nil {
			rel, err := filepath.Rel(modelsDir, path)
			if err != nil || strings.HasPrefix(rel, "..") {
				return "", fmt.Errorf("model file is outside models directory: %s", path)
			}
			return path, nil
		}
		cleanInput := input
		if !filepath.IsAbs(modelsPath) {
			prefix := filepath.ToSlash(filepath.Clean(modelsPath)) + "/"
			cleanInput = strings.TrimPrefix(filepath.ToSlash(cleanInput), prefix)
		}
		path3 := filepath.Join(modelsDir, cleanInput)
		if _, err := os.Stat(path3); err == nil {
			return path3, nil
		}
		path2 := filepath.Join(railsRoot, input)
		if _, err := os.Stat(path2); err == nil {
			rel, err := filepath.Rel(modelsDir, path2)
			if err != nil || strings.HasPrefix(rel, "..") {
				return "", fmt.Errorf("model file is outside models directory: %s", path2)
			}
			return path2, nil
		}
		return "", fmt.Errorf("model file not found: %s", input)
	}

	name := underscore(input)
	normalizedName := normalizeLookupName(name)
	target := normalizedName + ".rb"
	basenameOnly := filepath.Base(normalizedName) == normalizedName
	targetBase := filepath.Base(target)
	info, err := os.Stat(modelsDir)
	if err != nil {
		return "", fmt.Errorf("models path %s: %w", modelsDir, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("models path %s: not a directory", modelsDir)
	}
	var found string
	var matches []string
	err = filepath.WalkDir(modelsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		rel, relErr := filepath.Rel(modelsDir, path)
		if relErr != nil {
			return nil
		}
		if rel == target {
			found = path
			return fs.SkipAll
		}
		if basenameOnly && d.Name() == targetBase {
			matches = append(matches, path)
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("walking models path %s: %w", modelsDir, err)
	}
	if found != "" {
		return found, nil
	}
	if len(matches) > 1 {
		var relNames []string
		for _, m := range matches {
			if r, e := filepath.Rel(modelsDir, m); e == nil {
				relNames = append(relNames, filepath.ToSlash(r))
			} else {
				relNames = append(relNames, m)
			}
		}
		return "", fmt.Errorf("%w '%s', matches: %s", errAmbiguousModelName, input, strings.Join(relNames, ", "))
	}
	if len(matches) == 1 {
		return matches[0], nil
	}
	return "", fmt.Errorf("model file not found for '%s'", input)
}

// ListNames returns sorted snake_case model names (relative to the models
// directory, without .rb) for every model file under railsRoot's models path.
// Returns nil, nil if the models directory does not exist.
func ListNames(railsRoot, modelsPath string) ([]string, error) {
	modelsDir := config.ResolvePath(railsRoot, modelsPath)

	info, err := os.Stat(modelsDir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("models path %s: %w", modelsDir, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("models path %s: not a directory", modelsDir)
	}

	var names []string
	err = filepath.WalkDir(modelsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, ".rb") {
			rel, relErr := filepath.Rel(modelsDir, path)
			if relErr == nil {
				names = append(names, strings.TrimSuffix(filepath.ToSlash(rel), ".rb"))
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(names)
	return names, nil
}

// underscore converts a CamelCase or namespaced class name into its Rails
// autoloading-style snake_case path, mirroring ActiveSupport's `underscore`.
// Inputs without uppercase letters (already snake_case) are returned unchanged.
func underscore(input string) string {
	if !reHasUpper.MatchString(input) {
		return input
	}
	result := strings.ReplaceAll(input, "::", "/")
	result = reAcronymBoundary.ReplaceAllString(result, "${1}_${2}")
	result = reWordBoundary.ReplaceAllString(result, "${1}_${2}")
	return strings.ToLower(result)
}

func normalizeLookupName(name string) string {
	replaced := strings.ReplaceAll(name, "\\", string(filepath.Separator))
	replaced = strings.ReplaceAll(replaced, "/", string(filepath.Separator))
	return filepath.Clean(replaced)
}

// Parse reads a model file and returns its structural summary.
// NOTE: This function relies heavily on regular expressions to parse Ruby code.
// This approach is inherently fragile and may not correctly parse all Ruby
// code styles or complex constructs. It is a known trade-off for simplicity
// and avoiding a full Ruby parser dependency.
func Parse(modelPath, railsRoot, modelsPath string) (*Summary, error) {
	data, err := os.ReadFile(modelPath)
	if err != nil {
		return nil, fmt.Errorf("reading model file: %w", err)
	}

	rawLines := strings.Split(string(data), "\n")
	lines := joinContinuations(rawLines)

	s := &Summary{}

	rel, err := filepath.Rel(railsRoot, modelPath)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		rel = modelPath
	}
	s.RelPath = filepath.ToSlash(rel)

	// Derive class name from path relative to modelsPath, preserving namespace
	modelsDir := config.ResolvePath(railsRoot, modelsPath)
	namePart, err := filepath.Rel(modelsDir, modelPath)
	if err != nil || strings.HasPrefix(namePart, "..") {
		namePart = filepath.Base(modelPath)
	}
	namePart = strings.TrimSuffix(namePart, ".rb")
	var classSegments []string
	for _, seg := range strings.Split(namePart, string(filepath.Separator)) {
		parts := strings.Split(seg, "_")
		var camel string
		for _, p := range parts {
			if len(p) > 0 {
				camel += strings.ToUpper(p[:1]) + p[1:]
			}
		}
		classSegments = append(classSegments, camel)
	}
	s.ClassName = strings.Join(classSegments, "::")

	for _, line := range lines {
		switch {
		case reClassDecl.MatchString(line):
			m := reClassDecl.FindStringSubmatch(line)
			if len(m) > 2 {
				s.ParentClass = m[2]
			}

		case reTableName.MatchString(line):
			m := reTableName.FindStringSubmatch(line)
			s.TableName = m[1]

		case reConcern.MatchString(line):
			m := reConcern.FindStringSubmatch(line)
			mod := m[1]
			if !reSkipModules.MatchString(mod) {
				s.Concerns = append(s.Concerns, "  "+mod)
			}

		case reAssoc.MatchString(line):
			m := reAssoc.FindStringSubmatch(line)
			assocType, name := m[1], m[2]
			var opts []string
			if t := reThrough.FindStringSubmatch(line); t != nil {
				opts = append(opts, "through: "+t[1])
			}
			if c := reClassName.FindStringSubmatch(line); c != nil {
				opts = append(opts, "class_name: "+c[1])
			}
			if rePolymorphic.MatchString(line) {
				opts = append(opts, "polymorphic: true")
			}
			if d := reDependent.FindStringSubmatch(line); d != nil {
				opts = append(opts, "dependent: "+d[1])
			}
			if reOptional.MatchString(line) {
				opts = append(opts, "optional: true")
			}
			if inv := reInverseOf.FindStringSubmatch(line); inv != nil {
				opts = append(opts, "inverse_of: "+inv[1])
			}
			if src := reSource.FindStringSubmatch(line); src != nil {
				opts = append(opts, "source: "+src[1])
			}
			entry := "  " + assocType + " :" + name
			if len(opts) > 0 {
				entry += ", " + strings.Join(opts, ", ")
			}
			s.Assocs = append(s.Assocs, entry)

		case reValidate.MatchString(line) && !reValidates.MatchString(line):
			m := reValidate.FindStringSubmatch(line)
			s.Valids = append(s.Valids, "  validate :"+m[1]+" (custom)")

		case reValidates.MatchString(line):
			m := reValidates.FindStringSubmatch(line)
			macro, field := m[1], m[2]
			fields := []string{field}
			rest := line[strings.Index(line, ":"+field)+len(":"+field):]
			// Capture additional lowercase field symbols before option keys
			fieldSection := reFieldOptionSuffix.ReplaceAllString(rest, "")
			fieldSection = reConditionalSuffix.ReplaceAllString(fieldSection, "")
			for _, fm := range reFieldSymbol.FindAllStringSubmatch(fieldSection, -1) {
				fields = append(fields, fm[1])
			}
			types := extractValidationDetails(line)
			shortMacro := ""
			if macro != "validates" {
				shortMacro = strings.TrimPrefix(macro, "validates_")
			}
			if shortMacro != "" {
				found := false
				for _, t := range types {
					if t == shortMacro {
						found = true
						break
					}
				}
				if !found {
					types = append(types, shortMacro)
				}
			}
			entry := "  validates :" + strings.Join(fields, ", :")
			if len(types) > 0 {
				entry += ", " + strings.Join(types, ", ")
			}
			s.Valids = append(s.Valids, entry)

		case reScope.MatchString(line):
			m := reScope.FindStringSubmatch(line)
			name := m[1]
			if reScopeArgs.MatchString(line) {
				args := "..."
				if am := reLambdaArgs.FindStringSubmatch(line); am != nil {
					args = strings.TrimSpace(am[1])
				}
				s.Scopes = append(s.Scopes, "  "+name+"("+args+")")
			} else {
				s.Scopes = append(s.Scopes, "  "+name)
			}

		case reCallback.MatchString(line):
			m := reCallback.FindStringSubmatch(line)
			cb := m[1]
			target := ""
			if tm := reCallbackTarget.FindStringSubmatch(line[strings.Index(line, cb)+len(cb):]); tm != nil {
				target = " :" + tm[1]
			}
			s.Callbacks = append(s.Callbacks, "  "+cb+target)

		case reEnum.MatchString(line):
			m := reEnum.FindStringSubmatch(line)
			s.Enums = append(s.Enums, "  "+m[1])

		case reDelegate.MatchString(line):
			m := reDelegate.FindStringSubmatch(line)
			rest := strings.TrimSpace(m[1])
			if runes := []rune(rest); len(runes) > 80 {
				rest = string(runes[:80]) + "..."
			}
			s.Delegates = append(s.Delegates, "  delegate "+rest)
		}
	}
	return s, nil
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

// styleEntry colors the leading macro keyword of a "  macro ..." entry line
// produced by Parse, leaving the rest of the line untouched. Lines whose
// first token is not a known macro (bare names like concern or scope
// entries) pass through unchanged.
func styleEntry(entry string, st term.Styler) string {
	const indent = "  "
	if !strings.HasPrefix(entry, indent) {
		return entry
	}
	rest := entry[len(indent):]
	tok := rest
	if idx := strings.IndexByte(rest, ' '); idx >= 0 {
		tok = rest[:idx]
	}
	if !isMacroToken(tok) {
		return entry
	}
	return indent + st.Cyan(tok) + rest[len(tok):]
}

// Format renders the summary as a human-readable string. st controls
// terminal color accents; the zero value renders identically to the
// uncolored output.
func Format(s *Summary, st term.Styler) string {
	var sb strings.Builder
	sb.WriteString(st.Bold(s.ClassName))
	if s.ParentClass != "" {
		sb.WriteString(" < " + st.Cyan(s.ParentClass))
	}
	sb.WriteString(" " + st.Dim("("+s.RelPath+")") + "\n")
	sb.WriteString(st.Dim(strings.Repeat("=", 40)) + "\n")
	if s.TableName != "" {
		sb.WriteString("\n")
		sb.WriteString(st.Bold("Table:") + "\n")
		sb.WriteString("  " + s.TableName + "\n")
	}

	sections := []struct {
		label   string
		entries []string
	}{
		{"Concerns", s.Concerns},
		{"Associations", s.Assocs},
		{"Validations", s.Valids},
		{"Scopes", s.Scopes},
		{"Callbacks", s.Callbacks},
		{"Enums", s.Enums},
		{"Delegates", s.Delegates},
	}
	for _, sec := range sections {
		if len(sec.entries) == 0 {
			continue
		}
		sb.WriteString("\n")
		sb.WriteString(st.Bold(sec.label+":") + "\n")
		for _, e := range sec.entries {
			sb.WriteString(styleEntry(e, st) + "\n")
		}
	}
	sb.WriteString("\n")
	return sb.String()
}

// joinContinuations merges lines where a line ends with ',' and the next is more indented.
func joinContinuations(lines []string) []string {
	// Work on a copy to avoid mutating the input slice.
	work := make([]string, len(lines))
	copy(work, lines)

	result := make([]string, 0, len(work))
	i := 0
	for i < len(work) {
		line := strings.TrimRight(work[i], "\r\n")
		// Strip inline comment for continuation check
		stripped := reInlineComment.ReplaceAllString(line, "")
		stripped = strings.TrimRight(stripped, " \t")

		if strings.HasSuffix(stripped, ",") && i+1 < len(work) {
			indentCurr := len(line) - len(strings.TrimLeft(line, " \t"))
			nextLine := work[i+1]
			indentNext := len(nextLine) - len(strings.TrimLeft(nextLine, " \t"))
			if indentNext > indentCurr {
				combined := line + " " + strings.TrimSpace(nextLine)
				work[i+1] = strings.Repeat(" ", indentCurr) + strings.TrimLeft(combined, " \t")
				i++
				continue
			}
		}
		result = append(result, line)
		i++
	}
	return result
}

func extractValidationDetails(line string) []string {
	var details []string
	checks := []struct {
		re   *regexp.Regexp
		name string
	}{
		{rePresence, "presence"},
		{reUniqueness, "uniqueness"},
		{reLength, "length"},
		{reFormat, "format"},
		{reNumericality, "numericality"},
		{reInclusion, "inclusion"},
		{reExclusion, "exclusion"},
		{reConfirmation, "confirmation"},
		{reEmailFormat, "email_format"},
	}
	for _, c := range checks {
		if c.re.MatchString(line) {
			details = append(details, c.name)
		}
	}
	if reAllowNil.MatchString(line) {
		details = append(details, "allow_nil")
	}
	if reAllowBlank.MatchString(line) {
		details = append(details, "allow_blank")
	}
	if m := reOn.FindStringSubmatch(line); m != nil {
		details = append(details, "on: "+m[1])
	}
	return details
}
