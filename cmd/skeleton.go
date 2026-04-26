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

var skeletonCmd = &cobra.Command{
	Use:   "skeleton <path-or-model>",
	Short: "Compact Ruby AST skeleton via Prism",
	Long: `Show a compact structural skeleton for a Ruby file using Prism.

The input can be a Rails-root-relative .rb path, an absolute .rb path under
the Rails root, or a model name resolvable from the configured models path.

The command shells out to Ruby and requires Prism to be available. Existing
rails-kit commands do not require Prism.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		root, cfg, err := loadConfig()
		if err != nil {
			return err
		}

		path, relPath, err := resolveSkeletonPath(root, cfg, args[0])
		if err != nil {
			return err
		}

		parentCtx := cmd.Context()
		if parentCtx == nil {
			parentCtx = context.Background()
		}
		ctx, cancel := context.WithTimeout(parentCtx, 10*time.Second)
		defer cancel()

		runner := prismRunner
		if runner.Dir == "" {
			runner.Dir = root
		}
		files, err := runner.ParseFiles(ctx, []string{path})
		if err != nil {
			if prism.IsUnavailable(err) {
				return fmt.Errorf("prism is not available: install/use Ruby with the prism library available, then retry: %w", err)
			}
			return err
		}
		file := files[0]
		file.RelPath = relPath

		if jsonFlag {
			return printJSON(file)
		}
		fmt.Print(prism.Format(file))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(skeletonCmd)
}

func resolveSkeletonPath(root string, cfg config.Config, input string) (string, string, error) {
	if filepath.Ext(input) != "" || strings.Contains(input, string(filepath.Separator)) || strings.Contains(input, "/") {
		return resolveRubyPath(root, input)
	}

	path, err := model.Resolve(root, cfg.ModelsPath, input)
	if err != nil {
		return "", "", fmt.Errorf("resolving model or Ruby path: %w", err)
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		rel = path
	}
	return path, filepath.ToSlash(rel), nil
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
	if _, err := os.Stat(absPath); err != nil {
		return "", "", fmt.Errorf("ruby file not found: %s", input)
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
