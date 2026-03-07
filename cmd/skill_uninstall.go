package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var skillUninstallGlobal bool

var skillUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove the rails-kit skill from .claude/skills/",
	Long: `Remove the rails-kit Claude Code skill.

Local uninstall expects a Rails root (auto-detected or passed with --root).
Use --global to remove ~/.claude/skills/rails-kit instead.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		var (
			targetDir string
			err       error
		)
		if skillUninstallGlobal {
			targetDir, err = skillDir(true)
		} else {
			targetDir, err = validatedLocalSkillDir()
		}
		if err != nil {
			return err
		}

		if _, err := os.Stat(targetDir); err != nil {
			if os.IsNotExist(err) {
				fmt.Printf("Skill not installed at %s\n", targetDir)
				return nil
			}
			return fmt.Errorf("checking skill directory: %w", err)
		}

		if err := os.RemoveAll(targetDir); err != nil {
			return fmt.Errorf("removing skill directory: %w", err)
		}

		fmt.Printf("Uninstalled %s\n", targetDir)
		return nil
	},
}

func init() {
	skillUninstallCmd.Flags().BoolVar(&skillUninstallGlobal, "global", false, "Uninstall from ~/.claude/skills/ instead of <rails-root>/.claude/skills/")
	skillCmd.AddCommand(skillUninstallCmd)
}
