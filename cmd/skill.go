package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

const (
	skillTargetClaude = "claude"
	skillTargetCodex  = "codex"
	skillTargetAll    = "all"
)

var skillCmd = &cobra.Command{
	Use:   "skill",
	Short: "Install or remove the rails-kit agent skill",
}

func init() {
	rootCmd.AddCommand(skillCmd)
}

func skillTargets(value string) ([]string, error) {
	switch value {
	case skillTargetClaude:
		return []string{skillTargetClaude}, nil
	case skillTargetCodex:
		return []string{skillTargetCodex}, nil
	case skillTargetAll:
		return []string{skillTargetClaude, skillTargetCodex}, nil
	default:
		return nil, fmt.Errorf("invalid skill target %q: expected claude, codex, or all", value)
	}
}

func validateSkillScope(local, global bool) error {
	if local && global {
		return fmt.Errorf("--local and --global are mutually exclusive")
	}
	return nil
}

func skillDir(target string, local, validateLocal bool) (string, error) {
	var root string
	if local {
		if validateLocal || rootFlag == "" {
			var err error
			root, err = resolveRailsRoot()
			if err != nil {
				return "", err
			}
		} else {
			root = rootFlag
		}
	} else {
		var err error
		root, err = os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("finding home directory: %w", err)
		}
	}

	switch target {
	case skillTargetClaude:
		return filepath.Join(root, ".claude", "skills", "rails-kit"), nil
	case skillTargetCodex:
		return filepath.Join(root, ".agents", "skills", "rails-kit"), nil
	default:
		return "", fmt.Errorf("invalid skill target %q", target)
	}
}
