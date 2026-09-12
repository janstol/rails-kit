// Package reader holds the file-resolution, listing, and entry-styling
// conventions shared by the controllers, mailers, jobs, services, datagrids,
// helpers, decorators, formers, presenters, validators, and model readers.
// Each of those
// packages configures a Kind describing its own naming convention and
// delegates Resolve, ListNames, and StyleEntry to it, keeping the
// AST-parsing and Format logic (which does vary per reader) in the package
// itself.
package reader

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/janstol/rails-kit/internal/astutil"
	"github.com/janstol/rails-kit/internal/config"
	"github.com/janstol/rails-kit/internal/term"
)

// Kind describes one domain reader's file-layout conventions, so the
// resolution and listing logic can live here once instead of being
// copy-pasted per reader package.
type Kind struct {
	// Noun is the singular used in error text, e.g. "job" produces
	// "job file not found: %s".
	Noun string
	// Plural is used in error text for the configured directory, e.g.
	// "jobs" produces "jobs path %s: ...".
	Plural string
	// Suffix is the conventional filename suffix, e.g. "_job". Empty means
	// no universal suffix -- names are matched exactly as given (services).
	Suffix string
	// AltSuffixes holds additional filename suffix conventions checked after
	// Suffix, in order, e.g. formers' "_form" alongside its "_former" Suffix.
	// Empty for readers with a single settled convention.
	AltSuffixes []string
	// ErrAmbiguous is the caller's own sentinel error, so its errors.Is
	// relationship (via IsAmbiguousError) keeps working unchanged. Resolve
	// wraps it with %w.
	ErrAmbiguous error
	// IsMacro reports whether tok is a macro keyword that should get a
	// color accent in StyleEntry. Nil means no macro keywords are accented.
	IsMacro func(tok string) bool
}

// Resolve finds the file for the given name or path within railsRoot's
// dirPath. The name may be a short resource name, the full file basename
// (with Suffix), a CamelCase class name, or a file path ending in .rb.
func (k Kind) Resolve(railsRoot, dirPath, input string) (string, error) {
	dir := config.ResolvePath(railsRoot, dirPath)

	if strings.HasSuffix(input, ".rb") {
		return k.resolveRubyPath(railsRoot, dirPath, dir, input)
	}

	name := astutil.Underscore(input)
	normalizedName := astutil.NormalizeLookupName(name)
	info, err := os.Stat(dir)
	if err != nil {
		return "", fmt.Errorf("%s path %s: %w", k.Plural, dir, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%s path %s: not a directory", k.Plural, dir)
	}

	// Prefer the conventional suffixed file, then each alt suffix in order
	// (formers' "_form" alongside "_former"). Fall back to the name exactly
	// as given -- a handful of real apps keep .rb files directly under the
	// domain directory that don't follow that convention, and ListNames
	// surfaces those names unmodified. When Suffix is empty (services),
	// there is only the exact-name candidate.
	candidates := []string{normalizedName + ".rb"}
	if k.Suffix != "" {
		suffixed := appendSuffix(normalizedName, k.Suffix)
		candidates = []string{suffixed + ".rb"}
		for _, alt := range k.AltSuffixes {
			candidates = append(candidates, appendSuffix(normalizedName, alt)+".rb")
		}
		if suffixed != normalizedName || len(k.AltSuffixes) > 0 {
			candidates = append(candidates, normalizedName+".rb")
		}
	}

	for _, target := range candidates {
		found, matches, err := findFile(dir, target, k.Plural)
		if err != nil {
			return "", err
		}
		if found != "" {
			return found, nil
		}
		if len(matches) > 1 {
			var relNames []string
			for _, m := range matches {
				if r, e := filepath.Rel(dir, m); e == nil {
					relNames = append(relNames, filepath.ToSlash(r))
				} else {
					relNames = append(relNames, m)
				}
			}
			return "", fmt.Errorf("%w '%s', matches: %s", k.ErrAmbiguous, input, strings.Join(relNames, ", "))
		}
		if len(matches) == 1 {
			return matches[0], nil
		}
	}
	return "", fmt.Errorf("%s file not found for '%s'", k.Noun, input)
}

// resolveRubyPath handles the .rb branch of Resolve: input already names a
// file rather than a resource name. It tries three interpretations of input
// in order -- the input made absolute against the current working directory,
// the input joined onto dir after stripping a leading dirPath prefix, and
// the input joined onto railsRoot -- and validates all three against dir, so
// the answer no longer depends on the caller's working directory.
func (k Kind) resolveRubyPath(railsRoot, dirPath, dir, input string) (string, error) {
	path := input
	if !filepath.IsAbs(path) {
		if abs, err := filepath.Abs(path); err == nil {
			path = abs
		}
	}

	cleanInput := input
	if !filepath.IsAbs(dirPath) {
		prefix := filepath.ToSlash(filepath.Clean(dirPath)) + "/"
		cleanInput = strings.TrimPrefix(filepath.ToSlash(cleanInput), prefix)
	}

	candidates := []string{path, filepath.Join(dir, cleanInput), filepath.Join(railsRoot, input)}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err != nil {
			continue
		}
		if !withinDir(dir, candidate) {
			return "", fmt.Errorf("%s file is outside %s directory: %s", k.Noun, k.Plural, candidate)
		}
		return candidate, nil
	}
	return "", fmt.Errorf("%s file not found: %s", k.Noun, input)
}

// withinDir reports whether path is dir itself or lies somewhere under it,
// using path components rather than a raw string prefix -- so a sibling
// directory whose name happens to start with dir's name (or, symmetrically,
// a subdirectory of dir whose name starts with "..") isn't mistaken for an
// escape. Matches the form used by pathOutsideRoot (cmd/skeleton.go) and
// pathWithin (internal/routes/static.go).
func withinDir(dir, path string) bool {
	rel, err := filepath.Rel(dir, path)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// appendSuffix appends suffix to name unless name already ends with it.
func appendSuffix(name, suffix string) string {
	if strings.HasSuffix(name, suffix) {
		return name
	}
	return name + suffix
}

// findFile looks for target (a dir-relative path) under dir. If target has no
// path separators, it also matches by basename anywhere in the tree,
// returning every match so the caller can detect ambiguity. plural names the
// domain directory for the wrapped walk error (e.g. "jobs").
func findFile(dir, target, plural string) (found string, matches []string, err error) {
	basenameOnly := filepath.Base(target) == target
	targetBase := filepath.Base(target)
	err = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		rel, relErr := filepath.Rel(dir, path)
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
		return "", nil, fmt.Errorf("walking %s path %s: %w", plural, dir, err)
	}
	return found, matches, nil
}

// ListNames returns sorted snake_case names (relative to dirPath, without
// .rb, with Suffix stripped) for every file under railsRoot's dirPath.
// skipDirs holds already-resolved absolute directories to exclude from the
// walk (only controllers uses this, for its concerns/ directory). Returns
// nil, nil if the directory does not exist.
func (k Kind) ListNames(railsRoot, dirPath string, skipDirs ...string) ([]string, error) {
	dir := config.ResolvePath(railsRoot, dirPath)

	info, err := os.Stat(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("%s path %s: %w", k.Plural, dir, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s path %s: not a directory", k.Plural, dir)
	}

	skip := make(map[string]bool, len(skipDirs))
	for _, s := range skipDirs {
		skip[s] = true
	}

	var names []string
	err = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if skip[path] {
				return fs.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, ".rb") {
			rel, relErr := filepath.Rel(dir, path)
			if relErr == nil {
				name := strings.TrimSuffix(filepath.ToSlash(rel), ".rb")
				name = k.stripSuffix(name)
				names = append(names, name)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(names)
	// Two files that reduce to the same short name under different
	// conventions (e.g. a "_former" and a "_form" file for the same
	// resource) list once rather than twice.
	names = slices.Compact(names)
	return names, nil
}

// stripSuffix removes the first suffix that matches name -- Suffix, then
// each AltSuffixes entry in order -- or returns name unchanged if none match.
func (k Kind) stripSuffix(name string) string {
	if k.Suffix != "" && strings.HasSuffix(name, k.Suffix) {
		return strings.TrimSuffix(name, k.Suffix)
	}
	for _, alt := range k.AltSuffixes {
		if strings.HasSuffix(name, alt) {
			return strings.TrimSuffix(name, alt)
		}
	}
	return name
}

// StyleEntry colors the leading macro keyword of an indented "  macro ..."
// entry line produced by Parse, leaving the rest of the line untouched. The
// indent width is not fixed at two spaces: a former's nested `with_options`
// children render four spaces deep, so the leading whitespace is trimmed to
// find the token and re-emitted as-is rather than assumed. Lines with no
// leading indent (unindented entries), lines whose first token is not a known
// macro (bare names like a method), or Kinds with a nil IsMacro, pass through
// unchanged.
func (k Kind) StyleEntry(entry string, st term.Styler) string {
	rest := strings.TrimLeft(entry, " ")
	indent := entry[:len(entry)-len(rest)]
	if indent == "" {
		return entry
	}
	tok := rest
	if idx := strings.IndexByte(rest, ' '); idx >= 0 {
		tok = rest[:idx]
	}
	if k.IsMacro == nil || !k.IsMacro(tok) {
		return entry
	}
	return indent + st.Cyan(tok) + rest[len(tok):]
}
