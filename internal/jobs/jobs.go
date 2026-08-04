// Package jobs extracts a structural summary of a Rails ActiveJob file -- its
// queue, retry_on/discard_on handlers, included concerns, and public methods
// (notably `perform`) -- without booting Rails.
//
// Only the job's own file is parsed; queue, retry, or discard behavior
// inherited from ApplicationJob (or any other superclass) is not resolved.
// Summary reports ParentClass so callers know where to look next.
package jobs

import (
	"errors"
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

// Summary holds the extracted job structure.
type Summary struct {
	ClassName   string
	ParentClass string
	RelPath     string
	Concerns    []string
	Queue       string
	RetryOn     []string
	DiscardOn   []string
	Methods     []string
	ParseErrors []ParseDiagnostic
}

// ParseDiagnostic describes a recoverable Ruby syntax error reported by Prism.
type ParseDiagnostic struct {
	Line    int
	Message string
}

var errAmbiguousJobName = errors.New("ambiguous job name")

// IsAmbiguousError reports whether err indicates multiple job matches.
func IsAmbiguousError(err error) bool {
	return errors.Is(err, errAmbiguousJobName)
}

// Resolve finds the job file for the given name or path within railsRoot.
// The name may be a short resource name ("sync_user"), the full file basename
// ("sync_user_job"), a CamelCase class name ("SyncUserJob",
// "Admin::ExportJob"), or a file path ending in .rb.
func Resolve(railsRoot, jobsPath, input string) (string, error) {
	jobsDir := config.ResolvePath(railsRoot, jobsPath)

	if strings.HasSuffix(input, ".rb") {
		path := input
		if !filepath.IsAbs(path) {
			abs, err := filepath.Abs(path)
			if err == nil {
				path = abs
			}
		}
		if _, err := os.Stat(path); err == nil {
			rel, err := filepath.Rel(jobsDir, path)
			if err != nil || strings.HasPrefix(rel, "..") {
				return "", fmt.Errorf("job file is outside jobs directory: %s", path)
			}
			return path, nil
		}
		cleanInput := input
		if !filepath.IsAbs(jobsPath) {
			prefix := filepath.ToSlash(filepath.Clean(jobsPath)) + "/"
			cleanInput = strings.TrimPrefix(filepath.ToSlash(cleanInput), prefix)
		}
		path3 := filepath.Join(jobsDir, cleanInput)
		if _, err := os.Stat(path3); err == nil {
			return path3, nil
		}
		path2 := filepath.Join(railsRoot, input)
		if _, err := os.Stat(path2); err == nil {
			rel, err := filepath.Rel(jobsDir, path2)
			if err != nil || strings.HasPrefix(rel, "..") {
				return "", fmt.Errorf("job file is outside jobs directory: %s", path2)
			}
			return path2, nil
		}
		return "", fmt.Errorf("job file not found: %s", input)
	}

	name := astutil.Underscore(input)
	normalizedName := astutil.NormalizeLookupName(name)
	info, err := os.Stat(jobsDir)
	if err != nil {
		return "", fmt.Errorf("jobs path %s: %w", jobsPath, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("jobs path %s: not a directory", jobsPath)
	}

	// Prefer the conventional "*_job.rb" file. Fall back to the name exactly
	// as given -- a handful of real apps keep .rb files directly under
	// app/jobs that don't follow that convention, and ListNames surfaces
	// those names unmodified.
	suffixed := normalizedName
	if !strings.HasSuffix(suffixed, "_job") {
		suffixed += "_job"
	}
	candidates := []string{suffixed + ".rb"}
	if suffixed != normalizedName {
		candidates = append(candidates, normalizedName+".rb")
	}

	for _, target := range candidates {
		found, matches, err := findJobFile(jobsDir, target)
		if err != nil {
			return "", err
		}
		if found != "" {
			return found, nil
		}
		if len(matches) > 1 {
			var relNames []string
			for _, m := range matches {
				if r, e := filepath.Rel(jobsDir, m); e == nil {
					relNames = append(relNames, filepath.ToSlash(r))
				} else {
					relNames = append(relNames, m)
				}
			}
			return "", fmt.Errorf("%w '%s', matches: %s", errAmbiguousJobName, input, strings.Join(relNames, ", "))
		}
		if len(matches) == 1 {
			return matches[0], nil
		}
	}
	return "", fmt.Errorf("job file not found for '%s'", input)
}

// findJobFile looks for target (a jobs-dir-relative path) under jobsDir. If
// target has no path separators, it also matches by basename anywhere in the
// tree, returning every match so the caller can detect ambiguity.
func findJobFile(jobsDir, target string) (found string, matches []string, err error) {
	basenameOnly := filepath.Base(target) == target
	targetBase := filepath.Base(target)
	err = filepath.WalkDir(jobsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		rel, relErr := filepath.Rel(jobsDir, path)
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
		return "", nil, fmt.Errorf("walking jobs path %s: %w", jobsDir, err)
	}
	return found, matches, nil
}

// ListNames returns sorted snake_case job names (relative to the jobs
// directory, without .rb, with the "_job" suffix stripped) for every job file
// under railsRoot's jobs path. Returns nil, nil if the jobs directory does not
// exist.
func ListNames(railsRoot, jobsPath string) ([]string, error) {
	jobsDir := config.ResolvePath(railsRoot, jobsPath)

	info, err := os.Stat(jobsDir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("jobs path %s: %w", jobsDir, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("jobs path %s: not a directory", jobsDir)
	}

	var names []string
	err = filepath.WalkDir(jobsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if strings.HasSuffix(path, ".rb") {
			rel, relErr := filepath.Rel(jobsDir, path)
			if relErr == nil {
				name := strings.TrimSuffix(filepath.ToSlash(rel), ".rb")
				name = strings.TrimSuffix(name, "_job")
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

// macroAllowlist holds the job macros whose entry line gets a color accent in
// Format. queue_as renders its own value (the macro keyword is not part of the
// entry when it is a bare symbol/string), but retry_on and discard_on entries
// carry their keyword, so they are accent targets.
var macroAllowlist = map[string]bool{
	"queue_as":   true,
	"retry_on":   true,
	"discard_on": true,
}

func isMacroToken(tok string) bool {
	return macroAllowlist[tok]
}

// styleEntry colors the leading macro keyword of a "  macro ..." entry line
// produced by Parse, leaving the rest of the line untouched. Lines whose first
// token is not a known macro (bare names like a job method) pass through
// unchanged.
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

// Format renders the summary as a human-readable string. st controls terminal
// color accents; the zero value renders identically to the uncolored output.
func Format(s *Summary, st term.Styler) string {
	var sb strings.Builder
	sb.WriteString(st.Bold(s.ClassName))
	if s.ParentClass != "" {
		sb.WriteString(" < " + st.Cyan(s.ParentClass))
	}
	sb.WriteString(" " + st.Dim("("+s.RelPath+")") + "\n")
	sb.WriteString(st.Dim(strings.Repeat("=", 40)) + "\n")

	if s.Queue != "" {
		sb.WriteString("\n")
		sb.WriteString(st.Bold("Queue:") + "\n")
		sb.WriteString("  " + s.Queue + "\n")
	}

	sections := []struct {
		label   string
		entries []string
	}{
		{"Concerns", s.Concerns},
		{"Retry On", s.RetryOn},
		{"Discard On", s.DiscardOn},
		{"Job Methods", s.Methods},
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
