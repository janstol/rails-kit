package cmd

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/janstol/rails-kit/internal/config"
	"github.com/janstol/rails-kit/internal/model"
	"github.com/janstol/rails-kit/internal/prism"
)

var prismRunner = prism.Runner{}
var skeletonExcludes []string

const maxSkeletonFiles = 500

type skeletonInput struct {
	path    string
	relPath string
}

var skeletonCmd = &cobra.Command{
	Use:   "skeleton <path-or-model> [path-or-model...]",
	Short: "Compact Ruby AST skeleton via Prism",
	Long: `Show compact structural skeletons for Ruby files using Prism.

Inputs can be Rails-root-relative .rb paths, absolute .rb paths under the
Rails root, model names, directories, or glob patterns. Directories are
searched recursively. Quote globs to have rails-kit expand them from the
Rails root. Use repeatable --exclude patterns to prune directory discovery;
at most 500 unique files may be inspected at once.

The command shells out to Ruby and requires Prism to be available. Existing
rails-kit commands do not require Prism.`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		root, cfg, err := loadConfig()
		if err != nil {
			return err
		}

		inputs, err := resolveSkeletonPathsWithExcludes(root, cfg, args, skeletonExcludes)
		if err != nil {
			return err
		}

		parentCtx := cmd.Context()
		if parentCtx == nil {
			parentCtx = context.Background()
		}
		ctx, cancel := context.WithTimeout(parentCtx, skeletonTimeout(len(inputs)))
		defer cancel()

		paths := make([]string, len(inputs))
		for i, input := range inputs {
			paths[i] = input.path
		}
		runner := prismRunner
		if runner.Dir == "" {
			runner.Dir = root
		}
		files, err := runner.ParseFiles(ctx, paths)
		if err != nil {
			if prism.IsUnavailable(err) {
				return fmt.Errorf("prism is not available: install/use Ruby with the prism library available, then retry: %w", err)
			}
			return err
		}
		for i := range files {
			files[i].RelPath = inputs[i].relPath
		}

		if jsonFlag {
			if len(files) == 1 {
				return printJSON(files[0])
			}
			return printJSON(files)
		}
		for _, file := range files {
			fmt.Print(prism.Format(file))
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(skeletonCmd)
	skeletonCmd.Flags().StringArrayVar(&skeletonExcludes, "exclude", nil, "Exclude a Rails-root-relative glob from directory discovery (repeatable)")
}

func skeletonTimeout(fileCount int) time.Duration {
	const (
		base       = 10 * time.Second
		perExtra   = 100 * time.Millisecond
		maxTimeout = 60 * time.Second
	)
	if fileCount <= 1 {
		return base
	}
	timeout := base + time.Duration(fileCount-1)*perExtra
	if timeout > maxTimeout {
		return maxTimeout
	}
	return timeout
}

func resolveSkeletonPaths(root string, cfg config.Config, inputs []string) ([]skeletonInput, error) {
	return resolveSkeletonPathsWithExcludes(root, cfg, inputs, nil)
}

func resolveSkeletonPathsWithExcludes(root string, cfg config.Config, inputs, excludes []string) ([]skeletonInput, error) {
	normalizedExcludes, err := validateSkeletonExcludes(excludes)
	if err != nil {
		return nil, err
	}
	var resolved []skeletonInput
	seen := make(map[string]bool)
	for _, input := range inputs {
		expanded, err := expandSkeletonInput(root, cfg, input, normalizedExcludes)
		if err != nil {
			return nil, err
		}
		for _, item := range expanded {
			canonical, err := filepath.EvalSymlinks(item.path)
			if err != nil {
				return nil, fmt.Errorf("resolving Ruby file: %w", err)
			}
			canonical, err = filepath.Abs(canonical)
			if err != nil {
				return nil, fmt.Errorf("resolving Ruby file: %w", err)
			}
			if seen[canonical] {
				continue
			}
			seen[canonical] = true
			resolved = append(resolved, item)
			if len(resolved) > maxSkeletonFiles {
				return nil, fmt.Errorf("skeleton resolved more than %d unique files; narrow the inputs or add --exclude", maxSkeletonFiles)
			}
		}
	}
	return resolved, nil
}

func expandSkeletonInput(root string, cfg config.Config, input string, excludes []string) ([]skeletonInput, error) {
	if !strings.ContainsAny(input, "*?[") {
		if directory, isDirectory, err := skeletonDirectoryCandidate(root, input); err != nil {
			return nil, err
		} else if isDirectory {
			return expandSkeletonDirectory(root, input, directory, excludes)
		}
		path, relPath, err := resolveSkeletonPath(root, cfg, input)
		if err != nil {
			return nil, err
		}
		return []skeletonInput{{path: path, relPath: relPath}}, nil
	}

	pattern := input
	if !filepath.IsAbs(pattern) {
		pattern = filepath.Join(root, pattern)
	}
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid skeleton glob %q: %w", input, err)
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("skeleton glob %q matched no files", input)
	}
	resolved := make([]skeletonInput, 0, len(matches))
	for _, match := range matches {
		path, relPath, err := resolveRubyPath(root, match)
		if err != nil {
			return nil, fmt.Errorf("resolving match for glob %q: %w", input, err)
		}
		resolved = append(resolved, skeletonInput{path: path, relPath: relPath})
	}
	return resolved, nil
}

func skeletonDirectoryCandidate(root, input string) (string, bool, error) {
	candidate := input
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(root, candidate)
	}
	absPath, err := filepath.Abs(candidate)
	if err != nil {
		return "", false, fmt.Errorf("resolving directory: %w", err)
	}
	info, err := os.Lstat(absPath)
	if os.IsNotExist(err) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("checking skeleton input %s: %w", input, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		targetInfo, statErr := os.Stat(absPath)
		if statErr == nil && targetInfo.IsDir() {
			return "", false, fmt.Errorf("skeleton does not follow directory symlinks: %s", input)
		}
		return "", false, nil
	}
	return absPath, info.IsDir(), nil
}

func expandSkeletonDirectory(root, original, directory string, excludes []string) ([]skeletonInput, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolving Rails root: %w", err)
	}
	rel, err := filepath.Rel(absRoot, directory)
	if err != nil || pathOutsideRoot(rel) {
		return nil, fmt.Errorf("directory is outside Rails root: %s", original)
	}
	actualRoot, err := filepath.EvalSymlinks(absRoot)
	if err != nil {
		return nil, fmt.Errorf("resolving Rails root: %w", err)
	}
	actualDirectory, err := filepath.EvalSymlinks(directory)
	if err != nil {
		return nil, fmt.Errorf("resolving directory: %w", err)
	}
	actualRel, err := filepath.Rel(actualRoot, actualDirectory)
	if err != nil || pathOutsideRoot(actualRel) {
		return nil, fmt.Errorf("directory is outside Rails root: %s", original)
	}

	var resolved []skeletonInput
	err = filepath.WalkDir(directory, func(candidate string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		candidateRel, relErr := filepath.Rel(absRoot, candidate)
		if relErr != nil {
			return relErr
		}
		normalizedRel := filepath.ToSlash(candidateRel)
		if candidate != directory && entry.IsDir() && matchesAnySkeletonExclude(normalizedRel, excludes) {
			return filepath.SkipDir
		}
		if entry.Type()&os.ModeSymlink != 0 {
			if targetInfo, statErr := os.Stat(candidate); statErr == nil && targetInfo.IsDir() {
				return nil
			}
		}
		if entry.IsDir() || filepath.Ext(candidate) != ".rb" {
			return nil
		}
		if matchesAnySkeletonExclude(normalizedRel, excludes) {
			return nil
		}
		filePath, relPath, resolveErr := resolveRubyPath(root, candidate)
		if resolveErr != nil {
			return resolveErr
		}
		resolved = append(resolved, skeletonInput{path: filePath, relPath: relPath})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walking skeleton directory %s: %w", original, err)
	}
	if len(resolved) == 0 {
		return nil, fmt.Errorf("skeleton directory %q contains no Ruby files after exclusions", original)
	}
	return resolved, nil
}

func validateSkeletonExcludes(patterns []string) ([]string, error) {
	normalized := make([]string, len(patterns))
	for i, pattern := range patterns {
		pattern = filepath.ToSlash(strings.TrimSpace(pattern))
		if pattern == "" {
			return nil, fmt.Errorf("skeleton exclude pattern cannot be empty")
		}
		if filepath.IsAbs(patterns[i]) || path.IsAbs(pattern) {
			return nil, fmt.Errorf("skeleton exclude pattern must be Rails-root-relative: %q", patterns[i])
		}
		segments := strings.Split(pattern, "/")
		for _, segment := range segments {
			if segment == ".." {
				return nil, fmt.Errorf("skeleton exclude pattern cannot traverse outside the Rails root: %q", patterns[i])
			}
			if segment == "**" {
				continue
			}
			if _, err := path.Match(segment, ""); err != nil {
				return nil, fmt.Errorf("invalid skeleton exclude pattern %q: %w", patterns[i], err)
			}
		}
		normalized[i] = strings.TrimPrefix(path.Clean(pattern), "./")
	}
	return normalized, nil
}

func matchesAnySkeletonExclude(rel string, patterns []string) bool {
	for _, pattern := range patterns {
		if matchSkeletonPath(pattern, rel) {
			return true
		}
	}
	return false
}

func matchSkeletonPath(pattern, rel string) bool {
	patternSegments := strings.Split(pattern, "/")
	valueSegments := strings.Split(filepath.ToSlash(rel), "/")
	type state struct{ pattern, value int }
	memo := make(map[state]bool)
	visited := make(map[state]bool)
	var match func(int, int) bool
	match = func(patternIndex, valueIndex int) bool {
		key := state{pattern: patternIndex, value: valueIndex}
		if visited[key] {
			return memo[key]
		}
		visited[key] = true

		var result bool
		switch {
		case patternIndex == len(patternSegments):
			result = valueIndex == len(valueSegments)
		case patternSegments[patternIndex] == "**":
			result = match(patternIndex+1, valueIndex) ||
				(valueIndex < len(valueSegments) && match(patternIndex, valueIndex+1))
		case valueIndex < len(valueSegments):
			segmentMatch, err := path.Match(patternSegments[patternIndex], valueSegments[valueIndex])
			result = err == nil && segmentMatch && match(patternIndex+1, valueIndex+1)
		}
		memo[key] = result
		return result
	}
	return match(0, 0)
}

func pathOutsideRoot(rel string) bool {
	return rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func resolveSkeletonPath(root string, cfg config.Config, input string) (string, string, error) {
	if filepath.Ext(input) != "" || strings.Contains(input, string(filepath.Separator)) || strings.Contains(input, "/") {
		return resolveRubyPath(root, input)
	}

	path, err := model.Resolve(root, cfg.ModelsPath, input)
	if err != nil {
		return "", "", fmt.Errorf("resolving model or Ruby path: %w", err)
	}
	return resolveRubyPath(root, path)
}

func resolveRubyPath(root, input string) (string, string, error) {
	path := input
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, input)
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", "", fmt.Errorf("resolving path: %w", err)
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", "", fmt.Errorf("resolving Rails root: %w", err)
	}
	rel, err := filepath.Rel(absRoot, absPath)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", "", fmt.Errorf("ruby file is outside Rails root: %s", input)
	}
	if filepath.Ext(absPath) != ".rb" {
		return "", "", fmt.Errorf("skeleton only supports Ruby files ending in .rb: %s", input)
	}
	info, err := os.Stat(absPath)
	if err != nil {
		return "", "", fmt.Errorf("ruby file not found: %s", input)
	}
	if !info.Mode().IsRegular() {
		return "", "", fmt.Errorf("skeleton only supports regular Ruby files: %s", input)
	}
	actualRoot, err := filepath.EvalSymlinks(absRoot)
	if err != nil {
		return "", "", fmt.Errorf("resolving Rails root: %w", err)
	}
	actualPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		return "", "", fmt.Errorf("resolving Ruby file: %w", err)
	}
	actualRel, err := filepath.Rel(actualRoot, actualPath)
	if err != nil || actualRel == ".." || strings.HasPrefix(actualRel, ".."+string(filepath.Separator)) {
		return "", "", fmt.Errorf("ruby file is outside Rails root: %s", input)
	}
	return absPath, filepath.ToSlash(rel), nil
}
