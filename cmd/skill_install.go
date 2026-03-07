package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/janstol/rails-kit/internal/skill"
)

var skillInstallGlobal bool

var skillInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install the rails-kit skill into .claude/skills/",
	Long: `Install the rails-kit Claude Code skill.

By default installs to .claude/skills/rails-kit/ in the Rails root.
Use --global to install to ~/.claude/skills/rails-kit/ instead.

When --root is set, install writes to that directory even if it is not a Rails app.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		targetDir, err := skillDir(skillInstallGlobal)
		if err != nil {
			return err
		}

		skillFile := filepath.Join(targetDir, "SKILL.md")
		_, statErr := os.Stat(skillFile)
		existed := statErr == nil

		if err := os.MkdirAll(targetDir, 0755); err != nil {
			return fmt.Errorf("creating skill directory: %w", err)
		}

		if err := os.WriteFile(skillFile, []byte(skill.Content), 0644); err != nil {
			return fmt.Errorf("writing skill file: %w", err)
		}

		if existed {
			fmt.Printf("Updated %s\n", skillFile)
		} else {
			fmt.Printf("Installed %s\n", skillFile)
		}
		return nil
	},
}

func init() {
	skillInstallCmd.Flags().BoolVar(&skillInstallGlobal, "global", false, "Install to ~/.claude/skills/ instead of <rails-root>/.claude/skills/")
	skillCmd.AddCommand(skillInstallCmd)
}

func skillDir(global bool) (string, error) {
	if global {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("finding home directory: %w", err)
		}
		return filepath.Join(home, ".claude", "skills", "rails-kit"), nil
	}

	// When --root is explicitly set, use it directly (no Rails validation needed
	// for skill install — we just need a target directory).
	if rootFlag != "" {
		return filepath.Join(rootFlag, ".claude", "skills", "rails-kit"), nil
	}

	// Auto-detect Rails root from CWD.
	root, err := resolveRailsRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, ".claude", "skills", "rails-kit"), nil
}

func validatedLocalSkillDir() (string, error) {
	root, err := resolveRailsRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, ".claude", "skills", "rails-kit"), nil
}
