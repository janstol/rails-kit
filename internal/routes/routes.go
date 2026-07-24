package routes

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"text/tabwriter"
	"time"
)

var reRouteLine = regexp.MustCompile(`^\s*(?:(\S+)\s+)?([A-Z][A-Z0-9_/\-|]*)\s+(\S+)\s+(.+?)\s*$`)

// Run executes `bundle exec rails routes` and returns the output.
// If ctx has no deadline, a 60-second timeout is applied automatically.
// No cache is read or written.
// If stderr is nil, os.Stderr is used.
func Run(ctx context.Context, railsRoot string, stderr io.Writer) (string, error) {
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 60*time.Second)
		defer cancel()
	}

	if stderr == nil {
		stderr = os.Stderr
	}
	_, _ = fmt.Fprintln(stderr, "Running rails routes (this may take a moment)...")
	cmd := exec.CommandContext(ctx, "bundle", "exec", "rails", "routes")
	cmd.Dir = railsRoot
	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return "", fmt.Errorf("running rails routes: %w\nstderr: %s", err, exitErr.Stderr)
		}
		return "", fmt.Errorf("running rails routes: %w", err)
	}
	return string(out), nil
}

// writeCacheFiles writes the routes output to the cache file and updates the flag file.
func writeCacheFiles(railsRoot string, out string, stderr io.Writer) {
	cacheFile := filepath.Join(railsRoot, "tmp", "routes_cache.txt")
	routesDir := filepath.Join(railsRoot, "config", "routes")

	if err := os.MkdirAll(filepath.Dir(cacheFile), 0755); err != nil {
		_, _ = fmt.Fprintf(stderr, "Warning: could not create routes cache directory: %v\n", err)
		return
	}
	if err := os.WriteFile(cacheFile, []byte(out), 0644); err != nil {
		_, _ = fmt.Fprintf(stderr, "Warning: could not write routes cache: %v\n", err)
		return
	}

	// Track whether config/routes/ existed at cache-write time.
	flagFile := filepath.Join(railsRoot, "tmp", "routes_dir.flag")
	if _, err := os.Stat(routesDir); err == nil {
		_ = os.WriteFile(flagFile, []byte{}, 0644)
	} else {
		_ = os.Remove(flagFile)
	}
}

// Refresh unconditionally runs `bundle exec rails routes` and writes the result to cache.
// If ctx has no deadline, a 60-second timeout is applied automatically.
// If stderr is nil, os.Stderr is used.
func Refresh(ctx context.Context, railsRoot string, stderr io.Writer) (string, error) {
	if stderr == nil {
		stderr = os.Stderr
	}
	out, err := Run(ctx, railsRoot, stderr)
	if err != nil {
		return "", err
	}
	writeCacheFiles(railsRoot, out, stderr)
	return out, nil
}

// Cache reads cached routes output or regenerates it if stale.
// Returns the full routes output as a string.
// If ctx has no deadline, a 60-second timeout is applied automatically.
// If stderr is nil, os.Stderr is used.
func Cache(ctx context.Context, railsRoot string, stderr io.Writer) (string, error) {
	cacheFile := filepath.Join(railsRoot, "tmp", "routes_cache.txt")
	routesRb := filepath.Join(railsRoot, "config", "routes.rb")
	routesDir := filepath.Join(railsRoot, "config", "routes")

	if CacheValid(cacheFile, routesRb, routesDir) {
		data, err := os.ReadFile(cacheFile)
		if err != nil {
			return "", fmt.Errorf("reading routes cache: %w", err)
		}
		return string(data), nil
	}

	if stderr == nil {
		stderr = os.Stderr
	}
	out, err := Run(ctx, railsRoot, stderr)
	if err != nil {
		return "", err
	}

	writeCacheFiles(railsRoot, out, stderr)
	return out, nil
}

// findHeaderLine returns the index of the routes header line in lines.
// It looks for the canonical Rails columns first, then falls back to the
// first non-empty line. Returns -1 if no suitable line is found.
func findHeaderLine(lines []string) int {
	for i, l := range lines {
		if strings.Contains(l, "Prefix") && strings.Contains(l, "Verb") && strings.Contains(l, "URI Pattern") {
			return i
		}
	}
	for i, l := range lines {
		if strings.TrimSpace(l) != "" {
			return i
		}
	}
	return -1
}

// Filter returns header + matching body lines from routes output.
// Returns error if no lines match.
func Filter(output string, patterns []string) (string, error) {
	lines := strings.Split(output, "\n")
	if len(lines) == 0 {
		return "", fmt.Errorf("empty routes output")
	}

	headerIdx := findHeaderLine(lines)
	if headerIdx < 0 {
		return "", fmt.Errorf("empty routes output")
	}
	header := lines[headerIdx]
	body := lines[headerIdx+1:]

	lowered := make([]string, len(patterns))
	for i, p := range patterns {
		lowered[i] = strings.ToLower(p)
	}

	var matching []string
	for _, line := range body {
		lower := strings.ToLower(line)
		for _, p := range lowered {
			if strings.Contains(lower, p) {
				matching = append(matching, line)
				break
			}
		}
	}
	if len(matching) == 0 {
		return "", fmt.Errorf("no routes matching: %s", strings.Join(patterns, ", "))
	}
	return header + "\n" + strings.Join(matching, "\n") + "\n", nil
}

// RouteEntry represents a single parsed route from `rails routes` output.
type RouteEntry struct {
	Prefix           string `json:"prefix"`
	Verb             string `json:"verb"`
	URIPattern       string `json:"uri_pattern"`
	ControllerAction string `json:"controller_action"`
}

// ParseTable parses the columnar output of `rails routes` into a slice of RouteEntry.
// It splits rows on runs of 2+ spaces so it can handle blank prefixes and
// minor spacing differences across Rails versions.
func ParseTable(output string) ([]RouteEntry, error) {
	lines := strings.Split(output, "\n")

	// Find header line with canonical columns only — no fallback for ParseTable
	// since callers need a guaranteed structured format.
	headerIdx := -1
	for i, l := range lines {
		if strings.Contains(l, "Prefix") && strings.Contains(l, "Verb") && strings.Contains(l, "URI Pattern") {
			headerIdx = i
			break
		}
	}
	if headerIdx < 0 {
		return nil, fmt.Errorf("routes output is not in the standard tabular format")
	}

	var entries []RouteEntry
	for _, line := range lines[headerIdx+1:] {
		entry, ok := parseLine(line)
		if !ok {
			continue
		}
		entries = append(entries, entry)
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("routes output header found, but no route rows could be parsed")
	}
	return entries, nil
}

func parseLine(line string) (RouteEntry, bool) {
	m := reRouteLine.FindStringSubmatch(line)
	if m == nil {
		return RouteEntry{}, false
	}
	return RouteEntry{
		Prefix:           m[1],
		Verb:             m[2],
		URIPattern:       m[3],
		ControllerAction: m[4],
	}, true
}

// FormatTable renders entries as a columnar table matching `rails routes` output.
func FormatTable(entries []RouteEntry) string {
	var buf strings.Builder
	w := tabwriter.NewWriter(&buf, 0, 2, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "Prefix\tVerb\tURI Pattern\tController#Action")
	for _, e := range entries {
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", e.Prefix, e.Verb, e.URIPattern, e.ControllerAction)
	}
	_ = w.Flush()
	return buf.String()
}

// FilterEntries returns entries whose Prefix, Verb, URIPattern, or ControllerAction
// contains any of the given patterns (case-insensitive).
// Returns an error if no entries match.
func FilterEntries(entries []RouteEntry, patterns []string) ([]RouteEntry, error) {
	lowered := make([]string, len(patterns))
	for i, p := range patterns {
		lowered[i] = strings.ToLower(p)
	}

	var matching []RouteEntry
	for _, e := range entries {
		haystack := strings.ToLower(e.Prefix + " " + e.Verb + " " + e.URIPattern + " " + e.ControllerAction)
		for _, p := range lowered {
			if strings.Contains(haystack, p) {
				matching = append(matching, e)
				break
			}
		}
	}
	if len(matching) == 0 {
		return nil, fmt.Errorf("no routes matching: %s", strings.Join(patterns, ", "))
	}
	return matching, nil
}

func CacheValid(cacheFile, routesRb, routesDir string) bool {
	cacheInfo, err := os.Stat(cacheFile)
	if err != nil {
		return false
	}
	cacheMtime := cacheInfo.ModTime()

	routesInfo, err := os.Stat(routesRb)
	if err != nil || !cacheMtime.After(routesInfo.ModTime()) {
		return false
	}

	if dirInfo, err := os.Stat(routesDir); err == nil {
		if !cacheMtime.After(dirInfo.ModTime()) {
			return false
		}
		var invalid bool
		if err := filepath.WalkDir(routesDir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if invalid {
				return fs.SkipAll
			}
			if d.IsDir() {
				if path != routesDir {
					info, err := d.Info()
					if err != nil || !cacheMtime.After(info.ModTime()) {
						invalid = true
					}
				}
			} else if strings.HasSuffix(path, ".rb") {
				info, err := d.Info()
				if err != nil || !cacheMtime.After(info.ModTime()) {
					invalid = true
				}
			}
			return nil
		}); err != nil {
			return false
		}
		return !invalid
	}

	// config/routes/ doesn't exist. If the flag file indicates it existed at
	// cache-write time, the cache is stale.
	flagFile := filepath.Join(filepath.Dir(cacheFile), "routes_dir.flag")
	if _, err := os.Stat(flagFile); err == nil {
		return false
	}
	return true
}
