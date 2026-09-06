// Package reader holds the file-resolution, listing, and entry-styling
// conventions shared by the controllers, mailers, jobs, services, datagrids,
// and model readers. Each of those packages configures a Kind describing its
// own naming convention and delegates Resolve, ListNames, and StyleEntry to
// it, keeping the AST-parsing and Format logic (which does vary per reader)
// in the package itself.
package reader

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
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
		path := input
		if !filepath.IsAbs(path) {
			abs, err := filepath.Abs(path)
			if err == nil {
				path = abs
			}
		}
		if _, err := os.Stat(path); err == nil {
			rel, err := filepath.Rel(dir, path)
			if err != nil || strings.HasPrefix(rel, "..") {
				return "", fmt.Errorf("%s file is outside %s directory: %s", k.Noun, k.Plural, path)
			}
			return path, nil
		}
		cleanInput := input
		if !filepath.IsAbs(dirPath) {
			prefix := filepath.ToSlash(filepath.Clean(dirPath)) + "/"
			cleanInput = strings.TrimPrefix(filepath.ToSlash(cleanInput), prefix)
		}
		path3 := filepath.Join(dir, cleanInput)
		if _, err := os.Stat(path3); err == nil {
			return path3, nil
		}
		path2 := filepath.Join(railsRoot, input)
		if _, err := os.Stat(path2); err == nil {
			rel, err := filepath.Rel(dir, path2)
			if err != nil || strings.HasPrefix(rel, "..") {
				return "", fmt.Errorf("%s file is outside %s directory: %s", k.Noun, k.Plural, path2)
			}
			return path2, nil
		}
		return "", fmt.Errorf("%s file not found: %s", k.Noun, input)
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

	// Prefer the conventional suffixed file. Fall back to the name exactly
	// as given -- a handful of real apps keep .rb files directly under the
	// domain directory that don't follow that convention, and ListNames
	// surfaces those names unmodified. When Suffix is empty (services),
	// there is only the exact-name candidate.
	candidates := []string{normalizedName + ".rb"}
	if k.Suffix != "" {
		suffixed := normalizedName
		if !strings.HasSuffix(suffixed, k.Suffix) {
			suffixed += k.Suffix
		}
		candidates = []string{suffixed + ".rb"}
		if suffixed != normalizedName {
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
				if k.Suffix != "" {
					name = strings.TrimSuffix(name, k.Suffix)
				}
				names = append(names, name)
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

// StyleEntry colors the leading macro keyword of a "  macro ..." entry line
// produced by Parse, leaving the rest of the line untouched. Lines whose
// first token is not a known macro (bare names like a method), or Kinds with
// a nil IsMacro, pass through unchanged.
func (k Kind) StyleEntry(entry string, st term.Styler) string {
	const indent = "  "
	if !strings.HasPrefix(entry, indent) {
		return entry
	}
	rest := entry[len(indent):]
	tok := rest
	if idx := strings.IndexByte(rest, ' '); idx >= 0 {
		tok = rest[:idx]
	}
	if k.IsMacro == nil || !k.IsMacro(tok) {
		return entry
	}
	return indent + st.Cyan(tok) + rest[len(tok):]
}
