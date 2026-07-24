package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/janstol/rails-kit/internal/config"
	"github.com/janstol/rails-kit/internal/model"
	"github.com/janstol/rails-kit/internal/prism"
)

var prismRunner = prism.Runner{}

type skeletonInput struct {
	path    string
	relPath string
}

var skeletonCmd = &cobra.Command{
	Use:   "skeleton <path-or-model> [path-or-model...]",
	Short: "Compact Ruby AST skeleton via Prism",
	Long: `Show compact structural skeletons for Ruby files using Prism.

Inputs can be Rails-root-relative .rb paths, absolute .rb paths under the
Rails root, model names resolvable from the configured models path, or glob
patterns. Quote globs to have rails-kit expand them from the Rails root.

The command shells out to Ruby and requires Prism to be available. Existing
rails-kit commands do not require Prism.`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		root, cfg, err := loadConfig()
		if err != nil {
			return err
		}

		inputs, err := resolveSkeletonPaths(root, cfg, args)
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
	var resolved []skeletonInput
	seen := make(map[string]bool)
	for _, input := range inputs {
		expanded, err := expandSkeletonInput(root, cfg, input)
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
		}
	}
	return resolved, nil
}

func expandSkeletonInput(root string, cfg config.Config, input string) ([]skeletonInput, error) {
	if !strings.ContainsAny(input, "*?[") {
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
